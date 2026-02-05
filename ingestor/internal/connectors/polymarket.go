// internal/connectors/polymarket.go
package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/dragonuber/echoarb/ingestor/internal/config"
	"github.com/dragonuber/echoarb/ingestor/internal/metrics"
	"github.com/dragonuber/echoarb/ingestor/internal/models"
	"github.com/dragonuber/echoarb/ingestor/internal/redis"
	"github.com/dragonuber/echoarb/ingestor/internal/retry"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	polySource         = "POLYMARKET"
	polyPingInterval   = 30 * time.Second
	polyPongTimeout    = 60 * time.Second // Must be > pingInterval to handle idle markets
	polyMarketsURL     = "https://gamma-api.polymarket.com/markets"
	polyMaxFetchLimit  = 500
	polySubscribeDelay = 5 * time.Millisecond
)

// PolymarketConnector handles Polymarket WebSocket connection
type PolymarketConnector struct {
	config      *config.Config
	redis       *redis.Client
	metrics     *metrics.Registry
	logger      *zap.SugaredLogger
	lastPrices  sync.Map
	isConnected bool
	mu          sync.RWMutex
}

// NewPolymarketConnector creates a new Polymarket connector
func NewPolymarketConnector(
	cfg *config.Config,
	redisClient *redis.Client,
	metricsReg *metrics.Registry,
	logger *zap.SugaredLogger,
) *PolymarketConnector {
	return &PolymarketConnector{
		config:  cfg,
		redis:   redisClient,
		metrics: metricsReg,
		logger:  logger.Named("polymarket"),
	}
}

// Start begins the connection loop with retry
func (p *PolymarketConnector) Start(ctx context.Context) {
	p.logger.Info("Starting Polymarket connector")

	retryCfg := retry.Config{
		InitialInterval: p.config.Reconnect.InitialInterval,
		MaxInterval:     p.config.Reconnect.MaxInterval,
		MaxRetries:      p.config.Reconnect.MaxRetries,
		Multiplier:      2.0,
		Jitter:          true,
	}

	retry.RetryForever(ctx, retryCfg, p.logger, "Polymarket", func() error {
		return p.connect(ctx)
	})

	p.logger.Info("Polymarket connector stopped")
}

// connect establishes WebSocket connection
func (p *PolymarketConnector) connect(ctx context.Context) error {
	p.logger.Info("Connecting to Polymarket WebSocket...")

	dialer := websocket.Dialer{
		HandshakeTimeout: 30 * time.Second,
	}

	conn, _, err := dialer.Dial(p.config.PolyWSURL, nil)
	if err != nil {
		p.metrics.RecordConnection(polySource, false)
		p.metrics.RecordError(polySource, "connection_error")
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	p.metrics.RecordConnection(polySource, true)
	p.setConnected(true)
	p.logger.Info("Connected to Polymarket")

	// Set up ping/pong handlers
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(polyPongTimeout))
		return nil
	})

	// Subscribe to markets
	if err := p.subscribe(conn); err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	// Start ping routine
	pingCtx, pingCancel := context.WithCancel(ctx)
	defer pingCancel()
	go p.pingLoop(pingCtx, conn)

	// Read messages
	return p.readLoop(ctx, conn)
}

// subscribe subscribes to market updates
func (p *PolymarketConnector) subscribe(conn *websocket.Conn) error {
	tokenIDs, err := p.fetchActiveMarkets()
	if err != nil {
		return fmt.Errorf("failed to fetch Polymarket markets via Gamma API: %w", err)
	}
	if len(tokenIDs) == 0 {
		return fmt.Errorf("no Polymarket markets available for subscription")
	}

	p.logger.Infof("Subscribing to %d Polymarket markets", len(tokenIDs))

	for _, tokenID := range tokenIDs {
		subscribeMsg := map[string]interface{}{
			"type":   "subscribe",
			"market": tokenID,
		}

		if err := conn.WriteJSON(subscribeMsg); err != nil {
			return fmt.Errorf("failed to subscribe to %s: %w", tokenID, err)
		}

		p.logger.Infof("Subscribed to Polymarket market: %s", tokenID)
		time.Sleep(polySubscribeDelay)
	}

	return nil
}

type polymarketMarket struct {
	TokenID      string   `json:"tokenId"`
	TokenIDSnake string   `json:"token_id"`
	ClobTokenIDs []string `json:"clobTokenIds"`
	Closed       bool     `json:"closed"`
}

