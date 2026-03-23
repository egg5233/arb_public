package database

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

func lockKey(resource string) string {
	return fmt.Sprintf("arb:locks:%s", resource)
}

// AcquireLock attempts to acquire a distributed lock for the given resource
// using SET NX with the specified TTL. Returns true if the lock was acquired.
func (c *Client) AcquireLock(resource string, ttl time.Duration) (bool, error) {
	ctx := context.Background()
	ok, err := c.rdb.SetNX(ctx, lockKey(resource), "1", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("acquire lock %s: %w", resource, err)
	}
	return ok, nil
}

// ReleaseLock deletes the lock key, releasing the distributed lock.
func (c *Client) ReleaseLock(resource string) error {
	ctx := context.Background()
	err := c.rdb.Del(ctx, lockKey(resource)).Err()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("release lock %s: %w", resource, err)
	}
	return nil
}
