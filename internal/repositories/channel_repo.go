package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ads-marketplace/backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ChannelRepo struct {
	pool *pgxpool.Pool
}

func NewChannelRepo(pool *pgxpool.Pool) *ChannelRepo {
	return &ChannelRepo{pool: pool}
}

func (r *ChannelRepo) Create(ctx context.Context, ch *models.Channel) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO channels (username, title, added_by_user_id, bot_status)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at
	`, ch.Username, ch.Title, ch.AddedByUserID, ch.BotStatus).Scan(&ch.ID, &ch.CreatedAt, &ch.UpdatedAt)
}

func (r *ChannelRepo) UpsertByUsername(ctx context.Context, ch *models.Channel) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO channels (username, telegram_chat_id, title, added_by_user_id, bot_status, bot_added_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (username) DO UPDATE SET
			telegram_chat_id = COALESCE(EXCLUDED.telegram_chat_id, channels.telegram_chat_id),
			title = COALESCE(EXCLUDED.title, channels.title),
			added_by_user_id = COALESCE(EXCLUDED.added_by_user_id, channels.added_by_user_id),
			bot_status = EXCLUDED.bot_status,
			bot_added_at = COALESCE(EXCLUDED.bot_added_at, channels.bot_added_at),
			updated_at = now()
		RETURNING id, created_at, updated_at
	`, ch.Username, ch.TelegramChatID, ch.Title, ch.AddedByUserID, ch.BotStatus, ch.BotAddedAt).Scan(&ch.ID, &ch.CreatedAt, &ch.UpdatedAt)
}

func (r *ChannelRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Channel, error) {
	var ch models.Channel
	err := r.pool.QueryRow(ctx, `
		SELECT id, telegram_chat_id, username, title, added_by_user_id, bot_status, userbot_status,
		       bot_added_at, bot_removed_at, created_at, updated_at
		FROM channels WHERE id = $1
	`, id).Scan(&ch.ID, &ch.TelegramChatID, &ch.Username, &ch.Title, &ch.AddedByUserID,
		&ch.BotStatus, &ch.UserbotStatus, &ch.BotAddedAt, &ch.BotRemovedAt, &ch.CreatedAt, &ch.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &ch, nil
}

func (r *ChannelRepo) GetByUsername(ctx context.Context, username string) (*models.Channel, error) {
	var ch models.Channel
	err := r.pool.QueryRow(ctx, `
		SELECT id, telegram_chat_id, username, title, added_by_user_id, bot_status, userbot_status,
		       bot_added_at, bot_removed_at, created_at, updated_at
		FROM channels WHERE username = $1
	`, username).Scan(&ch.ID, &ch.TelegramChatID, &ch.Username, &ch.Title, &ch.AddedByUserID,
		&ch.BotStatus, &ch.UserbotStatus, &ch.BotAddedAt, &ch.BotRemovedAt, &ch.CreatedAt, &ch.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &ch, nil
}

func (r *ChannelRepo) UpdateUserbotStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx, `UPDATE channels SET userbot_status = $1, updated_at = now() WHERE id = $2`, status, id)
	return err
}

func (r *ChannelRepo) UpdateBotStatus(ctx context.Context, id uuid.UUID, status string) error {
	query := `UPDATE channels SET bot_status = $1, updated_at = now()`
	if status == "removed" {
		query += `, bot_removed_at = now()`
	}
	query += ` WHERE id = $1`
	// Fix: use correct param index
	if status == "removed" {
		_, err := r.pool.Exec(ctx, `UPDATE channels SET bot_status = $1, bot_removed_at = now(), updated_at = now() WHERE id = $2`, status, id)
		return err
	}
	_, err := r.pool.Exec(ctx, `UPDATE channels SET bot_status = $1, updated_at = now() WHERE id = $2`, status, id)
	return err
}

type ChannelFilter struct {
	MinSubscribers *int
	MaxSubscribers *int
	MinAvgViews    *int
	MaxPriceTON    *string
	LangGuess      *string
	Category       *string
	Language       *string
	Status         *string // listing status
	Limit          int
	Offset         int
}

