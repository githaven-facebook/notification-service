package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nicedavid98/notification-service/internal/model"
)

// PreferenceRepository defines the interface for user preference persistence operations.
type PreferenceRepository interface {
	GetByUserID(ctx context.Context, userID string) ([]*model.UserPreference, error)
	GetByUserAndChannel(ctx context.Context, userID string, channel model.NotificationChannel) (*model.UserPreference, error)
	Upsert(ctx context.Context, pref *model.UserPreference) error
	Delete(ctx context.Context, userID string, channel model.NotificationChannel) error
}

// postgresPreferenceRepo is the PostgreSQL implementation of PreferenceRepository.
type postgresPreferenceRepo struct {
	pool *pgxpool.Pool
}

// NewPreferenceRepository creates a new PostgreSQL preference repository.
func NewPreferenceRepository(pool *pgxpool.Pool) PreferenceRepository {
	return &postgresPreferenceRepo{pool: pool}
}

func (r *postgresPreferenceRepo) GetByUserID(ctx context.Context, userID string) ([]*model.UserPreference, error) {
	query := `
		SELECT id, user_id, channel, enabled, quiet_hours_start, quiet_hours_end,
		       digest_mode, frequency, created_at, updated_at
		FROM user_preferences WHERE user_id = $1
		ORDER BY channel`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query preferences by user: %w", err)
	}
	defer rows.Close()

	var prefs []*model.UserPreference
	for rows.Next() {
		p, err := scanPreference(rows)
		if err != nil {
			return nil, err
		}
		prefs = append(prefs, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate preference rows: %w", err)
	}
	return prefs, nil
}

func (r *postgresPreferenceRepo) GetByUserAndChannel(ctx context.Context, userID string, channel model.NotificationChannel) (*model.UserPreference, error) {
	query := `
		SELECT id, user_id, channel, enabled, quiet_hours_start, quiet_hours_end,
		       digest_mode, frequency, created_at, updated_at
		FROM user_preferences WHERE user_id = $1 AND channel = $2`

	row := r.pool.QueryRow(ctx, query, userID, channel)
	p := &model.UserPreference{}
	err := row.Scan(
		&p.ID, &p.UserID, &p.Channel, &p.Enabled,
		&p.QuietHoursStart, &p.QuietHoursEnd,
		&p.DigestMode, &p.Frequency, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan preference: %w", err)
	}
	return p, nil
}

func (r *postgresPreferenceRepo) Upsert(ctx context.Context, pref *model.UserPreference) error {
	pref.UpdatedAt = time.Now()
	if pref.CreatedAt.IsZero() {
		pref.CreatedAt = pref.UpdatedAt
	}

	query := `
		INSERT INTO user_preferences (user_id, channel, enabled, quiet_hours_start, quiet_hours_end,
		                              digest_mode, frequency, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (user_id, channel) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			quiet_hours_start = EXCLUDED.quiet_hours_start,
			quiet_hours_end = EXCLUDED.quiet_hours_end,
			digest_mode = EXCLUDED.digest_mode,
			frequency = EXCLUDED.frequency,
			updated_at = EXCLUDED.updated_at
		RETURNING id`

	err := r.pool.QueryRow(ctx, query,
		pref.UserID, pref.Channel, pref.Enabled,
		pref.QuietHoursStart, pref.QuietHoursEnd,
		pref.DigestMode, pref.Frequency,
		pref.CreatedAt, pref.UpdatedAt,
	).Scan(&pref.ID)
	if err != nil {
		return fmt.Errorf("upsert preference: %w", err)
	}
	return nil
}

func (r *postgresPreferenceRepo) Delete(ctx context.Context, userID string, channel model.NotificationChannel) error {
	query := `DELETE FROM user_preferences WHERE user_id = $1 AND channel = $2`
	_, err := r.pool.Exec(ctx, query, userID, channel)
	if err != nil {
		return fmt.Errorf("delete preference: %w", err)
	}
	return nil
}

func scanPreference(rows pgx.Rows) (*model.UserPreference, error) {
	p := &model.UserPreference{}
	err := rows.Scan(
		&p.ID, &p.UserID, &p.Channel, &p.Enabled,
		&p.QuietHoursStart, &p.QuietHoursEnd,
		&p.DigestMode, &p.Frequency, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan preference row: %w", err)
	}
	return p, nil
}
