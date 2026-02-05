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

	"github.com/dragonuber/echoarb/ingestor/internal/config"
	"github.com/dragonuber/echoarb/ingestor/internal/connectors"
	"github.com/dragonuber/echoarb/ingestor/internal/metrics"
	"github.com/dragonuber/echoarb/ingestor/internal/redis"
	"github.com/prometheus/client_golang/prometheus/promhttp"

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
	sugar.Info("Starting EchoArb Ingestor")

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

	// Create main context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize connectors
	polyConn := connectors.NewPolymarketConnector(
		cfg,
		redisClient,
		metricsRegistry,
		sugar,
	)

	// Start connectors
	sugar.Info("Starting connectors...")

	if cfg.HasKalshiCredentials() {
		kalshiConn, err := connectors.NewKalshiConnector(
			cfg,
			redisClient,
			metricsRegistry,
			sugar,
		)
		if err != nil {
			sugar.Fatalf("Failed to create Kalshi connector: %v", err)
		}
		go kalshiConn.Start(ctx)
	} else {
		sugar.Warn("Kalshi credentials not configured; skipping Kalshi connector")
	}

	go polyConn.Start(ctx)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	sugar.Info("Received shutdown signal")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	sugar.Info("Shutting down gracefully...")
	cancel() // Cancel main context to stop all connectors

	// Wait for shutdown or timeout
	<-shutdownCtx.Done()
	sugar.Info("Shutdown complete")
}

func startMetricsServer(port int, logger *zap.SugaredLogger) {
	mux := http.NewServeMux()

	// Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Health check endpoint
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
