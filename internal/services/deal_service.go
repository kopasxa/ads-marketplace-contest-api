package services

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/ads-marketplace/backend/internal/config"
	"github.com/ads-marketplace/backend/internal/events"
	"github.com/ads-marketplace/backend/internal/models"
	"github.com/ads-marketplace/backend/internal/repositories"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type DealService struct {
	dealRepo     *repositories.DealRepo
	channelRepo  *repositories.ChannelRepo
	escrowRepo   *repositories.EscrowRepo
	auditRepo    *repositories.AuditRepo
	withdrawRepo *repositories.WithdrawRepo
	walletRepo   *repositories.WalletRepo
	botClient    *BotClient
	publisher    events.Publisher
	cfg          *config.Config
	log          *zap.Logger
}

func NewDealService(
	dealRepo *repositories.DealRepo,
	channelRepo *repositories.ChannelRepo,
	escrowRepo *repositories.EscrowRepo,
	auditRepo *repositories.AuditRepo,
	withdrawRepo *repositories.WithdrawRepo,
	walletRepo *repositories.WalletRepo,
	botClient *BotClient,
	publisher events.Publisher,
	cfg *config.Config,
	log *zap.Logger,
) *DealService {
	return &DealService{
		dealRepo:     dealRepo,
		channelRepo:  channelRepo,
		escrowRepo:   escrowRepo,
		auditRepo:    auditRepo,
		withdrawRepo: withdrawRepo,
		walletRepo:   walletRepo,
		botClient:    botClient,
		publisher:    publisher,
		cfg:          cfg,
		log:          log,
	}
}

// transition validates and performs a status transition with audit logging.
func (s *DealService) transition(ctx context.Context, deal *models.Deal, newStatus string, actorID *uuid.UUID, actorType string) error {
	if !models.IsValidTransition(deal.Status, newStatus) {
		return fmt.Errorf("invalid transition from %s to %s", deal.Status, newStatus)
	}

	oldStatus := deal.Status
	if err := s.dealRepo.UpdateStatus(ctx, deal.ID, newStatus); err != nil {
		return err
	}
	deal.Status = newStatus

	// Audit log
	_ = s.auditRepo.Log(ctx, models.AuditLog{
		ActorUserID: actorID,
		ActorType:   actorType,
		Action:      fmt.Sprintf("deal_status_%s_to_%s", oldStatus, newStatus),
		EntityType:  "deal",
		EntityID:    &deal.ID,
		Meta:        map[string]any{"old_status": oldStatus, "new_status": newStatus},
	})

	// Publish event
	_ = s.publisher.Publish(ctx, "events:deal", events.Event{
		Type: events.EventDealStatusChanged,
		Payload: map[string]any{
			"deal_id":    deal.ID.String(),
			"old_status": oldStatus,
			"new_status": newStatus,
		},
	})

	return nil
}

func (s *DealService) CreateDeal(ctx context.Context, advertiserID, channelID uuid.UUID, adFormat string, brief *string, priceTON string, scheduledAt *time.Time) (*models.Deal, error) {
	// 1. Валидация формата
	if !models.IsValidAdFormat(adFormat) {
		return nil, fmt.Errorf("invalid ad format %q, must be one of: post, repost, story", adFormat)
	}

	// 2. Получаем листинг канала
	listing, err := s.channelRepo.GetListing(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("channel listing not found: %w", err)
	}

	// 3. Проверяем, что формат включён в листинге
	if !listing.IsFormatEnabled(adFormat) {
		return nil, fmt.Errorf("ad format %q is not enabled for this channel (available: %v)", adFormat, listing.FormatsEnabled)
	}

	// 4. Если цена не указана — берём из листинга
	if priceTON == "" || priceTON == "0" {
		listingPrice := listing.GetPriceForFormat(adFormat)
		if listingPrice == nil || *listingPrice == "" {
			return nil, fmt.Errorf("no price set for format %q in channel listing", adFormat)
		}
		priceTON = *listingPrice
	}

	// 5. Hold period: используем формат-специфичный из листинга, fallback на env
	holdSeconds := listing.GetHoldHoursForFormat(adFormat) * 3600
	if holdSeconds <= 0 {
		holdSeconds = s.cfg.HoldPeriodSeconds
	}

	deal := &models.Deal{
		ChannelID:         channelID,
		AdvertiserUserID:  advertiserID,
		Status:            models.DealStatusDraft,
		AdFormat:          adFormat,
		Brief:             brief,
		ScheduledAt:       scheduledAt,
		PriceTON:          priceTON,
		PlatformFeeBPS:    s.cfg.PlatformFeeBPS,
		HoldPeriodSeconds: holdSeconds,
	}

	if err := s.dealRepo.Create(ctx, deal); err != nil {
		return nil, err
	}

	_ = s.auditRepo.Log(ctx, models.AuditLog{
		ActorUserID: &advertiserID,
		ActorType:   "user",
		Action:      "deal_created",
		EntityType:  "deal",
		EntityID:    &deal.ID,
		Meta:        map[string]any{"ad_format": adFormat, "price_ton": priceTON},
	})

	return deal, nil
}