func (r *ChannelRepo) Search(ctx context.Context, f ChannelFilter) ([]models.Channel, error) {
	query := `
		SELECT c.id, c.telegram_chat_id, c.username, c.title, c.added_by_user_id, c.bot_status, c.userbot_status,
		       c.bot_added_at, c.bot_removed_at, c.created_at, c.updated_at
		FROM channels c
		LEFT JOIN channel_listings cl ON cl.channel_id = c.id
		LEFT JOIN LATERAL (
			SELECT subscribers, avg_views_20 FROM channel_stats_snapshots
			WHERE channel_id = c.id ORDER BY fetched_at DESC LIMIT 1
		) ss ON true
		WHERE c.bot_status = 'active'
	`
	args := []any{}
	argIdx := 1

	if f.Status != nil {
		query += fmt.Sprintf(" AND cl.status = $%d", argIdx)
		args = append(args, *f.Status)
		argIdx++
	} else {
		query += fmt.Sprintf(" AND cl.status = $%d", argIdx)
		args = append(args, "active")
		argIdx++
	}

	if f.MinSubscribers != nil {
		query += fmt.Sprintf(" AND ss.subscribers >= $%d", argIdx)
		args = append(args, *f.MinSubscribers)
		argIdx++
	}
	if f.MaxSubscribers != nil {
		query += fmt.Sprintf(" AND ss.subscribers <= $%d", argIdx)
		args = append(args, *f.MaxSubscribers)
		argIdx++
	}
	if f.MinAvgViews != nil {
		query += fmt.Sprintf(" AND ss.avg_views_20 >= $%d", argIdx)
		args = append(args, *f.MinAvgViews)
		argIdx++
	}

	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	query += fmt.Sprintf(" ORDER BY c.created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, f.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanChannels(rows)
}

func (r *ChannelRepo) GetByUserID(ctx context.Context, userID uuid.UUID) ([]models.Channel, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT c.id, c.telegram_chat_id, c.username, c.title, c.added_by_user_id, c.bot_status, c.userbot_status,
		       c.bot_added_at, c.bot_removed_at, c.created_at, c.updated_at
		FROM channels c
		LEFT JOIN channel_members cm ON cm.channel_id = c.id
		WHERE c.added_by_user_id = $1 OR cm.user_id = $1
		ORDER BY c.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChannels(rows)
}

func (r *ChannelRepo) GetActiveChannelsWithRecentUsers(ctx context.Context) ([]models.Channel, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT c.id, c.telegram_chat_id, c.username, c.title, c.added_by_user_id, c.bot_status, c.userbot_status,
		       c.bot_added_at, c.bot_removed_at, c.created_at, c.updated_at
		FROM channels c
		JOIN channel_members cm ON cm.channel_id = c.id
		JOIN users u ON u.id = cm.user_id
		WHERE c.bot_status = 'active'
		  AND u.last_active_at > now() - interval '48 hours'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChannels(rows)
}

func scanChannels(rows pgx.Rows) ([]models.Channel, error) {
	var channels []models.Channel
	for rows.Next() {
		var ch models.Channel
		if err := rows.Scan(&ch.ID, &ch.TelegramChatID, &ch.Username, &ch.Title, &ch.AddedByUserID,
			&ch.BotStatus, &ch.UserbotStatus, &ch.BotAddedAt, &ch.BotRemovedAt, &ch.CreatedAt, &ch.UpdatedAt); err != nil {
			return nil, err
		}
		channels = append(channels, ch)
	}
	return channels, nil
}

// ---- Explore (enriched channels) ----

type ExploreChannelRow struct {
	ID             uuid.UUID
	Username       string
	Title          *string
	BotStatus      string
	Subscribers    *int
	AvgViews       *int
	ERPercent      *float64
	ListingStatus  *string
	PricePostTON   *string
	PriceRepostTON *string
	PriceStoryTON  *string
	Description    *string
	Category       *string
	Language       *string
}

func (r *ChannelRepo) SearchExplore(ctx context.Context, f ChannelFilter) ([]ExploreChannelRow, error) {
	query := `
		SELECT c.id, c.username, c.title, c.bot_status,
		       ss.subscribers, ss.avg_views_20, ss.er_percent,
		       cl.status AS listing_status,
		       cl.price_post_ton, cl.price_repost_ton, cl.price_story_ton, cl.description,
		       cl.category, cl.language
		FROM channels c
		LEFT JOIN channel_listings cl ON cl.channel_id = c.id
		LEFT JOIN LATERAL (
			SELECT subscribers, avg_views_20, er_percent FROM channel_stats_snapshots
			WHERE channel_id = c.id ORDER BY fetched_at DESC LIMIT 1
		) ss ON true
		WHERE c.bot_status = 'active'
	`
	args := []any{}
	argIdx := 1

	if f.Status != nil {
		query += fmt.Sprintf(" AND cl.status = $%d", argIdx)
		args = append(args, *f.Status)
		argIdx++
	} else {
		query += fmt.Sprintf(" AND cl.status = $%d", argIdx)
		args = append(args, "active")
		argIdx++
	}
	if f.MinSubscribers != nil {
		query += fmt.Sprintf(" AND ss.subscribers >= $%d", argIdx)
		args = append(args, *f.MinSubscribers)
		argIdx++
	}
	if f.MaxSubscribers != nil {
		query += fmt.Sprintf(" AND ss.subscribers <= $%d", argIdx)
		args = append(args, *f.MaxSubscribers)
		argIdx++
	}
	if f.MinAvgViews != nil {
		query += fmt.Sprintf(" AND ss.avg_views_20 >= $%d", argIdx)
		args = append(args, *f.MinAvgViews)
		argIdx++
	}
	if f.Category != nil {
		query += fmt.Sprintf(" AND cl.category = $%d", argIdx)
		args = append(args, *f.Category)
		argIdx++
	}
	if f.Language != nil {
		query += fmt.Sprintf(" AND cl.language = $%d", argIdx)
		args = append(args, *f.Language)
		argIdx++
	}

	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	query += fmt.Sprintf(" ORDER BY c.created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, f.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ExploreChannelRow
	for rows.Next() {
		var row ExploreChannelRow
		if err := rows.Scan(&row.ID, &row.Username, &row.Title, &row.BotStatus,
			&row.Subscribers, &row.AvgViews, &row.ERPercent,
			&row.ListingStatus, &row.PricePostTON, &row.PriceRepostTON, &row.PriceStoryTON, &row.Description,
			&row.Category, &row.Language,
		); err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	return results, nil
}

// ---- Channel Members ----

func (r *ChannelRepo) AddMember(ctx context.Context, m *models.ChannelMember) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO channel_members (channel_id, user_id, role, can_post, last_admin_check_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (channel_id, user_id) DO UPDATE SET
			role = EXCLUDED.role, can_post = EXCLUDED.can_post, last_admin_check_at = now()
		RETURNING id
	`, m.ChannelID, m.UserID, m.Role, m.CanPost).Scan(&m.ID)
}

func (r *ChannelRepo) GetMembers(ctx context.Context, channelID uuid.UUID) ([]models.ChannelMember, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, channel_id, user_id, role, can_post, last_admin_check_at
		FROM channel_members WHERE channel_id = $1
	`, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []models.ChannelMember
	for rows.Next() {
		var m models.ChannelMember
		if err := rows.Scan(&m.ID, &m.ChannelID, &m.UserID, &m.Role, &m.CanPost, &m.LastAdminCheckAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, nil
}

func (r *ChannelRepo) CountMembers(ctx context.Context, channelID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM channel_members WHERE channel_id = $1`, channelID).Scan(&count)
	return count, err
}

func (r *ChannelRepo) GetMemberByUserAndChannel(ctx context.Context, channelID, userID uuid.UUID) (*models.ChannelMember, error) {
	var m models.ChannelMember
	err := r.pool.QueryRow(ctx, `
		SELECT id, channel_id, user_id, role, can_post, last_admin_check_at
		FROM channel_members WHERE channel_id = $1 AND user_id = $2
	`, channelID, userID).Scan(&m.ID, &m.ChannelID, &m.UserID, &m.Role, &m.CanPost, &m.LastAdminCheckAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ---- Listings ----

func (r *ChannelRepo) UpsertListing(ctx context.Context, l *models.ChannelListing) error {
	pricingBytes, err := json.Marshal(l.PricingJSON)
	if err != nil {
		return err
	}
	return r.pool.QueryRow(ctx, `
		INSERT INTO channel_listings (
			channel_id, status, pricing_json, min_lead_time_minutes, description,
			category, language,
			price_post_ton, price_repost_ton, price_story_ton, formats_enabled,
			hold_hours_post, hold_hours_repost, hold_hours_story, auto_accept
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (channel_id) DO UPDATE SET
			status = EXCLUDED.status,
			pricing_json = EXCLUDED.pricing_json,
			min_lead_time_minutes = EXCLUDED.min_lead_time_minutes,
			description = EXCLUDED.description,
			category = EXCLUDED.category,
			language = EXCLUDED.language,
			price_post_ton = EXCLUDED.price_post_ton,
			price_repost_ton = EXCLUDED.price_repost_ton,
			price_story_ton = EXCLUDED.price_story_ton,
			formats_enabled = EXCLUDED.formats_enabled,
			hold_hours_post = EXCLUDED.hold_hours_post,
			hold_hours_repost = EXCLUDED.hold_hours_repost,
			hold_hours_story = EXCLUDED.hold_hours_story,
			auto_accept = EXCLUDED.auto_accept,
			updated_at = now()
		RETURNING id, created_at, updated_at
	`, l.ChannelID, l.Status, pricingBytes, l.MinLeadTimeMinutes, l.Description,
		l.Category, l.Language,
		l.PricePostTON, l.PriceRepostTON, l.PriceStoryTON, l.FormatsEnabled,
		l.HoldHoursPost, l.HoldHoursRepost, l.HoldHoursStory, l.AutoAccept,
	).Scan(&l.ID, &l.CreatedAt, &l.UpdatedAt)
}

func (r *ChannelRepo) GetListing(ctx context.Context, channelID uuid.UUID) (*models.ChannelListing, error) {
	var l models.ChannelListing
	var pricingBytes []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, channel_id, status, pricing_json, min_lead_time_minutes, description,
		       category, language,
		       price_post_ton, price_repost_ton, price_story_ton, formats_enabled,
		       hold_hours_post, hold_hours_repost, hold_hours_story, auto_accept,
		       created_at, updated_at
		FROM channel_listings WHERE channel_id = $1
	`, channelID).Scan(
		&l.ID, &l.ChannelID, &l.Status, &pricingBytes, &l.MinLeadTimeMinutes, &l.Description,
		&l.Category, &l.Language,
		&l.PricePostTON, &l.PriceRepostTON, &l.PriceStoryTON, &l.FormatsEnabled,
		&l.HoldHoursPost, &l.HoldHoursRepost, &l.HoldHoursStory, &l.AutoAccept,
		&l.CreatedAt, &l.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(pricingBytes, &l.PricingJSON)
	return &l, nil
}

// ---- Stats ----

func (r *ChannelRepo) InsertStatsSnapshot(ctx context.Context, s *models.ChannelStatsSnapshot) error {
	rawBytes, _ := json.Marshal(s.RawJSON)
	if s.Source == "" {
		s.Source = "tme_parser"
	}
	return r.pool.QueryRow(ctx, `
		INSERT INTO channel_stats_snapshots (channel_id, subscribers, verified_badge, avg_views_20, last_post_id, raw_json, premium_count,
		                                     source, members_online, admins_count, growth_7d, growth_30d, posts_count,
		                                     views_per_post, shares_per_post, enabled_notifications_percent, er_percent)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		RETURNING id, fetched_at
	`, s.ChannelID, s.Subscribers, s.VerifiedBadge, s.AvgViews20, s.LastPostID, rawBytes, s.PremiumCount,
		s.Source, s.MembersOnline, s.AdminsCount, s.Growth7d, s.Growth30d, s.PostsCount,
		s.ViewsPerPost, s.SharesPerPost, s.EnabledNotificationsPercent, s.ERPercent).Scan(&s.ID, &s.FetchedAt)
}

func (r *ChannelRepo) GetLatestStats(ctx context.Context, channelID uuid.UUID) (*models.ChannelStatsSnapshot, error) {
	var s models.ChannelStatsSnapshot
	var rawBytes []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, channel_id, fetched_at, subscribers, verified_badge, avg_views_20, last_post_id, raw_json, premium_count,
		       source, members_online, admins_count, growth_7d, growth_30d, posts_count,
		       views_per_post, shares_per_post, enabled_notifications_percent, er_percent
		FROM channel_stats_snapshots WHERE channel_id = $1 ORDER BY fetched_at DESC LIMIT 1
	`, channelID).Scan(&s.ID, &s.ChannelID, &s.FetchedAt, &s.Subscribers, &s.VerifiedBadge, &s.AvgViews20, &s.LastPostID, &rawBytes, &s.PremiumCount,
		&s.Source, &s.MembersOnline, &s.AdminsCount, &s.Growth7d, &s.Growth30d, &s.PostsCount,
		&s.ViewsPerPost, &s.SharesPerPost, &s.EnabledNotificationsPercent, &s.ERPercent)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(rawBytes, &s.RawJSON)
	return &s, nil
}

// --- helper to normalise username ---
func NormalizeUsername(u string) string {
	u = strings.TrimPrefix(u, "@")
	u = strings.TrimPrefix(u, "https://t.me/")
	u = strings.TrimPrefix(u, "http://t.me/")
	return strings.ToLower(strings.TrimSpace(u))
}
