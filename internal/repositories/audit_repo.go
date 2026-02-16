package repositories

import (
	"context"

	"github.com/ads-marketplace/backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditRepo struct {
	pool *pgxpool.Pool
}

func NewAuditRepo(pool *pgxpool.Pool) *AuditRepo {
	return &AuditRepo{pool: pool}
}

func (r *AuditRepo) Log(ctx context.Context, entry models.AuditLog) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO audit_log (actor_user_id, actor_type, action, entity_type, entity_id, meta)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, entry.ActorUserID, entry.ActorType, entry.Action, entry.EntityType, entry.EntityID, entry.Meta)
	return err
}

func (r *AuditRepo) GetByEntity(ctx context.Context, entityType string, entityID uuid.UUID, limit, offset int) ([]models.AuditLog, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, actor_user_id, actor_type, action, entity_type, entity_id, meta, created_at
		FROM audit_log WHERE entity_type = $1 AND entity_id = $2
		ORDER BY created_at DESC LIMIT $3 OFFSET $4
	`, entityType, entityID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var l models.AuditLog
		if err := rows.Scan(&l.ID, &l.ActorUserID, &l.ActorType, &l.Action, &l.EntityType, &l.EntityID, &l.Meta, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, nil
}
