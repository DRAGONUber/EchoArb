// internal/redis/client.go
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/vmihailenco/msgpack/v5"
	"github.com/dragonuber/echoarb/ingestor/internal/config"
	"github.com/dragonuber/echoarb/ingestor/internal/models"
	"go.uber.org/zap"
)

const (
	StreamName     = "market_ticks"
	StreamMaxLen   = 10000
	PubSubPrefix   = "tick:"
)

// Client wraps Redis client with domain-specific methods
type Client struct {
	rdb    *redis.Client
	logger *zap.SugaredLogger
	ctx    context.Context
}

// NewClient creates a new Redis client
func NewClient(cfg config.RedisConfig, logger *zap.SugaredLogger) (*Client, error) {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
	})

	// Test connection
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Infof("Connected to Redis at %s", addr)

	return &Client{
		rdb:    rdb,
		logger: logger,
		ctx:    ctx,
	}, nil
}

// PublishTick publishes a tick to Redis Stream and Pub/Sub
func (c *Client) PublishTick(tick *models.Tick) error {
	// Validate tick
	if err := tick.Validate(); err != nil {
		return fmt.Errorf("invalid tick: %w", err)
	}

	// Serialize with msgpack
	data, err := msgpack.Marshal(tick)
	if err != nil {
		return fmt.Errorf("msgpack marshal error: %w", err)
	}

	// Use pipeline to batch Redis commands
	pipe := c.rdb.Pipeline()

	// Add to Stream
	pipe.XAdd(c.ctx, &redis.XAddArgs{
		Stream: StreamName,
		MaxLen: StreamMaxLen,
		Approx: true, // Use approximate trimming for performance
		Values: map[string]interface{}{
			"data": data,
		},
	})

	// Publish to Pub/Sub channel
	channel := fmt.Sprintf("%s%s", PubSubPrefix, tick.ContractID)
	pipe.Publish(c.ctx, channel, data)

	// Execute pipeline
	_, err = pipe.Exec(c.ctx)
	if err != nil {
		return fmt.Errorf("redis pipeline error: %w", err)
	}

	return nil
}

// GetLatestTicks retrieves the last N ticks from the stream
func (c *Client) GetLatestTicks(count int64) ([]models.Tick, error) {
	// Read from stream
	messages, err := c.rdb.XRevRangeN(c.ctx, StreamName, "+", "-", count).Result()
	if err != nil {
		return nil, fmt.Errorf("redis xrevrange error: %w", err)
	}

	ticks := make([]models.Tick, 0, len(messages))
	
	for _, msg := range messages {
		data, ok := msg.Values["data"].(string)
		if !ok {
			continue
		}

		var tick models.Tick
		if err := msgpack.Unmarshal([]byte(data), &tick); err != nil {
			c.logger.Warnf("Failed to unmarshal tick: %v", err)
			continue
		}

		ticks = append(ticks, tick)
	}

	return ticks, nil
}

// GetStreamLength returns the number of messages in the stream
func (c *Client) GetStreamLength() (int64, error) {
	return c.rdb.XLen(c.ctx, StreamName).Result()
}

// TrimStream trims the stream to the specified length
func (c *Client) TrimStream(maxLen int64) error {
	return c.rdb.XTrimMaxLen(c.ctx, StreamName, maxLen).Err()
}

// SubscribeToTicks creates a subscription to tick updates
func (c *Client) SubscribeToTicks(pattern string) *redis.PubSub {
	// Subscribe to pattern (e.g., "tick:*")
	return c.rdb.PSubscribe(c.ctx, pattern)
}

// SetValue sets a simple key-value pair
func (c *Client) SetValue(key string, value interface{}, expiration time.Duration) error {
	return c.rdb.Set(c.ctx, key, value, expiration).Err()
}

// GetValue gets a simple value by key
func (c *Client) GetValue(key string) (string, error) {
	return c.rdb.Get(c.ctx, key).Result()
}

// DeleteKey deletes a key
func (c *Client) DeleteKey(key string) error {
	return c.rdb.Del(c.ctx, key).Err()
}

// IncrementCounter increments a counter
func (c *Client) IncrementCounter(key string) (int64, error) {
	return c.rdb.Incr(c.ctx, key).Result()
}

// GetStats returns Redis statistics
func (c *Client) GetStats() (*RedisStats, error) {
	info, err := c.rdb.Info(c.ctx, "stats").Result()
	if err != nil {
		return nil, err
	}

	streamLen, err := c.GetStreamLength()
	if err != nil {
		streamLen = 0
	}

	return &RedisStats{
		StreamLength:    streamLen,
		ConnectedClients: c.extractStat(info, "connected_clients"),
		UsedMemory:      c.extractStat(info, "used_memory"),
	}, nil
}

// RedisStats holds Redis statistics
type RedisStats struct {
	StreamLength     int64
	ConnectedClients string
	UsedMemory       string
}

func (c *Client) extractStat(info, key string) string {
	// Simple extraction - in production use proper parsing
	return "N/A"
}

// Close closes the Redis connection
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Ping tests the connection
func (c *Client) Ping() error {
	return c.rdb.Ping(c.ctx).Err()
}