func (s *DealService) SubmitDeal(ctx context.Context, dealID uuid.UUID, actorID uuid.UUID) error {
	deal, err := s.dealRepo.GetByID(ctx, dealID)
	if err != nil {
		return err
	}
	if deal.AdvertiserUserID != actorID {
		return fmt.Errorf("only advertiser can submit deal")
	}
	return s.transition(ctx, deal, models.DealStatusSubmitted, &actorID, "user")
}

func (s *DealService) AcceptDeal(ctx context.Context, dealID uuid.UUID, actorID uuid.UUID) error {
	deal, err := s.dealRepo.GetByID(ctx, dealID)
	if err != nil {
		return err
	}
	if err := s.checkChannelRole(ctx, deal.ChannelID, actorID, false); err != nil {
		return err
	}

	if err := s.transition(ctx, deal, models.DealStatusAccepted, &actorID, "user"); err != nil {
		return err
	}

	// Auto-transition to awaiting_payment + create escrow
	if err := s.transition(ctx, deal, models.DealStatusAwaitingPayment, &actorID, "system"); err != nil {
		return err
	}

	memo := fmt.Sprintf("deal:%s", deal.ID.String())
	escrow := &models.EscrowLedger{
		DealID:             deal.ID,
		DepositExpectedTON: deal.PriceTON,
		DepositAddress:     s.cfg.TONHotWalletAddress,
		DepositMemo:        memo,
		Status:             models.EscrowStatusAwaiting,
	}
	return s.escrowRepo.Create(ctx, escrow)
}

func (s *DealService) RejectDeal(ctx context.Context, dealID uuid.UUID, actorID uuid.UUID) error {
	deal, err := s.dealRepo.GetByID(ctx, dealID)
	if err != nil {
		return err
	}
	if err := s.checkChannelRole(ctx, deal.ChannelID, actorID, false); err != nil {
		return err
	}
	return s.transition(ctx, deal, models.DealStatusRejected, &actorID, "user")
}

func (s *DealService) CancelDeal(ctx context.Context, dealID uuid.UUID, actorID uuid.UUID) error {
	deal, err := s.dealRepo.GetByID(ctx, dealID)
	if err != nil {
		return err
	}
	// Advertiser can cancel before funded; owner/manager also can cancel certain statuses
	if deal.AdvertiserUserID != actorID {
		if err := s.checkChannelRole(ctx, deal.ChannelID, actorID, false); err != nil {
			return fmt.Errorf("only advertiser or channel owner/manager can cancel")
		}
	}
	return s.transition(ctx, deal, models.DealStatusCancelled, &actorID, "user")
}

type SubmitCreativeInput struct {
	Text          string
	RepostFromURL *string
	MediaURLs     []string
	ButtonsJSON   any
}

func (s *DealService) SubmitCreative(ctx context.Context, dealID uuid.UUID, actorID uuid.UUID, input SubmitCreativeInput) error {
	deal, err := s.dealRepo.GetByID(ctx, dealID)
	if err != nil {
		return err
	}
	if err := s.checkChannelRole(ctx, deal.ChannelID, actorID, false); err != nil {
		return err
	}

	// Валидация формат-специфичных данных
	if deal.AdFormat == models.AdFormatRepost && (input.RepostFromURL == nil || *input.RepostFromURL == "") {
		return fmt.Errorf("repost format requires repost_from_url")
	}

	// Must be in creative_pending or creative_changes_requested
	if deal.Status != models.DealStatusCreativePending && deal.Status != models.DealStatusCreativeChangesRequested {
		// If funded, auto-transition to creative_pending first
		if deal.Status == models.DealStatusFunded {
			if err := s.transition(ctx, deal, models.DealStatusCreativePending, &actorID, "system"); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("deal is not in a state that accepts creatives: %s", deal.Status)
		}
	}

	maxV, _ := s.dealRepo.GetCreativeMaxVersion(ctx, dealID)
	creative := &models.DealCreative{
		DealID:            dealID,
		Version:           maxV + 1,
		OwnerComposedText: &input.Text,
		RepostFromURL:     input.RepostFromURL,
		Status:            "submitted",
	}
	if input.MediaURLs != nil {
		creative.MediaURLs = input.MediaURLs
	}
	if input.ButtonsJSON != nil {
		creative.ButtonsJSON = input.ButtonsJSON
	}
	if err := s.dealRepo.CreateCreative(ctx, creative); err != nil {
		return err
	}

	return s.transition(ctx, deal, models.DealStatusCreativeSubmitted, &actorID, "user")
}

