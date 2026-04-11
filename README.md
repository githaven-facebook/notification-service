# Notification Service

A production-grade unified notification service supporting Push (FCM/APNS), Email (SES), SMS (SNS), and In-App notifications. Built to handle high-throughput notification delivery with reliability guarantees.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Notification Service                      │
│                                                             │
│  ┌──────────────┐   ┌──────────────┐   ┌────────────────┐  │
│  │  HTTP API    │   │ Kafka Consumer│   │ Batch Processor│  │
│  │  (chi router)│   │(kafka-go)    │   │ (digest mode)  │  │
│  └──────┬───────┘   └──────┬───────┘   └───────┬────────┘  │
│         └─────────────────┼───────────────────┘           │
│                            ▼                               │
│                 ┌─────────────────────┐                   │
│                 │ Notification Service│                   │
│                 │ - Preference Check  │                   │
│                 │ - Throttle Check    │                   │
│                 │ - Deduplication     │                   │
│                 │ - Template Render   │                   │
│                 └──────────┬──────────┘                   │
│                            ▼                               │
│              ┌─────────────────────────┐                  │
│              │      Dispatcher         │                  │
│              │  - Retry with backoff   │                  │
│              └──┬────┬────┬────┬───────┘                  │
│                 │    │    │    │                           │
│                FCM  APNS  SES  SNS  In-App                │
└─────────────────────────────────────────────────────────────┘
```

## Features

- **Multi-channel delivery**: Push (FCM/APNS), Email (AWS SES), SMS (AWS SNS), In-App
- **Kafka integration**: Async event consumption from upstream services
- **Template engine**: Go text/template with locale fallback (e.g., `ko-KR → ko → en`)
- **User preferences**: Per-channel opt-in/out, quiet hours, digest mode
- **Throttling**: Redis-backed sliding window rate limiting per user/channel
- **Deduplication**: Hash-based dedup within configurable time windows
- **Retry**: Exponential backoff with jitter, dead letter queue for permanent failures
- **Batch/Digest**: Aggregate notifications and send at configured intervals
- **Observability**: Prometheus metrics, structured Zap logging

## Channel Configuration

### Push Notifications (FCM)
```yaml
channels:
  fcm:
    project_id: "your-firebase-project"
    credentials_file: "/path/to/service-account.json"
```

### Push Notifications (APNS)
```yaml
channels:
  apns:
    certificate_file: "/path/to/cert.p12"
    certificate_password: "password"
    bundle_id: "com.example.app"
    production: true
```

### Email (AWS SES)
```yaml
channels:
  ses:
    region: "us-east-1"
    from_address: "noreply@example.com"
    reply_to: "support@example.com"
```

### SMS (AWS SNS)
```yaml
channels:
  sns:
    region: "us-east-1"
    sender_id: "MyApp"
```

## Template System

Templates are stored in PostgreSQL and support Go's `text/template` syntax.

**Locale fallback**: `ko-KR → ko → en`

**Example template**:
```
Subject: Welcome to {{.AppName}}, {{.UserName}}!
Body: Hi {{.UserName}}, your account is ready.
```

**Seeded templates**: `welcome`, `password_reset`, `friend_request`, `comment`, `ad_report`

## Preference Management

Users can configure per-channel preferences:
- Enable/disable specific channels
- Quiet hours (no notifications during specified time range)
- Digest mode (batch notifications, configurable frequency)

## API Reference

### Send Notification
```
POST /api/v1/notifications/send
Content-Type: application/json

{
  "user_id": "user-123",
  "type": "push",
  "channel": "fcm",
  "title": "New message",
  "body": "You have a new message",
  "priority": "high",
  "template_id": "friend_request",
  "template_params": {"sender": "Alice"}
}
```

### Batch Send
```
POST /api/v1/notifications/batch
Content-Type: application/json

{
  "notifications": [...]
}
```

### Get Status
```
GET /api/v1/notifications/{id}/status
```

### Get User Notifications
```
GET /api/v1/notifications/user/{userId}?page=1&limit=20
```

### Get User Preferences
```
GET /api/v1/preferences/{userId}
```

### Update Preference
```
PUT /api/v1/preferences/{userId}/{channel}
Content-Type: application/json

{
  "enabled": true,
  "quiet_hours_start": "22:00",
  "quiet_hours_end": "08:00",
  "digest_mode": false
}
```

### Health Check
```
GET /health
GET /ready
```

## Deployment

### Docker Compose (local development)
```bash
docker-compose up -d
```

### Kubernetes
See `k8s/` directory for manifests.

### Environment Variables
| Variable | Description | Default |
|----------|-------------|---------|
| `SERVER_PORT` | HTTP server port | `8080` |
| `DB_URL` | PostgreSQL connection string | - |
| `REDIS_ADDR` | Redis address | `localhost:6379` |
| `KAFKA_BROKERS` | Comma-separated Kafka brokers | `localhost:9092` |

## Development

```bash
# Install dependencies
go mod download

# Run tests
make test

# Run linter
make lint

# Build
make build

# Run locally
make run
```
