package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/nicedavid98/notification-service/internal/config"
	"github.com/nicedavid98/notification-service/internal/consumer"
	"github.com/nicedavid98/notification-service/internal/delivery"
	"github.com/nicedavid98/notification-service/internal/handler"
	"github.com/nicedavid98/notification-service/internal/metrics"
	"github.com/nicedavid98/notification-service/internal/model"
	"github.com/nicedavid98/notification-service/internal/provider"
	"github.com/nicedavid98/notification-service/internal/provider/email"
	"github.com/nicedavid98/notification-service/internal/provider/inapp"
	"github.com/nicedavid98/notification-service/internal/provider/push"
	"github.com/nicedavid98/notification-service/internal/provider/sms"
	"github.com/nicedavid98/notification-service/internal/repository"
	"github.com/nicedavid98/notification-service/internal/service"
	tmpl "github.com/nicedavid98/notification-service/internal/template"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Initialize logger
	logger, err := buildLogger(cfg.Log)
	if err != nil {
		return fmt.Errorf("build logger: %w", err)
	}
	defer logger.Sync() //nolint:errcheck // Sync errors on stderr/stdout are non-actionable at shutdown

	logger.Info("Starting notification service",
		zap.Int("port", cfg.Server.Port),
		zap.Strings("kafka_brokers", cfg.Kafka.Brokers),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize metrics
	m := metrics.New(nil)

	// Initialize PostgreSQL pool
	dbPool, err := repository.NewPostgresPool(ctx, &cfg.DB)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer dbPool.Close()
	logger.Info("PostgreSQL connected")

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     cfg.Redis.PoolSize,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
	})
	defer redisClient.Close()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("connect to redis: %w", err)
	}
	logger.Info("Redis connected")

	// Initialize repositories
	notifRepo := repository.NewNotificationRepository(dbPool)
	prefRepo := repository.NewPreferenceRepository(dbPool)
	templateRepo := repository.NewTemplateRepository(dbPool)

	// Initialize providers
	providers := make(map[model.NotificationChannel]provider.Provider)

	fcmProvider := push.NewFCMProvider(cfg.Channels.FCM, logger)
	providers[model.ChannelFCM] = fcmProvider

	apnsProvider, err := push.NewAPNSProvider(&cfg.Channels.APNS, logger)
	if err != nil {
		logger.Warn("Failed to initialize APNS provider, skipping", zap.Error(err))
	} else {
		providers[model.ChannelAPNS] = apnsProvider
	}

	sesProvider, err := email.NewSESProvider(ctx, cfg.Channels.SES, logger)
	if err != nil {
		logger.Warn("Failed to initialize SES provider, skipping", zap.Error(err))
	} else {
		providers[model.ChannelSES] = sesProvider
	}

	snsProvider, err := sms.NewSNSProvider(ctx, cfg.Channels.SNS, logger)
	if err != nil {
		logger.Warn("Failed to initialize SNS provider, skipping", zap.Error(err))
	} else {
		providers[model.ChannelSNS] = snsProvider
	}

	inAppProvider := inapp.NewInAppProvider(notifRepo, redisClient, cfg.Channels.InApp, logger)
	providers[model.ChannelInApp] = inAppProvider

	logger.Info("Providers initialized", zap.Int("count", len(providers)))

	// Initialize services
	templateEngine := tmpl.NewEngine(templateRepo, logger)
	renderer := tmpl.NewRenderer(templateEngine)
	prefService := service.NewPreferenceService(prefRepo, logger)
	throttleService := service.NewThrottleService(redisClient, cfg.Throttle, logger)

	notifService := service.NewNotificationService(service.NotificationServiceConfig{
		NotifRepo:   notifRepo,
		PrefService: prefService,
		Throttle:    throttleService,
		Renderer:    renderer,
		Providers:   providers,
		RedisClient: redisClient,
		DedupTTL:    time.Duration(cfg.Throttle.DeduplicationTTL) * time.Second,
		Logger:      logger,
	})

	// Initialize dispatcher and retry manager
	dispatcher := delivery.NewDispatcher(providers, notifRepo, logger)
	retryMgr := delivery.NewRetryManager(dispatcher, notifRepo, delivery.RetryConfig{
		MaxRetries: 5,
		BaseDelay:  time.Second,
		MaxDelay:   16 * time.Second,
	}, logger)

	// Initialize batch processor (1-hour digest interval)
	batchProcessor := delivery.NewBatchProcessor(dispatcher, notifRepo, time.Hour, logger)
	batchProcessor.Start(ctx)
	defer batchProcessor.Stop()

	// Initialize HTTP router
	r := chi.NewRouter()
	r.Use(handler.RequestID)
	r.Use(handler.Logger(logger))
	r.Use(handler.Recoverer(logger))
	r.Use(chimiddleware.Compress(5))

	// Health endpoints
	healthHandler := handler.NewHealthHandler(dbPool, redisClient)
	r.Get("/health", healthHandler.Health)
	r.Get("/ready", healthHandler.Ready)

	// Prometheus metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	// API routes
	notifHandler := handler.NewNotificationHandler(notifService, logger)
	prefHandler := handler.NewPreferenceHandler(prefService, logger)
	templateHandler := handler.NewTemplateHandler(templateRepo, templateEngine, logger)

	r.Route("/api/v1", func(r chi.Router) {
		// Notification endpoints
		r.Route("/notifications", func(r chi.Router) {
			r.Post("/send", notifHandler.Send)
			r.Post("/batch", notifHandler.SendBatch)
			r.Get("/{id}/status", notifHandler.GetStatus)
			r.Get("/user/{userId}", notifHandler.GetUserNotifications)
		})

		// Preference endpoints
		r.Route("/preferences", func(r chi.Router) {
			r.Get("/{userId}", prefHandler.GetPreferences)
			r.Put("/{userId}/{channel}", prefHandler.UpdatePreference)
		})

		// Admin template endpoints
		r.Route("/admin/templates", func(r chi.Router) {
			r.Get("/", templateHandler.List)
			r.Post("/", templateHandler.Create)
			r.Get("/{id}", templateHandler.Get)
			r.Put("/{id}", templateHandler.Update)
			r.Delete("/{id}", templateHandler.Delete)
		})
	})

	// HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start Kafka consumer
	kafkaConsumer := consumer.NewKafkaConsumer(&cfg.Kafka, notifService, prefService, logger)
	go func() {
		logger.Info("Starting Kafka consumer")
		if err := kafkaConsumer.Start(ctx); err != nil {
			logger.Error("Kafka consumer stopped", zap.Error(err))
		}
	}()

	// Start retry worker (every 30 seconds)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := retryMgr.RetryFailed(ctx); err != nil {
					logger.Error("Retry worker failed", zap.Error(err))
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Start HTTP server
	serverErrCh := make(chan error, 1)
	go func() {
		logger.Info("HTTP server listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Info("Received shutdown signal", zap.String("signal", sig.String()))
	case err := <-serverErrCh:
		logger.Error("HTTP server error", zap.Error(err))
		return err
	}

	// Graceful shutdown
	logger.Info("Shutting down gracefully...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", zap.Error(err))
		return err
	}

	// Log metrics summary (in production, Prometheus handles this)
	_ = m

	logger.Info("Notification service stopped")
	return nil
}

// buildLogger constructs a zap.Logger from the log config.
func buildLogger(cfg config.LogConfig) (*zap.Logger, error) {
	level := zap.InfoLevel
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		level = zap.InfoLevel
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "ts"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	var encoding string
	if cfg.Format == "console" {
		encoding = "console"
	} else {
		encoding = "json"
	}

	zapCfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(level),
		Development:      false,
		Encoding:         encoding,
		EncoderConfig:    encoderCfg,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	return zapCfg.Build()
}
