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
	config      *config.Config
	logger      *zap.SugaredLogger
	conn        *websocket.Conn
	auth        *auth.KalshiAuth
	mu          sync.RWMutex
	lastPrices  sync.Map
	marketNames sync.Map // ticker -> title cache
	msgChan     chan<- models.Tick
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
		Title  string `json:"title"`
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
		// Cache market name for later use
		if m.Title != "" {
			k.marketNames.Store(m.Ticker, m.Title)
		}
	}

	if len(tickers) == 0 {
		return nil, fmt.Errorf("discovery returned no active markets")
	}

	k.logger.Infof("Discovered %d active Kalshi markets", len(tickers))
	return tickers, nil
}

func (k *KalshiConnector) subscribe(tickers []string) error {
	k.logger.Infof("Subscribing to %d Kalshi markets...", len(tickers))

	for i, ticker := range tickers {
		subscribeMsg := map[string]interface{}{
			"id":  i + 1,
			"cmd": "subscribe",
			"params": map[string]interface{}{
				"channels":      []string{"ticker"},
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
	var envelope map[string]interface{}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return
	}

	// Kalshi messages have type at top level, data nested in "msg"
	msgType, _ := envelope["type"].(string)
	if msgType != "ticker" {
		return
	}

	// Extract nested msg object
	msgData, ok := envelope["msg"].(map[string]interface{})
	if !ok {
		return
	}

	ticker, _ := msgData["market_ticker"].(string)
	if ticker == "" {
		return
	}

	// Extract ALL available Kalshi ticker fields
	// Prices (in cents 1-99)
	yesBid, _ := msgData["yes_bid"].(float64)
	yesAsk, _ := msgData["yes_ask"].(float64)
	lastPrice, _ := msgData["price"].(float64)

	// Volume & Interest
	volume, _ := msgData["volume"].(float64)
	openInterest, _ := msgData["open_interest"].(float64)
	dollarVolume, _ := msgData["dollar_volume"].(float64)
	dollarOpenInterest, _ := msgData["dollar_open_interest"].(float64)

	// Market ID
	marketID, _ := msgData["market_id"].(string)

	// Timestamp
	sourceTS, _ := msgData["ts"].(float64)

	// Calculate Mid Price (convert from cents to 0-1)
	var price float64
	if yesBid > 0 && yesAsk > 0 {
		price = (yesBid + yesAsk) / 200.0
	} else if lastPrice > 0 {
		price = lastPrice / 100.0
	} else if yesBid > 0 {
		price = yesBid / 100.0
	} else if yesAsk > 0 {
		price = yesAsk / 100.0
	} else {
		return
	}

	// Deduplication check
	if cached, ok := k.lastPrices.Load(ticker); ok {
		if cached.(float64) == price {
			return
		}
	}
	k.lastPrices.Store(ticker, price)

	// Get cached market name
	var marketName string
	if name, ok := k.marketNames.Load(ticker); ok {
		marketName = name.(string)
	}

	// Send to Channel with ALL fields
	k.msgChan <- models.Tick{
		Source:          "KALSHI",
		ContractID:      ticker,
		Price:           price,
		TimestampSource: int64(sourceTS * 1000),
		TimestampIngest: time.Now().UnixMilli(),

		// Prices (converted to 0-1)
		YesBid:    yesBid / 100.0,
		YesAsk:    yesAsk / 100.0,
		NoBid:     (100 - yesAsk) / 100.0, // Derived: no_bid = 1 - yes_ask
		NoAsk:     (100 - yesBid) / 100.0, // Derived: no_ask = 1 - yes_bid
		LastPrice: lastPrice / 100.0,

		// Volume
		Volume:        int64(volume),
		OpenInterest:  int64(openInterest),
		DollarVolume:  int64(dollarVolume),
		DollarOpenInt: int64(dollarOpenInterest),

		// Metadata
		MarketID:   marketID,
		MarketName: marketName,
		EventType:  "ticker",
	}
}