package repositories

import (
	"context"
	"time"

	"github.com/ads-marketplace/backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

func (r *UserRepo) UpsertByTelegramID(ctx context.Context, telegramID int64, username, firstName, lastName *string) (*models.User, error) {
	var u models.User
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (telegram_user_id, username, first_name, last_name)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (telegram_user_id) DO UPDATE SET
			username = COALESCE(EXCLUDED.username, users.username),
			first_name = COALESCE(EXCLUDED.first_name, users.first_name),
			last_name = COALESCE(EXCLUDED.last_name, users.last_name),
			last_active_at = now()
		RETURNING id, telegram_user_id, username, first_name, last_name, created_at, last_active_at
	`, telegramID, username, firstName, lastName).Scan(
		&u.ID, &u.TelegramUserID, &u.Username, &u.FirstName, &u.LastName, &u.CreatedAt, &u.LastActiveAt,
	)
	return &u, err
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var u models.User
	err := r.pool.QueryRow(ctx, `
		SELECT id, telegram_user_id, username, first_name, last_name, created_at, last_active_at
		FROM users WHERE id = $1
	`, id).Scan(&u.ID, &u.TelegramUserID, &u.Username, &u.FirstName, &u.LastName, &u.CreatedAt, &u.LastActiveAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) GetByTelegramID(ctx context.Context, telegramID int64) (*models.User, error) {
	var u models.User
	err := r.pool.QueryRow(ctx, `
		SELECT id, telegram_user_id, username, first_name, last_name, created_at, last_active_at
		FROM users WHERE telegram_user_id = $1
	`, telegramID).Scan(&u.ID, &u.TelegramUserID, &u.Username, &u.FirstName, &u.LastName, &u.CreatedAt, &u.LastActiveAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) UpdateLastActive(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET last_active_at = $1 WHERE id = $2`, time.Now(), id)
	return err
}

func (r *UserRepo) GetActiveUserIDs(ctx context.Context, since time.Time) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `SELECT id FROM users WHERE last_active_at > $1`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}
