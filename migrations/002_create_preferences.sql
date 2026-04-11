-- Migration 002: Create user_preferences table

CREATE TYPE digest_frequency AS ENUM ('hourly', 'daily', 'weekly');

CREATE TABLE IF NOT EXISTS user_preferences (
    id                BIGSERIAL PRIMARY KEY,
    user_id           VARCHAR(255) NOT NULL,
    channel           notification_channel NOT NULL,
    enabled           BOOLEAN NOT NULL DEFAULT true,
    quiet_hours_start VARCHAR(5),   -- e.g., '22:00'
    quiet_hours_end   VARCHAR(5),   -- e.g., '08:00'
    digest_mode       BOOLEAN NOT NULL DEFAULT false,
    frequency         digest_frequency NOT NULL DEFAULT 'daily',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, channel)
);

CREATE INDEX idx_user_preferences_user_id ON user_preferences(user_id);
CREATE INDEX idx_user_preferences_user_channel ON user_preferences(user_id, channel);
