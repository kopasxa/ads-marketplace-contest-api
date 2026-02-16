package services

import (
	"context"
	"fmt"

	"github.com/ads-marketplace/backend/internal/config"
	"github.com/ads-marketplace/backend/internal/models"
	"github.com/ads-marketplace/backend/internal/repositories"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type ChannelService struct {
	channelRepo *repositories.ChannelRepo
	userRepo    *repositories.UserRepo
	auditRepo   *repositories.AuditRepo
	botClient   *BotClient
	cfg         *config.Config
	log         *zap.Logger
}

func NewChannelService(
	channelRepo *repositories.ChannelRepo,
	userRepo *repositories.UserRepo,
	auditRepo *repositories.AuditRepo,
	botClient *BotClient,
	cfg *config.Config,
	log *zap.Logger,
) *ChannelService {
	return &ChannelService{
		channelRepo: channelRepo,
		userRepo:    userRepo,
		auditRepo:   auditRepo,
		botClient:   botClient,
		cfg:         cfg,
		log:         log,
	}
}

func (s *ChannelService) CreateChannel(ctx context.Context, username string, creatorUserID uuid.UUID) (*models.Channel, error) {
	username = repositories.NormalizeUsername(username)
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}

	ch := &models.Channel{
		Username:  username,
		BotStatus: "pending",
	}

	if err := s.channelRepo.Create(ctx, ch); err != nil {
		return nil, err
	}

	_ = s.auditRepo.Log(ctx, models.AuditLog{
		ActorUserID: &creatorUserID,
		ActorType:   "user",
		Action:      "channel_created",
		EntityType:  "channel",
		EntityID:    &ch.ID,
	})

	return ch, nil
}

func (s *ChannelService) GetChannel(ctx context.Context, id uuid.UUID) (*models.Channel, error) {
	return s.channelRepo.GetByID(ctx, id)
}

func (s *ChannelService) SearchChannels(ctx context.Context, f repositories.ChannelFilter) ([]models.Channel, error) {
	return s.channelRepo.Search(ctx, f)
}

func (s *ChannelService) GetMyChannels(ctx context.Context, userID uuid.UUID) ([]models.Channel, error) {
	return s.channelRepo.GetByUserID(ctx, userID)
}

func (s *ChannelService) GetBotInviteLink(ctx context.Context, channelID uuid.UUID) (string, error) {
	ch, err := s.channelRepo.GetByID(ctx, channelID)
	if err != nil {
		return "", err
	}
	// Return deeplink instruction
	// The user should add the bot as admin to @<username> channel
	return fmt.Sprintf("Add the bot as an administrator to @%s with 'Post Messages' permission. Use this link: https://t.me/YOUR_BOT_USERNAME?startchannel&admin=post_messages", ch.Username), nil
}

func (s *ChannelService) AddManager(ctx context.Context, channelID uuid.UUID, actorID uuid.UUID, managerTelegramID int64) error {
	// Check actor is owner
	member, err := s.channelRepo.GetMemberByUserAndChannel(ctx, channelID, actorID)
	if err != nil || member.Role != "owner" {
		return fmt.Errorf("only owner can add managers")
	}

	// Check member count
	count, err := s.channelRepo.CountMembers(ctx, channelID)
	if err != nil {
		return err
	}
	if count >= 3 {
		return fmt.Errorf("maximum 3 members (owner + 2 managers) allowed")
	}

	// Get or create manager user
	managerUser, err := s.userRepo.UpsertByTelegramID(ctx, managerTelegramID, nil, nil, nil)
	if err != nil {
		return err
	}

	// Verify via bot that this person is actually admin
	ch, err := s.channelRepo.GetByID(ctx, channelID)
	if err != nil {
		return err
	}

	result, err := s.botClient.CheckAdmin(ctx, ch.Username, managerTelegramID)
	if err != nil {
		return fmt.Errorf("failed to verify admin: %w", err)
	}
	if !result.IsAdmin {
		return fmt.Errorf("user %d is not an admin of channel @%s", managerTelegramID, ch.Username)
	}

	m := &models.ChannelMember{
		ChannelID: channelID,
		UserID:    managerUser.ID,
		Role:      "manager",
		CanPost:   result.CanPostMessages,
	}

	return s.channelRepo.AddMember(ctx, m)
}

func (s *ChannelService) GetAdmins(ctx context.Context, channelID uuid.UUID) ([]AdminInfo, error) {
	ch, err := s.channelRepo.GetByID(ctx, channelID)
	if err != nil {
		return nil, err
	}
	return s.botClient.GetAdmins(ctx, ch.Username)
}

func (s *ChannelService) GetMembers(ctx context.Context, channelID uuid.UUID) ([]models.ChannelMember, error) {
	return s.channelRepo.GetMembers(ctx, channelID)
}

