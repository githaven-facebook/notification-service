-- Migration 001: Create notifications table

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TYPE notification_type AS ENUM ('push', 'email', 'sms', 'in_app');
CREATE TYPE notification_channel AS ENUM ('fcm', 'apns', 'ses', 'sns', 'in_app');
CREATE TYPE notification_priority AS ENUM ('high', 'normal', 'low');
CREATE TYPE notification_status AS ENUM ('pending', 'sent', 'delivered', 'failed');

CREATE TABLE IF NOT EXISTS notifications (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id           VARCHAR(255) NOT NULL,
    type              notification_type NOT NULL,
    channel           notification_channel NOT NULL,
    title             VARCHAR(500) NOT NULL,
    body              TEXT NOT NULL,
    data              JSONB NOT NULL DEFAULT '{}',
    priority          notification_priority NOT NULL DEFAULT 'normal',
    status            notification_status NOT NULL DEFAULT 'pending',
    template_id       VARCHAR(255),
    template_params   JSONB NOT NULL DEFAULT '{}',
    device_token      VARCHAR(500),
    recipient         VARCHAR(500),
    error_message     TEXT,
    retry_count       INTEGER NOT NULL DEFAULT 0,
    deduplication_key VARCHAR(255),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at           TIMESTAMPTZ,
    delivered_at      TIMESTAMPTZ,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_user_id ON notifications(user_id);
CREATE INDEX idx_notifications_status ON notifications(status);
CREATE INDEX idx_notifications_user_status ON notifications(user_id, status);
CREATE INDEX idx_notifications_user_type ON notifications(user_id, type);
CREATE INDEX idx_notifications_dedup_key ON notifications(deduplication_key) WHERE deduplication_key IS NOT NULL;
CREATE INDEX idx_notifications_created_at ON notifications(created_at DESC);
CREATE INDEX idx_notifications_retry ON notifications(status, retry_count) WHERE status = 'failed';
