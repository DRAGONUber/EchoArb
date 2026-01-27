// internal/connectors/manifold.go
package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dragonuber/echoarb/ingestor/internal/config"
	"github.com/dragonuber/echoarb/ingestor/internal/metrics"
	"github.com/dragonuber/echoarb/ingestor/internal/models"
	"github.com/dragonuber/echoarb/ingestor/internal/redis"
	"go.uber.org/zap"
)

const (
	manifoldSource        = "MANIFOLD"
	manifoldPollInterval  = 2 * time.Second
	manifoldFastInterval  = 500 * time.Millisecond
	manifoldVolatilityThreshold = 0.02 // 2% change triggers fast polling
)

// ManifoldConnector handles Manifold API polling
type ManifoldConnector struct {
	config      *config.Config
	redis       *redis.Client
	metrics     *metrics.Registry
	logger      *zap.SugaredLogger
	httpClient  *http.Client
	lastPrices  sync.Map
	isActive    bool
	mu          sync.RWMutex
}

// ManifoldMarket represents a market from Manifold API
type ManifoldMarket struct {
	ID             string  `json:"id"`
	Slug           string  `json:"slug"`
	Question       string  `json:"question"`
	Probability    float64 `json:"probability"`
	LastUpdatedTime int64  `json:"lastUpdatedTime"`
	Volume         float64 `json:"volume"`
	IsResolved     bool    `json:"isResolved"`
}

// NewManifoldConnector creates a new Manifold connector
func NewManifoldConnector(
	cfg *config.Config,
	redisClient *redis.Client,
	metricsReg *metrics.Registry,
	logger *zap.SugaredLogger,
) *ManifoldConnector {
	return &ManifoldConnector{
		config: cfg,
		redis:  redisClient,
		metrics: metricsReg,
		logger:  logger.Named("manifold"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Start begins the polling loop
func (m *ManifoldConnector) Start(ctx context.Context) {
	m.logger.Info("Starting Manifold connector")
	m.setActive(true)
	defer m.setActive(false)

	// Collect slugs to poll
	slugs := m.collectSlugs()
	if len(slugs) == 0 {
		m.logger.Warn("No Manifold markets configured, connector will idle")
		<-ctx.Done()
		return
	}

	m.logger.Infof("Polling %d Manifold markets", len(slugs))

	// Adaptive polling interval
	currentInterval := manifoldPollInterval
	ticker := time.NewTicker(currentInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("Manifold connector stopped")
			return
		case <-ticker.C:
			startTime := time.Now()
			
			// Poll all markets
			volatile := m.pollMarkets(ctx, slugs)
			
			// Adjust polling interval based on volatility
			if volatile {
				// Switch to fast polling if prices are changing rapidly
				if currentInterval != manifoldFastInterval {
					currentInterval = manifoldFastInterval
					ticker.Reset(currentInterval)
					m.logger.Info("Switching to fast polling mode")
				}
			} else {
				// Return to normal polling
				if currentInterval != manifoldPollInterval {
					currentInterval = manifoldPollInterval
					ticker.Reset(currentInterval)
					m.logger.Info("Returning to normal polling mode")
				}
			}

			// Record polling duration
			m.metrics.RecordProcessingTime(manifoldSource, time.Since(startTime))
		}
	}
}

// collectSlugs collects all Manifold slugs from configuration
func (m *ManifoldConnector) collectSlugs() []string {
	slugs := make([]string, 0)
	for _, pair := range m.config.Pairs {
		if pair.Manifold != nil && pair.Manifold.Slug != "" {
			slugs = append(slugs, pair.Manifold.Slug)
		}
	}
	return slugs
}

// pollMarkets polls all markets and returns true if any showed volatility
func (m *ManifoldConnector) pollMarkets(ctx context.Context, slugs []string) bool {
	volatile := false

	// Batch API calls if possible (Manifold supports multi-market queries)
	markets, err := m.fetchMarkets(ctx, slugs)
	if err != nil {
		m.logger.Warnf("Failed to fetch markets: %v", err)
		m.metrics.RecordError(manifoldSource, "fetch_error")
		return false
	}

	// Process each market
	for _, market := range markets {
		if market.IsResolved {
			continue // Skip resolved markets
		}

		// Check for price change
		cacheKey := market.ID
		lastPrice, exists := m.lastPrices.Load(cacheKey)
		
		priceChanged := false
		if exists {
			change := abs(market.Probability - lastPrice.(float64))
			if change > manifoldVolatilityThreshold {
				volatile = true
			}
			if change > 0.001 { // Only publish if changed by >0.1%
				priceChanged = true
			}
		} else {
			priceChanged = true // First time seeing this market
		}

		if !priceChanged {
			m.metrics.RecordDuplicate(manifoldSource)
			continue
		}

		m.lastPrices.Store(cacheKey, market.Probability)

		// Create tick
		tick := &models.Tick{
			Source:          manifoldSource,
			ContractID:      market.Slug,
			Price:           market.Probability,
			TimestampSource: market.LastUpdatedTime,
			TimestampIngest: time.Now().UnixMilli(),
		}

		// Publish to Redis
		if err := m.redis.PublishTick(tick); err != nil {
			m.logger.Warnf("Failed to publish tick for %s: %v", market.Slug, err)
			m.metrics.RecordError(manifoldSource, "redis_error")
			continue
		}

		// Record metrics
		m.metrics.RecordMessage(manifoldSource, tick.TimestampSource, true)
		m.metrics.RecordPrice(manifoldSource, market.Slug, market.Probability)
	}

	return volatile
}

// fetchMarkets fetches markets from Manifold API
func (m *ManifoldConnector) fetchMarkets(ctx context.Context, slugs []string) ([]ManifoldMarket, error) {
	if len(slugs) == 0 {
		return nil, nil
	}

	// Build URL with slug filters
	// Manifold API supports: /v0/markets?ids=id1,id2,id3
	url := fmt.Sprintf("%s?slugs=%s", m.config.ManifoldAPIURL, strings.Join(slugs, ","))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var markets []ManifoldMarket
	if err := json.NewDecoder(resp.Body).Decode(&markets); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	m.metrics.RecordMessage(manifoldSource, time.Now().UnixMilli(), true)

	return markets, nil
}

// Helper methods
func (m *ManifoldConnector) setActive(active bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isActive = active
	m.metrics.SetConnectionActive(manifoldSource, active)
	m.metrics.SetHealthStatus("manifold", active)
}

func (m *ManifoldConnector) IsActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isActive
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}