func (s *DealService) ApproveCreative(ctx context.Context, dealID uuid.UUID, actorID uuid.UUID) error {
	deal, err := s.dealRepo.GetByID(ctx, dealID)
	if err != nil {
		return err
	}
	if deal.AdvertiserUserID != actorID {
		return fmt.Errorf("only advertiser can approve creative")
	}

	creative, err := s.dealRepo.GetLatestCreative(ctx, dealID)
	if err != nil {
		return err
	}
	_ = s.dealRepo.UpdateCreativeStatus(ctx, creative.ID, "approved")

	return s.transition(ctx, deal, models.DealStatusCreativeApproved, &actorID, "user")
}

func (s *DealService) RequestCreativeChanges(ctx context.Context, dealID uuid.UUID, actorID uuid.UUID, feedback *string) error {
	deal, err := s.dealRepo.GetByID(ctx, dealID)
	if err != nil {
		return err
	}
	if deal.AdvertiserUserID != actorID {
		return fmt.Errorf("only advertiser can request changes")
	}

	creative, err := s.dealRepo.GetLatestCreative(ctx, dealID)
	if err != nil {
		return err
	}
	_ = s.dealRepo.UpdateCreativeStatus(ctx, creative.ID, "changes_requested")

	if err := s.transition(ctx, deal, models.DealStatusCreativeChangesRequested, &actorID, "user"); err != nil {
		return err
	}

	if feedback != nil && *feedback != "" {
		_ = s.auditRepo.Log(ctx, models.AuditLog{
			ActorUserID: &actorID,
			ActorType:   "user",
			Action:      "creative_changes_feedback",
			EntityType:  "deal",
			EntityID:    &dealID,
			Meta:        map[string]any{"feedback": *feedback},
		})
	}

	return nil
}

func (s *DealService) MarkManualPost(ctx context.Context, dealID uuid.UUID, actorID uuid.UUID, postURL string) error {
	deal, err := s.dealRepo.GetByID(ctx, dealID)
	if err != nil {
		return err
	}
	if err := s.checkChannelRole(ctx, deal.ChannelID, actorID, false); err != nil {
		return err
	}
	if deal.Status != models.DealStatusCreativeApproved && deal.Status != models.DealStatusScheduled {
		return fmt.Errorf("deal must be creative_approved or scheduled to mark post")
	}

	// Create content hash from URL (placeholder)
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(postURL)))

	now := time.Now()
	post := &models.DealPost{
		DealID:      dealID,
		PostURL:     &postURL,
		ContentHash: &hash,
		PostedAt:    &now,
	}
	if err := s.dealRepo.UpsertPost(ctx, post); err != nil {
		return err
	}

	if err := s.transition(ctx, deal, models.DealStatusPosted, &actorID, "user"); err != nil {
		return err
	}
	return s.transition(ctx, deal, models.DealStatusHoldVerification, &actorID, "system")
}

func (s *DealService) SetWithdrawWallet(ctx context.Context, dealID uuid.UUID, actorID uuid.UUID, walletAddress string) error {
	deal, err := s.dealRepo.GetByID(ctx, dealID)
	if err != nil {
		return err
	}

	// Must be owner — re-check admin via bot
	if err := s.recheckOwnerAdmin(ctx, deal.ChannelID, actorID); err != nil {
		return err
	}

	// Проверяем, что у пользователя есть верифицированный кошелёк и адрес совпадает
	userWallet, err := s.walletRepo.GetActiveWallet(ctx, actorID)
	if err != nil {
		return fmt.Errorf("no verified wallet connected — connect your wallet via TON Connect first")
	}
	if !userWallet.Verified {
		return fmt.Errorf("connected wallet is not verified")
	}
	// Адрес для вывода должен совпадать с подключённым (или быть тем же)
	if walletAddress != userWallet.Address && walletAddress != userWallet.AddressFriendly {
		return fmt.Errorf("withdraw address must match your connected verified wallet (%s)", userWallet.AddressFriendly)
	}

	wallet := &models.WithdrawWallet{
		ChannelID:     deal.ChannelID,
		OwnerUserID:   actorID,
		WalletAddress: userWallet.AddressFriendly,
	}
	return s.withdrawRepo.Upsert(ctx, wallet)
}