func (s *ChannelService) UpsertListing(ctx context.Context, channelID uuid.UUID, actorID uuid.UUID, listing *models.ChannelListing) error {
	// Check actor is owner or manager
	_, err := s.channelRepo.GetMemberByUserAndChannel(ctx, channelID, actorID)
	if err != nil {
		return fmt.Errorf("user is not a member of this channel")
	}

	listing.ChannelID = channelID
	return s.channelRepo.UpsertListing(ctx, listing)
}

func (s *ChannelService) GetListing(ctx context.Context, channelID uuid.UUID) (*models.ChannelListing, error) {
	return s.channelRepo.GetListing(ctx, channelID)
}

func (s *ChannelService) GetLatestStats(ctx context.Context, channelID uuid.UUID) (*models.ChannelStatsSnapshot, error) {
	return s.channelRepo.GetLatestStats(ctx, channelID)
}

// ChannelStatsResponse is a frontend-friendly stats representation.
type ChannelStatsResponse struct {
	Subscribers                 *int     `json:"subscribers"`
	AvgViews                    *int     `json:"avg_views"`
	PremiumCount                *int     `json:"premium_count,omitempty"`
	LastUpdated                 *string  `json:"last_updated,omitempty"`
	Source                      string   `json:"source"`
	MembersOnline               *int     `json:"members_online,omitempty"`
	AdminsCount                 *int     `json:"admins_count,omitempty"`
	Growth7d                    *int     `json:"growth_7d,omitempty"`
	Growth30d                   *int     `json:"growth_30d,omitempty"`
	PostsCount                  *int     `json:"posts_count,omitempty"`
	ViewsPerPost                *float64 `json:"views_per_post,omitempty"`
	SharesPerPost               *float64 `json:"shares_per_post,omitempty"`
	EnabledNotificationsPercent *float64 `json:"enabled_notifications_percent,omitempty"`
	ERPercent                   *float64 `json:"er_percent,omitempty"`
}

func (s *ChannelService) GetChannelStats(ctx context.Context, channelID uuid.UUID) (*ChannelStatsResponse, error) {
	stats, err := s.channelRepo.GetLatestStats(ctx, channelID)
	if err != nil {
		return nil, err
	}

	ts := stats.FetchedAt.Format("2006-01-02T15:04:05Z07:00")
	resp := &ChannelStatsResponse{
		Subscribers:                 stats.Subscribers,
		AvgViews:                    stats.AvgViews20,
		PremiumCount:                stats.PremiumCount,
		LastUpdated:                 &ts,
		Source:                      stats.Source,
		MembersOnline:               stats.MembersOnline,
		AdminsCount:                 stats.AdminsCount,
		Growth7d:                    stats.Growth7d,
		Growth30d:                   stats.Growth30d,
		PostsCount:                  stats.PostsCount,
		ViewsPerPost:                stats.ViewsPerPost,
		SharesPerPost:               stats.SharesPerPost,
		EnabledNotificationsPercent: stats.EnabledNotificationsPercent,
		ERPercent:                   stats.ERPercent,
	}
	return resp, nil
}

// ExploreChannel is an enriched channel representation for the explore/marketplace page.
type ExploreChannel struct {
	ID          uuid.UUID              `json:"id"`
	Username    string                 `json:"username"`
	Title       *string                `json:"title,omitempty"`
	BotStatus   string                 `json:"bot_status"`
	Subscribers *int                   `json:"subscribers,omitempty"`
	AvgViews    *int                   `json:"avg_views,omitempty"`
	ERPercent   *float64               `json:"er_percent,omitempty"`
	Category    *string                `json:"category,omitempty"`
	Language    *string                `json:"language,omitempty"`
	Listing     *ExploreChannelListing `json:"listing,omitempty"`
}

type ExploreChannelListing struct {
	Status         string  `json:"status"`
	PricePostTON   *string `json:"price_post_ton,omitempty"`
	PriceRepostTON *string `json:"price_repost_ton,omitempty"`
	PriceStoryTON  *string `json:"price_story_ton,omitempty"`
	Description    *string `json:"description,omitempty"`
}

func (s *ChannelService) ExploreChannels(ctx context.Context, f repositories.ChannelFilter) ([]ExploreChannel, error) {
	rows, err := s.channelRepo.SearchExplore(ctx, f)
	if err != nil {
		return nil, err
	}

	result := make([]ExploreChannel, 0, len(rows))
	for _, r := range rows {
		ec := ExploreChannel{
			ID:          r.ID,
			Username:    r.Username,
			Title:       r.Title,
			BotStatus:   r.BotStatus,
			Subscribers: r.Subscribers,
			ERPercent:   r.ERPercent,
			AvgViews:    r.AvgViews,
			Category:    r.Category,
			Language:    r.Language,
		}
		if r.ListingStatus != nil {
			ec.Listing = &ExploreChannelListing{
				Status:         *r.ListingStatus,
				PricePostTON:   r.PricePostTON,
				PriceRepostTON: r.PriceRepostTON,
				PriceStoryTON:  r.PriceStoryTON,
				Description:    r.Description,
			}
		}
		result = append(result, ec)
	}
	return result, nil
}
