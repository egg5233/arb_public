package database

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps a Redis client for the arb project.
type Client struct {
	rdb *redis.Client
}

// New creates a new Redis client, connects to the specified address, and
// verifies connectivity with a PING. DB defaults to 2 if the caller passes 0.
func New(addr, password string, db int) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

// Close gracefully shuts down the Redis connection.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Redis returns the underlying go-redis client for advanced usage.
func (c *Client) Redis() *redis.Client {
	return c.rdb
}

// JSONGet executes a RedisJSON JSON.GET command and returns the raw bytes.
func (c *Client) JSONGet(key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := c.rdb.Do(ctx, "JSON.GET", key, "$").Text()
	if err != nil {
		return nil, fmt.Errorf("JSON.GET %s: %w", key, err)
	}
	return []byte(result), nil
}
