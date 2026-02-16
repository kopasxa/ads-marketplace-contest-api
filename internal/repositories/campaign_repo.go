package repositories

import (
	"context"
	"fmt"

	"github.com/ads-marketplace/backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CampaignRepo struct {
	pool *pgxpool.Pool
}

func NewCampaignRepo(pool *pgxpool.Pool) *CampaignRepo {
	return &CampaignRepo{pool: pool}
}

func (r *CampaignRepo) Create(ctx context.Context, c *models.Campaign) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO campaigns (advertiser_user_id, title, target_audience, key_messages, budget_ton, preferred_date, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`, c.AdvertiserUserID, c.Title, c.TargetAudience, c.KeyMessages,
		c.BudgetTON, c.PreferredDate, c.Status,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
}

func (r *CampaignRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Campaign, error) {
	var c models.Campaign
	err := r.pool.QueryRow(ctx, `
		SELECT id, advertiser_user_id, title, target_audience, key_messages,
		       budget_ton, preferred_date, status, created_at, updated_at
		FROM campaigns WHERE id = $1
	`, id).Scan(&c.ID, &c.AdvertiserUserID, &c.Title, &c.TargetAudience,
		&c.KeyMessages, &c.BudgetTON, &c.PreferredDate, &c.Status,
		&c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CampaignRepo) Update(ctx context.Context, c *models.Campaign) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE campaigns SET title = $1, target_audience = $2, key_messages = $3,
		       budget_ton = $4, preferred_date = $5, status = $6, updated_at = now()
		WHERE id = $7
	`, c.Title, c.TargetAudience, c.KeyMessages,
		c.BudgetTON, c.PreferredDate, c.Status, c.ID)
	return err
}

func (r *CampaignRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM campaigns WHERE id = $1`, id)
	return err
}

type CampaignFilter struct {
	AdvertiserUserID *uuid.UUID
	Status           *string
	Limit            int
	Offset           int
}

func (r *CampaignRepo) List(ctx context.Context, f CampaignFilter) ([]models.Campaign, error) {
	query := `
		SELECT id, advertiser_user_id, title, target_audience, key_messages,
		       budget_ton, preferred_date, status, created_at, updated_at
		FROM campaigns
	`
	args := []any{}
	argIdx := 1
	where := []string{}

	if f.AdvertiserUserID != nil {
		where = append(where, fmt.Sprintf("advertiser_user_id = $%d", argIdx))
		args = append(args, *f.AdvertiserUserID)
		argIdx++
	}
	if f.Status != nil {
		where = append(where, fmt.Sprintf("status = $%d", argIdx))
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
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, f.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var campaigns []models.Campaign
	for rows.Next() {
		var c models.Campaign
		if err := rows.Scan(&c.ID, &c.AdvertiserUserID, &c.Title, &c.TargetAudience,
			&c.KeyMessages, &c.BudgetTON, &c.PreferredDate, &c.Status,
			&c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		campaigns = append(campaigns, c)
	}
	return campaigns, nil
}