func (p *PolymarketConnector) fetchActiveMarkets() ([]string, error) {
	p.logger.Info("Fetching active Polymarket markets via Gamma API")
	client := &http.Client{Timeout: 15 * time.Second}

	tokenIDs := []string{}
	offset := 0

	for {
		query := url.Values{}
		query.Set("closed", "false")
		query.Set("limit", fmt.Sprintf("%d", polyMaxFetchLimit))
		query.Set("offset", fmt.Sprintf("%d", offset))
		requestURL := fmt.Sprintf("%s?%s", polyMarketsURL, query.Encode())

		resp, err := client.Get(requestURL)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("polymarket markets request failed: %s", resp.Status)
		}

		var markets []polymarketMarket
		if err := json.Unmarshal(body, &markets); err != nil {
			return nil, err
		}

		if len(markets) == 0 {
			break
		}

		for _, market := range markets {
			if market.Closed {
				continue
			}
			if len(market.ClobTokenIDs) > 0 {
				tokenIDs = append(tokenIDs, market.ClobTokenIDs...)
				continue
			}
			if market.TokenID != "" {
				tokenIDs = append(tokenIDs, market.TokenID)
				continue
			}
			if market.TokenIDSnake != "" {
				tokenIDs = append(tokenIDs, market.TokenIDSnake)
			}
		}

		offset += polyMaxFetchLimit
	}

	return tokenIDs, nil
}

// pingLoop sends periodic pings
func (p *PolymarketConnector) pingLoop(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(polyPingInterval)
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
				p.logger.Warnf("Ping failed: %v", err)
				return
			}
		}
	}
}

// readLoop reads messages from WebSocket
func (p *PolymarketConnector) readLoop(ctx context.Context, conn *websocket.Conn) error {
	defer p.setConnected(false)

	conn.SetReadDeadline(time.Now().Add(polyPongTimeout))

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			p.metrics.RecordError(polySource, "read_error")
			return fmt.Errorf("read error: %w", err)
		}

		if err := p.processMessage(message); err != nil {
			p.logger.Warnf("Failed to process message: %v", err)
			p.metrics.RecordError(polySource, "process_error")
		}
	}
}

// processMessage processes a single message
func (p *PolymarketConnector) processMessage(data []byte) error {
	startTime := time.Now()

	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("json unmarshal error: %w", err)
	}

	// Polymarket message structure varies by type
	msgType, _ := msg["type"].(string)

	switch msgType {
	case "price_update", "book_update":
		return p.processPriceUpdate(msg, startTime)
	case "trade":
		return p.processTradeUpdate(msg, startTime)
	default:
		// Ignore other message types
		return nil
	}
}

// processPriceUpdate handles price update messages
func (p *PolymarketConnector) processPriceUpdate(msg map[string]interface{}, startTime time.Time) error {
	// Extract fields (structure varies, this is simplified)
	assetID, _ := msg["asset_id"].(string)
	if assetID == "" {
		tokenID, _ := msg["token_id"].(string)
		assetID = tokenID
	}

	if assetID == "" {
		return fmt.Errorf("missing asset_id")
	}

	// Price is usually in the range 0-1
	price, _ := msg["price"].(float64)

	// Some messages have mid_price
	if price == 0 {
		midPrice, _ := msg["mid_price"].(float64)
		price = midPrice
	}

	// Timestamp (milliseconds)
	timestamp, _ := msg["timestamp"].(float64)
	if timestamp == 0 {
		timestamp = float64(time.Now().UnixMilli())
	}

	// Check for duplicate
	cacheKey := assetID
	if lastPrice, ok := p.lastPrices.Load(cacheKey); ok {
		if lastPrice.(float64) == price {
			p.metrics.RecordDuplicate(polySource)
			return nil
		}
	}
	p.lastPrices.Store(cacheKey, price)

	// Create tick
	tick := &models.Tick{
		Source:          polySource,
		ContractID:      assetID,
		Price:           price,
		TimestampSource: int64(timestamp),
		TimestampIngest: time.Now().UnixMilli(),
	}

	// Publish to Redis
	if err := p.redis.PublishTick(tick); err != nil {
		p.metrics.RecordError(polySource, "redis_error")
		return fmt.Errorf("failed to publish: %w", err)
	}

	// Record metrics
	p.metrics.RecordMessage(polySource, tick.TimestampSource, true)
	p.metrics.RecordProcessingTime(polySource, time.Since(startTime))
	p.metrics.RecordPrice(polySource, assetID, price)

	return nil
}

// processTradeUpdate handles trade messages
func (p *PolymarketConnector) processTradeUpdate(msg map[string]interface{}, startTime time.Time) error {
	// Extract trade details
	assetID, _ := msg["asset_id"].(string)
	price, _ := msg["price"].(float64)

	if assetID == "" || price == 0 {
		return nil // Ignore incomplete trades
	}

	// Update last price based on trade
	p.lastPrices.Store(assetID, price)

	// Create tick from trade price
	tick := &models.Tick{
		Source:          polySource,
		ContractID:      assetID,
		Price:           price,
		TimestampSource: time.Now().UnixMilli(),
		TimestampIngest: time.Now().UnixMilli(),
	}

	if err := p.redis.PublishTick(tick); err != nil {
		return fmt.Errorf("failed to publish: %w", err)
	}

	p.metrics.RecordMessage(polySource, tick.TimestampSource, true)
	p.metrics.RecordProcessingTime(polySource, time.Since(startTime))

	return nil
}

// Helper methods
func (p *PolymarketConnector) setConnected(connected bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.isConnected = connected
	p.metrics.SetConnectionActive(polySource, connected)
	p.metrics.SetHealthStatus("polymarket", connected)
}

func (p *PolymarketConnector) IsConnected() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.isConnected
}
