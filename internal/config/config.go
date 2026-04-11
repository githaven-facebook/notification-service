package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the notification service.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Kafka    KafkaConfig    `yaml:"kafka"`
	DB       DBConfig       `yaml:"db"`
	Redis    RedisConfig    `yaml:"redis"`
	Channels ChannelsConfig `yaml:"channels"`
	Throttle ThrottleConfig `yaml:"throttle"`
	Log      LogConfig      `yaml:"log"`
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

// KafkaConfig holds Kafka consumer configuration.
type KafkaConfig struct {
	Brokers       []string `yaml:"brokers"`
	ConsumerGroup string   `yaml:"consumer_group"`
	Topics        []string `yaml:"topics"`
	MinBytes      int      `yaml:"min_bytes"`
	MaxBytes      int      `yaml:"max_bytes"`
	MaxWait       time.Duration `yaml:"max_wait"`
}

// DBConfig holds PostgreSQL configuration.
type DBConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	Name            string        `yaml:"name"`
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	SSLMode         string        `yaml:"ssl_mode"`
	MaxConns        int32         `yaml:"max_conns"`
	MinConns        int32         `yaml:"min_conns"`
	MaxConnLifetime time.Duration `yaml:"max_conn_lifetime"`
	MaxConnIdleTime time.Duration `yaml:"max_conn_idle_time"`
}

// DSN returns the PostgreSQL connection string.
func (d DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		d.Host, d.Port, d.Name, d.User, d.Password, d.SSLMode,
	)
}

// RedisConfig holds Redis configuration.
type RedisConfig struct {
	Addr         string        `yaml:"addr"`
	Password     string        `yaml:"password"`
	DB           int           `yaml:"db"`
	PoolSize     int           `yaml:"pool_size"`
	DialTimeout  time.Duration `yaml:"dial_timeout"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

// ChannelsConfig holds configuration for all notification channels.
type ChannelsConfig struct {
	FCM   FCMConfig   `yaml:"fcm"`
	APNS  APNSConfig  `yaml:"apns"`
	SES   SESConfig   `yaml:"ses"`
	SNS   SNSConfig   `yaml:"sns"`
	InApp InAppConfig `yaml:"in_app"`
}

// FCMConfig holds Firebase Cloud Messaging configuration.
type FCMConfig struct {
	ProjectID       string `yaml:"project_id"`
	CredentialsFile string `yaml:"credentials_file"`
	DefaultTTL      int    `yaml:"default_ttl"` // seconds
}

// APNSConfig holds Apple Push Notification Service configuration.
type APNSConfig struct {
	CertificateFile     string `yaml:"certificate_file"`
	CertificatePassword string `yaml:"certificate_password"`
	KeyFile             string `yaml:"key_file"`
	BundleID            string `yaml:"bundle_id"`
	Production          bool   `yaml:"production"`
	DefaultExpiration   int    `yaml:"default_expiration"` // seconds
}

// SESConfig holds AWS SES configuration.
type SESConfig struct {
	Region      string `yaml:"region"`
	FromAddress string `yaml:"from_address"`
	ReplyTo     string `yaml:"reply_to"`
}

// SNSConfig holds AWS SNS configuration.
type SNSConfig struct {
	Region   string `yaml:"region"`
	SenderID string `yaml:"sender_id"`
}

// InAppConfig holds in-app notification configuration.
type InAppConfig struct {
	RedisChannel string `yaml:"redis_channel"`
}

// ThrottleConfig holds rate limiting configuration.
type ThrottleConfig struct {
	MaxPerUserPerHour  int `yaml:"max_per_user_per_hour"`
	MaxPushPerHour     int `yaml:"max_push_per_hour"`
	MaxEmailPerHour    int `yaml:"max_email_per_hour"`
	MaxSMSPerHour      int `yaml:"max_sms_per_hour"`
	MaxInAppPerHour    int `yaml:"max_in_app_per_hour"`
	DeduplicationTTL   int `yaml:"deduplication_ttl"` // seconds
}

// LogConfig holds logging configuration.
type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"` // json or console
}