func (s *DealService) ReleaseFunds(ctx context.Context, dealID uuid.UUID) error {
	deal, err := s.dealRepo.GetByID(ctx, dealID)
	if err != nil {
		return err
	}
	if deal.Status != models.DealStatusHoldVerification {
		return fmt.Errorf("deal is not in hold_verification")
	}

	// Check post not deleted
	post, err := s.dealRepo.GetPost(ctx, dealID)
	if err != nil {
		return err
	}
	if post.IsDeleted {
		return s.transition(ctx, deal, models.DealStatusHoldVerificationFailed, nil, "system")
	}

	// Calculate release amount
	feeBPS := deal.PlatformFeeBPS
	// This is a simplified calculation — in production use big decimal
	_ = feeBPS

	if err := s.transition(ctx, deal, models.DealStatusCompleted, nil, "system"); err != nil {
		return err
	}

	// Mark escrow released (tx_hash will be filled by actual TON send)
	return s.escrowRepo.MarkReleased(ctx, dealID, deal.PriceTON, "pending_send")
}

func (s *DealService) RefundDeal(ctx context.Context, dealID uuid.UUID) error {
	deal, err := s.dealRepo.GetByID(ctx, dealID)
	if err != nil {
		return err
	}
	return s.transition(ctx, deal, models.DealStatusRefunded, nil, "system")
}

func (s *DealService) GetDeal(ctx context.Context, id uuid.UUID) (*models.DealWithChannel, error) {
	return s.dealRepo.GetByIDWithChannel(ctx, id)
}

func (s *DealService) ListDeals(ctx context.Context, f repositories.DealFilter) ([]models.DealWithChannel, error) {
	return s.dealRepo.ListWithChannel(ctx, f)
}

func (s *DealService) GetLatestCreative(ctx context.Context, dealID uuid.UUID) (*models.DealCreative, error) {
	return s.dealRepo.GetLatestCreative(ctx, dealID)
}

func (s *DealService) GetDealEvents(ctx context.Context, dealID uuid.UUID) ([]models.AuditLog, error) {
	return s.auditRepo.GetByEntity(ctx, "deal", dealID, 100, 0)
}

func (s *DealService) GetPaymentInfo(ctx context.Context, dealID uuid.UUID) (*models.EscrowLedger, error) {
	return s.escrowRepo.GetByDealID(ctx, dealID)
}

// --- helpers ---

func (s *DealService) checkChannelRole(ctx context.Context, channelID, userID uuid.UUID, ownerOnly bool) error {
	member, err := s.channelRepo.GetMemberByUserAndChannel(ctx, channelID, userID)
	if err != nil {
		return fmt.Errorf("user is not a member of this channel")
	}
	if ownerOnly && member.Role != "owner" {
		return fmt.Errorf("only owner can perform this action")
	}
	return nil
}

func (s *DealService) recheckOwnerAdmin(ctx context.Context, channelID, userID uuid.UUID) error {
	// Check is owner in DB
	member, err := s.channelRepo.GetMemberByUserAndChannel(ctx, channelID, userID)
	if err != nil || member.Role != "owner" {
		return fmt.Errorf("user is not owner of this channel")
	}

	// Get channel
	ch, err := s.channelRepo.GetByID(ctx, channelID)
	if err != nil {
		return err
	}

	// Get user to obtain telegram_id — we need a user repo here
	// For now, we trust the DB record but log a warning
	// In production, inject UserRepo and do full re-check via bot
	s.log.Warn("re-check admin: full bot verification recommended",
		zap.String("channel", ch.Username),
		zap.String("user_id", userID.String()),
	)

	return nil
}

// RecheckAdminViaBot performs actual admin check through bot service.
func (s *DealService) RecheckAdminViaBot(ctx context.Context, channelUsername string, telegramUserID int64) error {
	result, err := s.botClient.CheckAdmin(ctx, channelUsername, telegramUserID)
	if err != nil {
		return fmt.Errorf("failed to verify admin status: %w", err)
	}
	if !result.IsAdmin || !result.CanPostMessages {
		return fmt.Errorf("user is not an admin with posting rights")
	}
	return nil
}
