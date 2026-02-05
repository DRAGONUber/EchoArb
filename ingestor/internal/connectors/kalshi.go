// internal/connectors/kalshi.go
package connectors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/dragonuber/echoarb/ingestor/internal/auth"
	"github.com/dragonuber/echoarb/ingestor/internal/config"
	"github.com/dragonuber/echoarb/ingestor/internal/models"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type KalshiConnector struct {
	config     *config.Config
	logger     *zap.SugaredLogger
	conn       *websocket.Conn
	auth       *auth.KalshiAuth
	mu         sync.RWMutex
	lastPrices sync.Map
	msgChan    chan<- models.Tick
}

func NewKalshiConnector(cfg *config.Config, logger *zap.SugaredLogger, msgChan chan<- models.Tick) *KalshiConnector {
	// Initialize Auth
	authenticator, err := auth.NewKalshiAuth(cfg.KalshiAPIKey, cfg.KalshiPrivateKeyPEM)
	if err != nil {
		logger.Fatalf("Failed to initialize Kalshi auth: %v", err)
	}

	return &KalshiConnector{
		config:  cfg,
		logger:  logger,
		msgChan: msgChan,
		auth:    authenticator,
	}
}

func (k *KalshiConnector) Start() error {
	k.logger.Info("Starting Kalshi connector")
	return k.connect()
}

func (k *KalshiConnector) connect() error {
	k.logger.Info("Connecting to Kalshi WebSocket...")

	// 1. Get Auth Headers
	headers, err := k.auth.GetWebSocketHeaders()
	if err != nil {
		return fmt.Errorf("failed to get auth headers: %w", err)
	}

	// 2. Fetch active markets (Firehose)
	tickers, err := k.fetchActiveMarkets()
	if err != nil {
		return fmt.Errorf("failed to fetch markets: %w", err)
	}

	// 3. Connect with Headers
	conn, _, err := websocket.DefaultDialer.Dial(k.config.KalshiWSURL, headers)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}

	k.mu.Lock()
	k.conn = conn
	k.mu.Unlock()

	k.logger.Info("Connected to Kalshi")

	// 4. Subscribe
	if err := k.subscribe(tickers); err != nil {
		return err
	}

	// 5. Read loop
	go k.readLoop()

	return nil
}

type KalshiMarketResponse struct {
	Markets []struct {
		Ticker string `json:"ticker"`
	} `json:"markets"`
	Cursor string `json:"cursor"`
}

func (k *KalshiConnector) fetchActiveMarkets() ([]string, error) {
	k.logger.Info("Fetching all OPEN Kalshi markets via REST API...")

	url := "https://api.elections.kalshi.com/trade-api/v2/markets?status=open&limit=1000"

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("api returned status: %s", resp.Status)
	}

	var result KalshiMarketResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode error: %w", err)
	}

	var tickers []string
	for _, m := range result.Markets {
		tickers = append(tickers, m.Ticker)
	}
	return tickers, nil
}

func (k *KalshiConnector) subscribe(tickers []string) error {
	k.logger.Infof("Subscribing to %d Kalshi markets...", len(tickers))

	for _, ticker := range tickers {
		subscribeMsg := map[string]interface{}{
			"id":  1,
			"cmd": "subscribe",
			"params": map[string]interface{}{
				"channels":      []string{"orderbook_delta"},
				"market_ticker": ticker,
			},
		}

		k.mu.Lock()
		if err := k.conn.WriteJSON(subscribeMsg); err != nil {
			k.mu.Unlock()
			continue
		}
		k.mu.Unlock()
		
		time.Sleep(10 * time.Millisecond) // Rate limit protection
	}
	return nil
}

func (k *KalshiConnector) readLoop() {
	defer k.conn.Close()

	for {
		_, message, err := k.conn.ReadMessage()
		if err != nil {
			k.logger.Error("Read error:", err)
			return
		}
		k.processMessage(message)
	}
}

func (k *KalshiConnector) processMessage(data []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}

	if msgType, ok := msg["type"].(string); !ok || msgType != "orderbook_delta" {
		return
	}

	ticker, _ := msg["ticker"].(string)
	if ticker == "" {
		return
	}

	yesBid, _ := msg["yes_bid"].(float64)
	yesAsk, _ := msg["yes_ask"].(float64)
	sourceTS, _ := msg["ts"].(float64)

	// Calculate Mid Price
	price := (yesBid + yesAsk) / 200.0

	// Deduplication check
	if lastPrice, ok := k.lastPrices.Load(ticker); ok {
		if lastPrice.(float64) == price {
			return 
		}
	}
	k.lastPrices.Store(ticker, price)

	// Send to Channel
	k.msgChan <- models.Tick{
		Source:          "KALSHI",
		ContractID:      ticker,
		Price:           price,
		TimestampSource: int64(sourceTS * 1000), 
		TimestampIngest: time.Now().UnixMilli(),
	}
}