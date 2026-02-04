// internal/connectors/kalshi.go
package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/dragonuber/echoarb/ingestor/internal/auth"
	"github.com/dragonuber/echoarb/ingestor/internal/config"
	"github.com/dragonuber/echoarb/ingestor/internal/metrics"
	"github.com/dragonuber/echoarb/ingestor/internal/models"
	"github.com/dragonuber/echoarb/ingestor/internal/redis"
	"github.com/dragonuber/echoarb/ingestor/internal/retry"
	"go.uber.org/zap"
)

const (
	kalshiSource = "KALSHI"
	pingInterval = 30 * time.Second
	pongTimeout  = 60 * time.Second  // Must be > pingInterval to handle idle markets
)

// KalshiConnector handles Kalshi WebSocket connection
type KalshiConnector struct {
	config       *config.Config
	auth         *auth.KalshiAuth
	redis        *redis.Client
	metrics      *metrics.Registry
	logger       *zap.SugaredLogger
	
	// State management
	lastPrices   sync.Map // map[string]float64 for deduplication
	isConnected  bool
	mu           sync.RWMutex
}

// NewKalshiConnector creates a new Kalshi connector
func NewKalshiConnector(
	cfg *config.Config,
	redisClient *redis.Client,
	metricsReg *metrics.Registry,
	logger *zap.SugaredLogger,
) (*KalshiConnector, error) {
	// Initialize Kalshi authentication
	auth, err := auth.NewKalshiAuth(cfg.KalshiAPIKey, cfg.KalshiPrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Kalshi auth: %w", err)
	}

	return &KalshiConnector{
		config:  cfg,
		auth:    auth,
		redis:   redisClient,
		metrics: metricsReg,
		logger:  logger.Named("kalshi"),
	}, nil
}

// Start begins the connection loop with retry
func (k *KalshiConnector) Start(ctx context.Context) {
	k.logger.Info("Starting Kalshi connector")

	retryCfg := retry.Config{
		InitialInterval: k.config.Reconnect.InitialInterval,
		MaxInterval:     k.config.Reconnect.MaxInterval,
		MaxRetries:      k.config.Reconnect.MaxRetries,
		Multiplier:      2.0,
		Jitter:          true,
	}

	retry.RetryForever(ctx, retryCfg, k.logger, "Kalshi", func() error {
		return k.connect(ctx)
	})

	k.logger.Info("Kalshi connector stopped")
}

// connect establishes WebSocket connection
func (k *KalshiConnector) connect(ctx context.Context) error {
	k.logger.Info("Connecting to Kalshi WebSocket...")

	// Get authenticated headers
	headers, err := k.auth.GetWebSocketHeaders()
	if err != nil {
		k.metrics.RecordError(kalshiSource, "auth_error")
		return fmt.Errorf("failed to get auth headers: %w", err)
	}

	// Create dialer with timeout
	dialer := websocket.Dialer{
		HandshakeTimeout: 30 * time.Second,
	}

	// Connect
	conn, _, err := dialer.Dial(k.config.KalshiWSURL, headers)
	if err != nil {
		k.metrics.RecordConnection(kalshiSource, false)
		k.metrics.RecordError(kalshiSource, "connection_error")
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	k.metrics.RecordConnection(kalshiSource, true)
	k.setConnected(true)
	k.logger.Info("Connected to Kalshi")

	// Set up ping/pong handlers
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongTimeout))
		return nil
	})

	// Subscribe to markets
	if err := k.subscribe(conn); err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	// Start ping routine
	pingCtx, pingCancel := context.WithCancel(ctx)
	defer pingCancel()
	go k.pingLoop(pingCtx, conn)

	// Read messages
	return k.readLoop(ctx, conn)
}

// subscribe subscribes to market updates
func (k *KalshiConnector) subscribe(conn *websocket.Conn) error {
	for _, pair := range k.config.Pairs {
		if pair.Kalshi == nil {
			continue
		}

		subscribeMsg := map[string]interface{}{
			"id":  1,
			"cmd": "subscribe",
			"params": map[string]interface{}{
				"channels":      []string{"orderbook_delta"},
				"market_ticker": pair.Kalshi.Ticker,
			},
		}

		if err := conn.WriteJSON(subscribeMsg); err != nil {
			return fmt.Errorf("failed to subscribe to %s: %w", pair.Kalshi.Ticker, err)
		}

		k.logger.Infof("Subscribed to Kalshi market: %s", pair.Kalshi.Ticker)
	}

	return nil
}

// pingLoop sends periodic pings
func (k *KalshiConnector) pingLoop(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := conn.WriteControl(
				websocket.PingMessage,
				[]byte{},
				time.Now().Add(10*time.Second),
			); err != nil {
				k.logger.Warnf("Ping failed: %v", err)
				return
			}
		}
	}
}

// readLoop reads messages from WebSocket
func (k *KalshiConnector) readLoop(ctx context.Context, conn *websocket.Conn) error {
	defer k.setConnected(false)

	// Set initial read deadline
	conn.SetReadDeadline(time.Now().Add(pongTimeout))

	for {
		// Check context
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Read message
		_, message, err := conn.ReadMessage()
		if err != nil {
			k.metrics.RecordError(kalshiSource, "read_error")
			return fmt.Errorf("read error: %w", err)
		}

		// Process message
		if err := k.processMessage(message); err != nil {
			k.logger.Warnf("Failed to process message: %v", err)
			k.metrics.RecordError(kalshiSource, "process_error")
			// Continue reading despite processing errors
		}
	}
}

// processMessage processes a single message
func (k *KalshiConnector) processMessage(data []byte) error {
	startTime := time.Now()

	// Parse JSON
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("json unmarshal error: %w", err)
	}

	// Check message type
	msgType, _ := msg["type"].(string)
	if msgType != "orderbook_delta" {
		return nil // Ignore non-orderbook messages
	}

	// Extract fields
	ticker, _ := msg["ticker"].(string)
	if ticker == "" {
		return fmt.Errorf("missing ticker")
	}

	yesBid, _ := msg["yes_bid"].(float64)
	yesAsk, _ := msg["yes_ask"].(float64)
	sourceTS, _ := msg["ts"].(float64)

	// Calculate mid-price (convert from cents to probability)
	price := (yesBid + yesAsk) / 200.0

	// Check for duplicate
	cacheKey := ticker
	if lastPrice, ok := k.lastPrices.Load(cacheKey); ok {
		if lastPrice.(float64) == price {
			k.metrics.RecordDuplicate(kalshiSource)
			return nil
		}
	}
	k.lastPrices.Store(cacheKey, price)

	// Create tick
	tick := &models.Tick{
		Source:          kalshiSource,
		ContractID:      ticker,
		Price:           price,
		TimestampSource: int64(sourceTS),
		TimestampIngest: time.Now().UnixMilli(),
	}

	// Publish to Redis
	if err := k.redis.PublishTick(tick); err != nil {
		k.metrics.RecordError(kalshiSource, "redis_error")
		return fmt.Errorf("failed to publish: %w", err)
	}

	// Record metrics
	k.metrics.RecordMessage(kalshiSource, tick.TimestampSource, true)
	k.metrics.RecordProcessingTime(kalshiSource, time.Since(startTime))
	k.metrics.RecordPrice(kalshiSource, ticker, price)

	return nil
}

// Helper methods

func (k *KalshiConnector) setConnected(connected bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.isConnected = connected
	k.metrics.SetConnectionActive(kalshiSource, connected)
	k.metrics.SetHealthStatus("kalshi", connected)
}

func (k *KalshiConnector) IsConnected() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.isConnected
}