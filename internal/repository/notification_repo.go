package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nicedavid98/notification-service/internal/model"
)

// NotificationRepository defines the interface for notification persistence operations.
type NotificationRepository interface {
	Create(ctx context.Context, n *model.Notification) error
	BatchCreate(ctx context.Context, notifications []*model.Notification) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Notification, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.NotificationStatus, errMsg string) error
	UpdateSentAt(ctx context.Context, id uuid.UUID, sentAt time.Time) error
	UpdateDeliveredAt(ctx context.Context, id uuid.UUID, deliveredAt time.Time) error
	IncrementRetryCount(ctx context.Context, id uuid.UUID) error
	GetByUserID(ctx context.Context, userID string, limit, offset int) ([]*model.Notification, error)
	GetByStatus(ctx context.Context, status model.NotificationStatus, limit int) ([]*model.Notification, error)
	GetByUserAndType(ctx context.Context, userID string, notifType model.NotificationType, limit int) ([]*model.Notification, error)
	GetPendingForRetry(ctx context.Context, maxRetries, limit int) ([]*model.Notification, error)
}

// postgresNotificationRepo is the PostgreSQL implementation of NotificationRepository.
type postgresNotificationRepo struct {
	pool *pgxpool.Pool
}

// NewNotificationRepository creates a new PostgreSQL notification repository.
func NewNotificationRepository(pool *pgxpool.Pool) NotificationRepository {
	return &postgresNotificationRepo{pool: pool}
}

func (r *postgresNotificationRepo) Create(ctx context.Context, n *model.Notification) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	n.CreatedAt = time.Now()
	n.UpdatedAt = time.Now()

	dataJSON, err := marshalJSON(n.Data)
	if err != nil {
		return fmt.Errorf("marshal data: %w", err)
	}
	paramsJSON, err := marshalJSON(n.TemplateParams)
	if err != nil {
		return fmt.Errorf("marshal template_params: %w", err)
	}

	query := `
		INSERT INTO notifications (
			id, user_id, type, channel, title, body, data, priority, status,
			template_id, template_params, device_token, recipient,
			error_message, retry_count, deduplication_key, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9,
			$10, $11, $12, $13, $14, $15, $16, $17, $18
		)`

	_, err = r.pool.Exec(ctx, query,
		n.ID, n.UserID, n.Type, n.Channel, n.Title, n.Body,
		dataJSON, n.Priority, n.Status,
		n.TemplateID, paramsJSON, n.DeviceToken, n.Recipient,
		n.ErrorMessage, n.RetryCount, n.DeduplicationKey,
		n.CreatedAt, n.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	return nil
}

func (r *postgresNotificationRepo) BatchCreate(ctx context.Context, notifications []*model.Notification) error {
	if len(notifications) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	query := `
		INSERT INTO notifications (
			id, user_id, type, channel, title, body, data, priority, status,
			template_id, template_params, device_token, recipient,
			error_message, retry_count, deduplication_key, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9,
			$10, $11, $12, $13, $14, $15, $16, $17, $18
		)`

	now := time.Now()
	for i := range notifications {
		n := &notifications[i]
		if n.ID == uuid.Nil {
			n.ID = uuid.New()
		}
		n.CreatedAt = now
		n.UpdatedAt = now

		dataJSON, _ := marshalJSON(n.Data)
		paramsJSON, _ := marshalJSON(n.TemplateParams)

		batch.Queue(query,
			n.ID, n.UserID, n.Type, n.Channel, n.Title, n.Body,
			dataJSON, n.Priority, n.Status,
			n.TemplateID, paramsJSON, n.DeviceToken, n.Recipient,
			n.ErrorMessage, n.RetryCount, n.DeduplicationKey,
			n.CreatedAt, n.UpdatedAt,
		)
	}

	results := r.pool.SendBatch(ctx, batch)
	defer results.Close()

	for i := 0; i < len(notifications); i++ {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("batch insert notification[%d]: %w", i, err)
		}
	}
	return nil
}

func (r *postgresNotificationRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Notification, error) {
	query := `
		SELECT id, user_id, type, channel, title, body, data, priority, status,
		       template_id, template_params, device_token, recipient,
		       error_message, retry_count, deduplication_key,
		       created_at, sent_at, delivered_at, updated_at
		FROM notifications WHERE id = $1`

	row := r.pool.QueryRow(ctx, query, id)
	return scanNotification(row)
}

func (r *postgresNotificationRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status model.NotificationStatus, errMsg string) error {
	query := `UPDATE notifications SET status = $1, error_message = $2, updated_at = $3 WHERE id = $4`
	_, err := r.pool.Exec(ctx, query, status, errMsg, time.Now(), id)
	if err != nil {
		return fmt.Errorf("update notification status: %w", err)
	}
	return nil
}

func (r *postgresNotificationRepo) UpdateSentAt(ctx context.Context, id uuid.UUID, sentAt time.Time) error {
	query := `UPDATE notifications SET sent_at = $1, updated_at = $2 WHERE id = $3`
	_, err := r.pool.Exec(ctx, query, sentAt, time.Now(), id)
	if err != nil {
		return fmt.Errorf("update notification sent_at: %w", err)
	}
	return nil
}

