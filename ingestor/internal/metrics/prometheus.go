package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Counters
	MessagesReceived = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "echoarb_messages_received_total",
		Help: "Total ticks received from exchanges",
	}, []string{"source"})

	ErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "echoarb_errors_total",
		Help: "Total errors by source and type",
	}, []string{"source", "error_type"})

	DuplicatesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "echoarb_duplicates_total",
		Help: "Total duplicate messages received",
	}, []string{"source"})

	// Gauges
	ConnectionStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "echoarb_connection_status",
		Help: "1 if connected, 0 if disconnected",
	}, []string{"source"})

	HealthStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "echoarb_health_status",
		Help: "Health status of services (1=healthy, 0=unhealthy)",
	}, []string{"service"})

	CurrentPrice = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "echoarb_current_price",
		Help: "Current price for a contract",
	}, []string{"source", "ticker"})

	// Histograms (Latency)
	IngestLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "echoarb_ingest_latency_seconds",
		Help:    "Time from exchange timestamp to ingestor receipt",
		Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1.0},
	}, []string{"source"})

	ProcessingTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "echoarb_processing_time_seconds",
		Help:    "Time to process a message",
		Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1},
	}, []string{"source"})
)

type Registry struct{}

func NewRegistry() *Registry {
	return &Registry{}
}

// RecordError records an error event
func (r *Registry) RecordError(source, errorType string) {
	ErrorsTotal.WithLabelValues(source, errorType).Inc()
}

// RecordConnection records connection status
func (r *Registry) RecordConnection(source string, connected bool) {
	val := 0.0
	if connected {
		val = 1.0
	}
	ConnectionStatus.WithLabelValues(source).Set(val)
}

// RecordMessage records a successfully received message
func (r *Registry) RecordMessage(source string, timestamp int64, success bool) {
	if success {
		MessagesReceived.WithLabelValues(source).Inc()

		// Calculate latency
		now := time.Now().UnixMilli()
		latencyMs := now - timestamp
		latencySec := float64(latencyMs) / 1000.0
		IngestLatency.WithLabelValues(source).Observe(latencySec)
	}
}

// RecordDuplicate records a duplicate message
func (r *Registry) RecordDuplicate(source string) {
	DuplicatesTotal.WithLabelValues(source).Inc()
}

// RecordProcessingTime records time spent processing a message
func (r *Registry) RecordProcessingTime(source string, duration time.Duration) {
	ProcessingTime.WithLabelValues(source).Observe(duration.Seconds())
}

// RecordPrice records the current price for a contract
func (r *Registry) RecordPrice(source, ticker string, price float64) {
	CurrentPrice.WithLabelValues(source, ticker).Set(price)
}

// SetConnectionActive sets connection as active/inactive
func (r *Registry) SetConnectionActive(source string, active bool) {
	val := 0.0
	if active {
		val = 1.0
	}
	ConnectionStatus.WithLabelValues(source).Set(val)
}

// SetHealthStatus sets health status for a service
func (r *Registry) SetHealthStatus(service string, healthy bool) {
	val := 0.0
	if healthy {
		val = 1.0
	}
	HealthStatus.WithLabelValues(service).Set(val)
}