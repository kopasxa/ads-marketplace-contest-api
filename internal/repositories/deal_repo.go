package repositories

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ads-marketplace/backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DealRepo struct {
	pool *pgxpool.Pool
}

func NewDealRepo(pool *pgxpool.Pool) *DealRepo {
	return &DealRepo{pool: pool}
}

func (r *DealRepo) Create(ctx context.Context, d *models.Deal) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO deals (channel_id, advertiser_user_id, status, ad_format, brief, scheduled_at, price_ton, platform_fee_bps, hold_period_seconds)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at
	`, d.ChannelID, d.AdvertiserUserID, d.Status, d.AdFormat, d.Brief, d.ScheduledAt, d.PriceTON, d.PlatformFeeBPS, d.HoldPeriodSeconds,
	).Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt)
}

func (r *DealRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Deal, error) {
	var d models.Deal
	err := r.pool.QueryRow(ctx, `
		SELECT id, channel_id, advertiser_user_id, status, ad_format, brief, scheduled_at,
		       price_ton, platform_fee_bps, hold_period_seconds, created_at, updated_at
		FROM deals WHERE id = $1
	`, id).Scan(&d.ID, &d.ChannelID, &d.AdvertiserUserID, &d.Status, &d.AdFormat, &d.Brief, &d.ScheduledAt,
		&d.PriceTON, &d.PlatformFeeBPS, &d.HoldPeriodSeconds, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *DealRepo) GetByIDWithChannel(ctx context.Context, id uuid.UUID) (*models.DealWithChannel, error) {
	var d models.DealWithChannel
	err := r.pool.QueryRow(ctx, `
		SELECT d.id, d.channel_id, d.advertiser_user_id, d.status, d.ad_format, d.brief, d.scheduled_at,
		       d.price_ton, d.platform_fee_bps, d.hold_period_seconds, d.created_at, d.updated_at,
		       c.title, c.username
		FROM deals d
		JOIN channels c ON c.id = d.channel_id
		WHERE d.id = $1
	`, id).Scan(&d.ID, &d.ChannelID, &d.AdvertiserUserID, &d.Status, &d.AdFormat, &d.Brief, &d.ScheduledAt,
		&d.PriceTON, &d.PlatformFeeBPS, &d.HoldPeriodSeconds, &d.CreatedAt, &d.UpdatedAt,
		&d.ChannelTitle, &d.ChannelUsername)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *DealRepo) ListWithChannel(ctx context.Context, f DealFilter) ([]models.DealWithChannel, error) {
	query := `
		SELECT d.id, d.channel_id, d.advertiser_user_id, d.status, d.ad_format, d.brief, d.scheduled_at,
		       d.price_ton, d.platform_fee_bps, d.hold_period_seconds, d.created_at, d.updated_at,
		       c.title, c.username
		FROM deals d
		JOIN channels c ON c.id = d.channel_id
	`
	args := []any{}
	argIdx := 1
	where := []string{}

	if f.ChannelID != nil {
		where = append(where, fmt.Sprintf("d.channel_id = $%d", argIdx))
		args = append(args, *f.ChannelID)
		argIdx++
	}
	if f.AdvertiserUserID != nil {
		where = append(where, fmt.Sprintf("d.advertiser_user_id = $%d", argIdx))
		args = append(args, *f.AdvertiserUserID)
		argIdx++
	}
	if f.OwnerUserID != nil {
		query += ` JOIN channel_members cm ON cm.channel_id = d.channel_id `
		where = append(where, fmt.Sprintf("cm.user_id = $%d", argIdx))
		args = append(args, *f.OwnerUserID)
		argIdx++
	}
	if f.Status != nil {
		where = append(where, fmt.Sprintf("d.status = $%d", argIdx))
		args = append(args, *f.Status)
		argIdx++
	}

	if len(where) > 0 {
		query += " WHERE "
		for i, w := range where {
			if i > 0 {
				query += " AND "
			}
			query += w
		}
	}

	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	query += fmt.Sprintf(" ORDER BY d.created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, f.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deals []models.DealWithChannel
	for rows.Next() {
		var d models.DealWithChannel
		if err := rows.Scan(&d.ID, &d.ChannelID, &d.AdvertiserUserID, &d.Status, &d.AdFormat, &d.Brief, &d.ScheduledAt,
			&d.PriceTON, &d.PlatformFeeBPS, &d.HoldPeriodSeconds, &d.CreatedAt, &d.UpdatedAt,
			&d.ChannelTitle, &d.ChannelUsername); err != nil {
			return nil, err
		}
		deals = append(deals, d)
	}
	return deals, nil
}

func (r *DealRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx, `UPDATE deals SET status = $1, updated_at = now() WHERE id = $2`, status, id)
	return err
}

func (r *DealRepo) UpdateScheduledAt(ctx context.Context, id uuid.UUID, d *models.Deal) error {
	_, err := r.pool.Exec(ctx, `UPDATE deals SET scheduled_at = $1, updated_at = now() WHERE id = $2`, d.ScheduledAt, id)
	return err
}

type DealFilter struct {
	ChannelID        *uuid.UUID
	AdvertiserUserID *uuid.UUID
	OwnerUserID      *uuid.UUID // through channel_members
	Status           *string
	Limit            int
	Offset           int
}

func (r *DealRepo) List(ctx context.Context, f DealFilter) ([]models.Deal, error) {
	query := `
		SELECT d.id, d.channel_id, d.advertiser_user_id, d.status, d.ad_format, d.brief, d.scheduled_at,
		       d.price_ton, d.platform_fee_bps, d.hold_period_seconds, d.created_at, d.updated_at
		FROM deals d
	`
	args := []any{}
	argIdx := 1
	where := []string{}

	if f.ChannelID != nil {
		where = append(where, fmt.Sprintf("d.channel_id = $%d", argIdx))
		args = append(args, *f.ChannelID)
		argIdx++
	}
	if f.AdvertiserUserID != nil {
		where = append(where, fmt.Sprintf("d.advertiser_user_id = $%d", argIdx))
		args = append(args, *f.AdvertiserUserID)
		argIdx++
	}
	if f.OwnerUserID != nil {
		query += ` JOIN channel_members cm ON cm.channel_id = d.channel_id `
		where = append(where, fmt.Sprintf("cm.user_id = $%d", argIdx))
		args = append(args, *f.OwnerUserID)
		argIdx++
	}
	if f.Status != nil {
		where = append(where, fmt.Sprintf("d.status = $%d", argIdx))
		args = append(args, *f.Status)
		argIdx++
	}

	if len(where) > 0 {
		query += " WHERE "
		for i, w := range where {
			if i > 0 {
				query += " AND "
			}
			query += w
		}
	}

	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	query += fmt.Sprintf(" ORDER BY d.created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, f.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deals []models.Deal
	for rows.Next() {
		var d models.Deal
		if err := rows.Scan(&d.ID, &d.ChannelID, &d.AdvertiserUserID, &d.Status, &d.AdFormat, &d.Brief, &d.ScheduledAt,
			&d.PriceTON, &d.PlatformFeeBPS, &d.HoldPeriodSeconds, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		deals = append(deals, d)
	}
	return deals, nil
}

func (r *DealRepo) GetTimedOutDeals(ctx context.Context, status string, timeoutSeconds int) ([]models.Deal, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, channel_id, advertiser_user_id, status, ad_format, brief, scheduled_at,
		       price_ton, platform_fee_bps, hold_period_seconds, created_at, updated_at
		FROM deals
		WHERE status = $1 AND updated_at < now() - ($2 || ' seconds')::interval
	`, status, fmt.Sprintf("%d", timeoutSeconds))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deals []models.Deal
	for rows.Next() {
		var d models.Deal
		if err := rows.Scan(&d.ID, &d.ChannelID, &d.AdvertiserUserID, &d.Status, &d.AdFormat, &d.Brief, &d.ScheduledAt,
			&d.PriceTON, &d.PlatformFeeBPS, &d.HoldPeriodSeconds, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		deals = append(deals, d)
	}
	return deals, nil
}