func (r *postgresNotificationRepo) UpdateDeliveredAt(ctx context.Context, id uuid.UUID, deliveredAt time.Time) error {
	query := `UPDATE notifications SET delivered_at = $1, status = $2, updated_at = $3 WHERE id = $4`
	_, err := r.pool.Exec(ctx, query, deliveredAt, model.StatusDelivered, time.Now(), id)
	if err != nil {
		return fmt.Errorf("update notification delivered_at: %w", err)
	}
	return nil
}

func (r *postgresNotificationRepo) IncrementRetryCount(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE notifications SET retry_count = retry_count + 1, updated_at = $1 WHERE id = $2`
	_, err := r.pool.Exec(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("increment retry count: %w", err)
	}
	return nil
}

func (r *postgresNotificationRepo) GetByUserID(ctx context.Context, userID string, limit, offset int) ([]*model.Notification, error) {
	query := `
		SELECT id, user_id, type, channel, title, body, data, priority, status,
		       template_id, template_params, device_token, recipient,
		       error_message, retry_count, deduplication_key,
		       created_at, sent_at, delivered_at, updated_at
		FROM notifications WHERE user_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query notifications by user: %w", err)
	}
	defer rows.Close()

	return scanNotifications(rows)
}

func (r *postgresNotificationRepo) GetByStatus(ctx context.Context, status model.NotificationStatus, limit int) ([]*model.Notification, error) {
	query := `
		SELECT id, user_id, type, channel, title, body, data, priority, status,
		       template_id, template_params, device_token, recipient,
		       error_message, retry_count, deduplication_key,
		       created_at, sent_at, delivered_at, updated_at
		FROM notifications WHERE status = $1
		ORDER BY created_at ASC LIMIT $2`

	rows, err := r.pool.Query(ctx, query, status, limit)
	if err != nil {
		return nil, fmt.Errorf("query notifications by status: %w", err)
	}
	defer rows.Close()

	return scanNotifications(rows)
}

func (r *postgresNotificationRepo) GetByUserAndType(ctx context.Context, userID string, notifType model.NotificationType, limit int) ([]*model.Notification, error) {
	query := `
		SELECT id, user_id, type, channel, title, body, data, priority, status,
		       template_id, template_params, device_token, recipient,
		       error_message, retry_count, deduplication_key,
		       created_at, sent_at, delivered_at, updated_at
		FROM notifications WHERE user_id = $1 AND type = $2
		ORDER BY created_at DESC LIMIT $3`

	rows, err := r.pool.Query(ctx, query, userID, notifType, limit)
	if err != nil {
		return nil, fmt.Errorf("query notifications by user and type: %w", err)
	}
	defer rows.Close()

	return scanNotifications(rows)
}

func (r *postgresNotificationRepo) GetPendingForRetry(ctx context.Context, maxRetries, limit int) ([]*model.Notification, error) {
	query := `
		SELECT id, user_id, type, channel, title, body, data, priority, status,
		       template_id, template_params, device_token, recipient,
		       error_message, retry_count, deduplication_key,
		       created_at, sent_at, delivered_at, updated_at
		FROM notifications
		WHERE status = $1 AND retry_count < $2
		ORDER BY created_at ASC LIMIT $3`

	rows, err := r.pool.Query(ctx, query, model.StatusFailed, maxRetries, limit)
	if err != nil {
		return nil, fmt.Errorf("query notifications for retry: %w", err)
	}
	defer rows.Close()

	return scanNotifications(rows)
}

func scanNotification(row pgx.Row) (*model.Notification, error) {
	var n model.Notification
	var dataJSON, paramsJSON []byte

	err := row.Scan(
		&n.ID, &n.UserID, &n.Type, &n.Channel, &n.Title, &n.Body,
		&dataJSON, &n.Priority, &n.Status,
		&n.TemplateID, &paramsJSON, &n.DeviceToken, &n.Recipient,
		&n.ErrorMessage, &n.RetryCount, &n.DeduplicationKey,
		&n.CreatedAt, &n.SentAt, &n.DeliveredAt, &n.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan notification: %w", err)
	}

	if len(dataJSON) > 0 {
		if err := json.Unmarshal(dataJSON, &n.Data); err != nil {
			return nil, fmt.Errorf("unmarshal data: %w", err)
		}
	}
	if len(paramsJSON) > 0 {
		if err := json.Unmarshal(paramsJSON, &n.TemplateParams); err != nil {
			return nil, fmt.Errorf("unmarshal template_params: %w", err)
		}
	}

	return &n, nil
}

func scanNotifications(rows pgx.Rows) ([]*model.Notification, error) {
	var notifications []*model.Notification
	for rows.Next() {
		var n model.Notification
		var dataJSON, paramsJSON []byte

		err := rows.Scan(
			&n.ID, &n.UserID, &n.Type, &n.Channel, &n.Title, &n.Body,
			&dataJSON, &n.Priority, &n.Status,
			&n.TemplateID, &paramsJSON, &n.DeviceToken, &n.Recipient,
			&n.ErrorMessage, &n.RetryCount, &n.DeduplicationKey,
			&n.CreatedAt, &n.SentAt, &n.DeliveredAt, &n.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan notification row: %w", err)
		}

		if len(dataJSON) > 0 {
			_ = json.Unmarshal(dataJSON, &n.Data)
		}
		if len(paramsJSON) > 0 {
			_ = json.Unmarshal(paramsJSON, &n.TemplateParams)
		}

		notifications = append(notifications, &n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notification rows: %w", err)
	}
	return notifications, nil
}

func marshalJSON(v interface{}) ([]byte, error) {
	if v == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(v)
}
