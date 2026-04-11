package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metric collectors for the notification service.
type Metrics struct {
	NotificationsSentTotal     *prometheus.CounterVec
	NotificationSendDuration   *prometheus.HistogramVec
	DeliveryRetryTotal         *prometheus.CounterVec
	ThrottleBlockedTotal       *prometheus.CounterVec
	KafkaMessagesProcessedTotal *prometheus.CounterVec
	DeduplicationHitsTotal     *prometheus.CounterVec
	ActiveBatchQueueSize       *prometheus.GaugeVec
	TemplateRenderDuration     prometheus.Histogram
}

// New creates and registers all Prometheus metrics.
func New(reg prometheus.Registerer) *Metrics {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}

	factory := promauto.With(reg)

	return &Metrics{
		NotificationsSentTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "notification",
				Name:      "sent_total",
				Help:      "Total number of notifications sent, labeled by channel and status.",
			},
			[]string{"channel", "status"},
		),

		NotificationSendDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "notification",
				Name:      "send_duration_seconds",
				Help:      "Histogram of notification send duration in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"channel"},
		),

		DeliveryRetryTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "notification",
				Name:      "delivery_retry_total",
				Help:      "Total number of delivery retry attempts, labeled by channel.",
			},
			[]string{"channel"},
		),

		ThrottleBlockedTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "notification",
				Name:      "throttle_blocked_total",
				Help:      "Total number of notifications blocked by throttling, labeled by channel.",
			},
			[]string{"channel"},
		),

		KafkaMessagesProcessedTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "notification",
				Name:      "kafka_messages_processed_total",
				Help:      "Total number of Kafka messages processed, labeled by topic and status.",
			},
			[]string{"topic", "status"},
		),

		DeduplicationHitsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "notification",
				Name:      "deduplication_hits_total",
				Help:      "Total number of duplicate notifications suppressed, labeled by channel.",
			},
			[]string{"channel"},
		),

		ActiveBatchQueueSize: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "notification",
				Name:      "batch_queue_size",
				Help:      "Current number of notifications pending in batch queues, labeled by channel.",
			},
			[]string{"channel"},
		),

		TemplateRenderDuration: factory.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: "notification",
				Name:      "template_render_duration_seconds",
				Help:      "Histogram of template render duration in seconds.",
				Buckets:   []float64{.0001, .0005, .001, .005, .01, .025, .05},
			},
		),
	}
}

// RecordSent increments the notifications sent counter.
func (m *Metrics) RecordSent(channel, status string) {
	m.NotificationsSentTotal.WithLabelValues(channel, status).Inc()
}

// RecordSendDuration observes the send duration for a channel.
func (m *Metrics) RecordSendDuration(channel string, durationSeconds float64) {
	m.NotificationSendDuration.WithLabelValues(channel).Observe(durationSeconds)
}

// RecordRetry increments the retry counter for a channel.
func (m *Metrics) RecordRetry(channel string) {
	m.DeliveryRetryTotal.WithLabelValues(channel).Inc()
}

// RecordThrottleBlocked increments the throttle blocked counter.
func (m *Metrics) RecordThrottleBlocked(channel string) {
	m.ThrottleBlockedTotal.WithLabelValues(channel).Inc()
}

// RecordKafkaMessage increments the Kafka messages processed counter.
func (m *Metrics) RecordKafkaMessage(topic, status string) {
	m.KafkaMessagesProcessedTotal.WithLabelValues(topic, status).Inc()
}

// RecordDeduplicationHit increments the deduplication hits counter.
func (m *Metrics) RecordDeduplicationHit(channel string) {
	m.DeduplicationHitsTotal.WithLabelValues(channel).Inc()
}