func (r *DealRepo) GetPostedDealsInHold(ctx context.Context) ([]models.Deal, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT d.id, d.channel_id, d.advertiser_user_id, d.status, d.ad_format, d.brief, d.scheduled_at,
		       d.price_ton, d.platform_fee_bps, d.hold_period_seconds, d.created_at, d.updated_at
		FROM deals d
		JOIN deal_posts dp ON dp.deal_id = d.id
		WHERE d.status = 'hold_verification'
		  AND dp.posted_at + (d.hold_period_seconds || ' seconds')::interval < now()
		  AND dp.is_deleted = false
		  AND dp.is_edited = false
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deals []models.Deal
	for rows.Next() {
		var d models.Deal
		if err := rows.Scan(&d.ID, &d.ChannelID, &d.AdvertiserUserID, &d.Status, &d.AdFormat, &d.Brief, &d.ScheduledAt,
			&d.PriceTON, &d.PlatformFeeBPS, &d.HoldPeriodSeconds, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		deals = append(deals, d)
	}
	return deals, nil
}

// ---- Creatives ----

func (r *DealRepo) CreateCreative(ctx context.Context, c *models.DealCreative) error {
	mediaBytes, _ := json.Marshal(c.MediaURLs)
	buttonsBytes, _ := json.Marshal(c.ButtonsJSON)
	return r.pool.QueryRow(ctx, `
		INSERT INTO deal_creatives (deal_id, version, owner_composed_text, advertiser_materials_text, status,
		                            repost_from_chat_id, repost_from_msg_id, repost_from_url, media_urls, buttons_json)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at
	`, c.DealID, c.Version, c.OwnerComposedText, c.AdvertiserMaterialsText, c.Status,
		c.RepostFromChatID, c.RepostFromMsgID, c.RepostFromURL, mediaBytes, buttonsBytes,
	).Scan(&c.ID, &c.CreatedAt)
}

func (r *DealRepo) GetLatestCreative(ctx context.Context, dealID uuid.UUID) (*models.DealCreative, error) {
	var c models.DealCreative
	var mediaBytes, buttonsBytes []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, deal_id, version, owner_composed_text, advertiser_materials_text, status,
		       repost_from_chat_id, repost_from_msg_id, repost_from_url, media_urls, buttons_json, created_at
		FROM deal_creatives WHERE deal_id = $1 ORDER BY version DESC LIMIT 1
	`, dealID).Scan(&c.ID, &c.DealID, &c.Version, &c.OwnerComposedText, &c.AdvertiserMaterialsText, &c.Status,
		&c.RepostFromChatID, &c.RepostFromMsgID, &c.RepostFromURL, &mediaBytes, &buttonsBytes, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(mediaBytes, &c.MediaURLs)
	_ = json.Unmarshal(buttonsBytes, &c.ButtonsJSON)
	return &c, nil
}

func (r *DealRepo) UpdateCreativeStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx, `UPDATE deal_creatives SET status = $1 WHERE id = $2`, status, id)
	return err
}

func (r *DealRepo) GetCreativeMaxVersion(ctx context.Context, dealID uuid.UUID) (int, error) {
	var v *int
	err := r.pool.QueryRow(ctx, `SELECT MAX(version) FROM deal_creatives WHERE deal_id = $1`, dealID).Scan(&v)
	if err != nil || v == nil {
		return 0, err
	}
	return *v, nil
}

// ---- Posts ----

func (r *DealRepo) UpsertPost(ctx context.Context, p *models.DealPost) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO deal_posts (deal_id, telegram_message_id, telegram_chat_id, post_url, content_hash, posted_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (deal_id) DO UPDATE SET
			telegram_message_id = COALESCE(EXCLUDED.telegram_message_id, deal_posts.telegram_message_id),
			telegram_chat_id = COALESCE(EXCLUDED.telegram_chat_id, deal_posts.telegram_chat_id),
			post_url = COALESCE(EXCLUDED.post_url, deal_posts.post_url),
			content_hash = COALESCE(EXCLUDED.content_hash, deal_posts.content_hash),
			posted_at = COALESCE(EXCLUDED.posted_at, deal_posts.posted_at)
		RETURNING id
	`, p.DealID, p.TelegramMessageID, p.TelegramChatID, p.PostURL, p.ContentHash, p.PostedAt).Scan(&p.ID)
}

func (r *DealRepo) GetPost(ctx context.Context, dealID uuid.UUID) (*models.DealPost, error) {
	var p models.DealPost
	err := r.pool.QueryRow(ctx, `
		SELECT id, deal_id, telegram_message_id, telegram_chat_id, post_url, content_hash,
		       posted_at, last_checked_at, is_deleted, is_edited
		FROM deal_posts WHERE deal_id = $1
	`, dealID).Scan(&p.ID, &p.DealID, &p.TelegramMessageID, &p.TelegramChatID, &p.PostURL, &p.ContentHash,
		&p.PostedAt, &p.LastCheckedAt, &p.IsDeleted, &p.IsEdited)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *DealRepo) UpdatePostFlags(ctx context.Context, dealID uuid.UUID, isDeleted, isEdited bool) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE deal_posts SET is_deleted = $1, is_edited = $2, last_checked_at = now() WHERE deal_id = $3
	`, isDeleted, isEdited, dealID)
	return err
}
