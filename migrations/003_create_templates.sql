-- Migration 003: Create notification_templates table

CREATE TABLE IF NOT EXISTS notification_templates (
    id         BIGSERIAL PRIMARY KEY,
    name       VARCHAR(255) NOT NULL,
    channel    notification_channel NOT NULL,
    subject    VARCHAR(500) NOT NULL DEFAULT '',
    body       TEXT NOT NULL,
    locale     VARCHAR(10) NOT NULL DEFAULT 'en',
    version    INTEGER NOT NULL DEFAULT 1,
    active     BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_templates_name_channel_locale ON notification_templates(name, channel, locale);
CREATE INDEX idx_templates_active ON notification_templates(active) WHERE active = true;
CREATE UNIQUE INDEX idx_templates_name_channel_locale_version
    ON notification_templates(name, channel, locale, version);