// Load loads configuration from environment variables with defaults.
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:            getEnvInt("SERVER_PORT", 8080),
			ReadTimeout:     getEnvDuration("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:    getEnvDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
			ShutdownTimeout: getEnvDuration("SERVER_SHUTDOWN_TIMEOUT", 30*time.Second),
		},
		Kafka: KafkaConfig{
			Brokers:       getEnvStringSlice("KAFKA_BROKERS", []string{"localhost:9092"}),
			ConsumerGroup: getEnvString("KAFKA_CONSUMER_GROUP", "notification-service"),
			Topics:        getEnvStringSlice("KAFKA_TOPICS", []string{"notification-events", "batch-notifications", "preference-updates"}),
			MinBytes:      getEnvInt("KAFKA_MIN_BYTES", 10e3),
			MaxBytes:      getEnvInt("KAFKA_MAX_BYTES", 10e6),
			MaxWait:       getEnvDuration("KAFKA_MAX_WAIT", 1*time.Second),
		},
		DB: DBConfig{
			Host:            getEnvString("DB_HOST", "localhost"),
			Port:            getEnvInt("DB_PORT", 5432),
			Name:            getEnvString("DB_NAME", "notifications"),
			User:            getEnvString("DB_USER", "postgres"),
			Password:        getEnvString("DB_PASSWORD", "postgres"),
			SSLMode:         getEnvString("DB_SSLMODE", "disable"),
			MaxConns:        int32(getEnvInt("DB_MAX_CONNS", 25)),
			MinConns:        int32(getEnvInt("DB_MIN_CONNS", 5)),
			MaxConnLifetime: getEnvDuration("DB_MAX_CONN_LIFETIME", 30*time.Minute),
			MaxConnIdleTime: getEnvDuration("DB_MAX_CONN_IDLE_TIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			Addr:         getEnvString("REDIS_ADDR", "localhost:6379"),
			Password:     getEnvString("REDIS_PASSWORD", ""),
			DB:           getEnvInt("REDIS_DB", 0),
			PoolSize:     getEnvInt("REDIS_POOL_SIZE", 10),
			DialTimeout:  getEnvDuration("REDIS_DIAL_TIMEOUT", 5*time.Second),
			ReadTimeout:  getEnvDuration("REDIS_READ_TIMEOUT", 3*time.Second),
			WriteTimeout: getEnvDuration("REDIS_WRITE_TIMEOUT", 3*time.Second),
		},
		Channels: ChannelsConfig{
			FCM: FCMConfig{
				ProjectID:       getEnvString("FCM_PROJECT_ID", ""),
				CredentialsFile: getEnvString("FCM_CREDENTIALS_FILE", ""),
				DefaultTTL:      getEnvInt("FCM_DEFAULT_TTL", 86400),
			},
			APNS: APNSConfig{
				CertificateFile:     getEnvString("APNS_CERT_FILE", ""),
				CertificatePassword: getEnvString("APNS_CERT_PASSWORD", ""),
				KeyFile:             getEnvString("APNS_KEY_FILE", ""),
				BundleID:            getEnvString("APNS_BUNDLE_ID", ""),
				Production:          getEnvBool("APNS_PRODUCTION", false),
				DefaultExpiration:   getEnvInt("APNS_DEFAULT_EXPIRATION", 86400),
			},
			SES: SESConfig{
				Region:      getEnvString("SES_REGION", "us-east-1"),
				FromAddress: getEnvString("SES_FROM_ADDRESS", "noreply@example.com"),
				ReplyTo:     getEnvString("SES_REPLY_TO", ""),
			},
			SNS: SNSConfig{
				Region:   getEnvString("SNS_REGION", "us-east-1"),
				SenderID: getEnvString("SNS_SENDER_ID", ""),
			},
			InApp: InAppConfig{
				RedisChannel: getEnvString("INAPP_REDIS_CHANNEL", "notifications"),
			},
		},
		Throttle: ThrottleConfig{
			MaxPerUserPerHour: getEnvInt("THROTTLE_MAX_PER_USER_PER_HOUR", 100),
			MaxPushPerHour:    getEnvInt("THROTTLE_MAX_PUSH_PER_HOUR", 50),
			MaxEmailPerHour:   getEnvInt("THROTTLE_MAX_EMAIL_PER_HOUR", 10),
			MaxSMSPerHour:     getEnvInt("THROTTLE_MAX_SMS_PER_HOUR", 5),
			MaxInAppPerHour:   getEnvInt("THROTTLE_MAX_INAPP_PER_HOUR", 200),
			DeduplicationTTL:  getEnvInt("THROTTLE_DEDUPLICATION_TTL", 300),
		},
		Log: LogConfig{
			Level:  getEnvString("LOG_LEVEL", "info"),
			Format: getEnvString("LOG_FORMAT", "json"),
		},
	}

	return cfg, nil
}

func getEnvString(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return defaultVal
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultVal
}

func getEnvStringSlice(key string, defaultVal []string) []string {
	if val := os.Getenv(key); val != "" {
		return strings.Split(val, ",")
	}
	return defaultVal
}
