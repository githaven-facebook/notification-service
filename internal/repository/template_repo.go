package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nicedavid98/notification-service/internal/model"
)

// TemplateRepository defines the interface for template persistence operations.
type TemplateRepository interface {
	Create(ctx context.Context, t *model.NotificationTemplate) error
	GetByID(ctx context.Context, id int64) (*model.NotificationTemplate, error)
	GetByNameAndLocale(ctx context.Context, name string, channel model.NotificationChannel, locale string) (*model.NotificationTemplate, error)
	GetLatestVersion(ctx context.Context, name string, channel model.NotificationChannel, locale string) (*model.NotificationTemplate, error)
	List(ctx context.Context, channel model.NotificationChannel, limit, offset int) ([]*model.NotificationTemplate, error)
	Update(ctx context.Context, id int64, req *model.UpdateTemplateRequest) error
	Delete(ctx context.Context, id int64) error
}

// postgresTemplateRepo is the PostgreSQL implementation of TemplateRepository.
type postgresTemplateRepo struct {
	pool *pgxpool.Pool
}

// NewTemplateRepository creates a new PostgreSQL template repository.
func NewTemplateRepository(pool *pgxpool.Pool) TemplateRepository {
	return &postgresTemplateRepo{pool: pool}
}

func (r *postgresTemplateRepo) Create(ctx context.Context, t *model.NotificationTemplate) error {
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()

	query := `
		INSERT INTO notification_templates (name, channel, subject, body, locale, version, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`

	err := r.pool.QueryRow(ctx, query,
		t.Name, t.Channel, t.Subject, t.Body, t.Locale, t.Version, t.Active,
		t.CreatedAt, t.UpdatedAt,
	).Scan(&t.ID)
	if err != nil {
		return fmt.Errorf("insert template: %w", err)
	}
	return nil
}

func (r *postgresTemplateRepo) GetByID(ctx context.Context, id int64) (*model.NotificationTemplate, error) {
	query := `
		SELECT id, name, channel, subject, body, locale, version, active, created_at, updated_at
		FROM notification_templates WHERE id = $1`

	row := r.pool.QueryRow(ctx, query, id)
	t, err := scanTemplate(row)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (r *postgresTemplateRepo) GetByNameAndLocale(ctx context.Context, name string, channel model.NotificationChannel, locale string) (*model.NotificationTemplate, error) {
	query := `
		SELECT id, name, channel, subject, body, locale, version, active, created_at, updated_at
		FROM notification_templates
		WHERE name = $1 AND channel = $2 AND locale = $3 AND active = true
		ORDER BY version DESC LIMIT 1`

	row := r.pool.QueryRow(ctx, query, name, channel, locale)
	return scanTemplate(row)
}

func (r *postgresTemplateRepo) GetLatestVersion(ctx context.Context, name string, channel model.NotificationChannel, locale string) (*model.NotificationTemplate, error) {
	query := `
		SELECT id, name, channel, subject, body, locale, version, active, created_at, updated_at
		FROM notification_templates
		WHERE name = $1 AND channel = $2 AND locale = $3
		ORDER BY version DESC LIMIT 1`

	row := r.pool.QueryRow(ctx, query, name, channel, locale)
	return scanTemplate(row)
}

func (r *postgresTemplateRepo) List(ctx context.Context, channel model.NotificationChannel, limit, offset int) ([]*model.NotificationTemplate, error) {
	var query string
	var args []interface{}

	if channel != "" {
		query = `
			SELECT id, name, channel, subject, body, locale, version, active, created_at, updated_at
			FROM notification_templates WHERE channel = $1
			ORDER BY name, locale, version DESC LIMIT $2 OFFSET $3`
		args = []interface{}{channel, limit, offset}
	} else {
		query = `
			SELECT id, name, channel, subject, body, locale, version, active, created_at, updated_at
			FROM notification_templates
			ORDER BY name, locale, version DESC LIMIT $1 OFFSET $2`
		args = []interface{}{limit, offset}
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	defer rows.Close()

	var templates []*model.NotificationTemplate
	for rows.Next() {
		t := &model.NotificationTemplate{}
		err := rows.Scan(
			&t.ID, &t.Name, &t.Channel, &t.Subject, &t.Body,
			&t.Locale, &t.Version, &t.Active, &t.CreatedAt, &t.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan template row: %w", err)
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate template rows: %w", err)
	}
	return templates, nil
}

func (r *postgresTemplateRepo) Update(ctx context.Context, id int64, req *model.UpdateTemplateRequest) error {
	query := `
		UPDATE notification_templates
		SET subject = COALESCE(NULLIF($1, ''), subject),
		    body = COALESCE(NULLIF($2, ''), body),
		    active = COALESCE($3, active),
		    updated_at = $4
		WHERE id = $5`

	_, err := r.pool.Exec(ctx, query, req.Subject, req.Body, req.Active, time.Now(), id)
	if err != nil {
		return fmt.Errorf("update template: %w", err)
	}
	return nil
}

func (r *postgresTemplateRepo) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM notification_templates WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete template: %w", err)
	}
	return nil
}

func scanTemplate(row pgx.Row) (*model.NotificationTemplate, error) {
	t := &model.NotificationTemplate{}
	err := row.Scan(
		&t.ID, &t.Name, &t.Channel, &t.Subject, &t.Body,
		&t.Locale, &t.Version, &t.Active, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan template: %w", err)
	}
	return t, nil
}
