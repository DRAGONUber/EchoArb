// cmd/ingestor/main.go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/dragonuber/echoarb/ingestor/internal/config"
	"github.com/dragonuber/echoarb/ingestor/internal/connectors"
	"github.com/dragonuber/echoarb/ingestor/internal/metrics"
	"github.com/dragonuber/echoarb/ingestor/internal/models"
	"github.com/dragonuber/echoarb/ingestor/internal/redis"
	
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		panic(fmt.Sprintf("failed to initialize logger: %v", err))
	}
	defer logger.Sync()

	sugar := logger.Sugar()
	sugar.Info("Starting EchoArb Ingestor (Firehose Mode)")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		sugar.Fatalf("Failed to load config: %v", err)
	}

	// Initialize metrics
	metricsRegistry := metrics.NewRegistry()
	
	// Start metrics server
	go startMetricsServer(cfg.MetricsPort, sugar)

	// Initialize Redis client
	redisClient, err := redis.NewClient(cfg.Redis, sugar)
	if err != nil {
		sugar.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	// Create tick channel (Buffer 1000 to handle bursts)
	msgChan := make(chan models.Tick, 1000)

	// Initialize connectors
	// Note: We pass sugar (Logger) and msgChan. No Redis client needed here.
	kalshiConn := connectors.NewKalshiConnector(cfg, sugar, msgChan)
	polyConn := connectors.NewPolymarketConnector(cfg, sugar, msgChan)

	// Start connectors
	sugar.Info("Starting connectors...")

	// Kalshi
	go func() {
		if err := kalshiConn.Start(); err != nil {
			sugar.Errorf("Kalshi connector failed: %v", err)
		}
	}()

	// Polymarket
	go func() {
		if err := polyConn.Start(); err != nil {
			sugar.Errorf("Polymarket connector failed: %v", err)
		}
	}()

	// Start the Tick Processor (Reads channel -> Writes to Redis)
	go processTicks(msgChan, redisClient, metricsRegistry, sugar)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	
	<-sigChan
	sugar.Info("Received shutdown signal")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	sugar.Info("Shutting down gracefully...")
	<-shutdownCtx.Done()
	sugar.Info("Shutdown complete")
}

// processTicks reads ticks from the channel and publishes them to Redis
func processTicks(ch <-chan models.Tick, rdb *redis.Client, m *metrics.Registry, logger *zap.SugaredLogger) {
	for tick := range ch {
		// Record metrics
		m.RecordMessage(tick.Source, tick.TimestampSource, true)
		m.RecordPrice(tick.Source, tick.ContractID, tick.Price)

		// Publish to Redis (Stream + PubSub)
		if err := rdb.PublishTick(&tick); err != nil {
			logger.Errorf("Failed to publish tick from %s: %v", tick.Source, err)
			m.RecordError(tick.Source, "redis_publish_error")
		}
	}
}

func startMetricsServer(port int, logger *zap.SugaredLogger) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	addr := fmt.Sprintf(":%d", port)
	logger.Infof("Starting metrics server on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Fatalf("Metrics server failed: %v", err)
	}
}