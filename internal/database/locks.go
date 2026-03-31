package database

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	refreshLockScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
return 0
`)
	releaseLockScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`)
)

func lockKey(resource string) string {
	return fmt.Sprintf("arb:locks:%s", resource)
}

// OwnedLock is a Redis lease that can only be refreshed/released by the
// caller that acquired it.
type OwnedLock struct {
	client   *Client
	resource string
	token    string
	ttl      time.Duration

	stopRenew chan struct{}
	renewDone chan struct{}

	releaseOnce sync.Once
	releaseErr  error
}

func newLockToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate lock token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// AcquireOwnedLock attempts to acquire a distributed lock for the given
// resource. The lock stores a unique ownership token, renews its TTL while
// held, and releases with compare-and-delete semantics.
func (c *Client) AcquireOwnedLock(resource string, ttl time.Duration) (*OwnedLock, bool, error) {
	token, err := newLockToken()
	if err != nil {
		return nil, false, err
	}

	ctx := context.Background()
	ok, err := c.rdb.SetNX(ctx, lockKey(resource), token, ttl).Result()
	if err != nil {
		return nil, false, fmt.Errorf("acquire lock %s: %w", resource, err)
	}
	if !ok {
		return nil, false, nil
	}

	lock := &OwnedLock{
		client:    c,
		resource:  resource,
		token:     token,
		ttl:       ttl,
		stopRenew: make(chan struct{}),
		renewDone: make(chan struct{}),
	}
	go lock.renewLoop()
	return lock, true, nil
}

func (l *OwnedLock) renewLoop() {
	defer close(l.renewDone)

	interval := l.ttl / 3
	if interval <= 0 {
		interval = time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ok, err := l.refresh()
			if err != nil || !ok {
				return
			}
		case <-l.stopRenew:
			return
		}
	}
}

func (l *OwnedLock) refresh() (bool, error) {
	ctx := context.Background()
	result, err := refreshLockScript.Run(ctx, l.client.rdb, []string{lockKey(l.resource)}, l.token, l.ttl.Milliseconds()).Int()
	if err != nil && err != redis.Nil {
		return false, fmt.Errorf("refresh lock %s: %w", l.resource, err)
	}
	return result == 1, nil
}

// Release stops TTL renewal and deletes the Redis key only if this caller
// still owns the lock token.
func (l *OwnedLock) Release() error {
	l.releaseOnce.Do(func() {
		close(l.stopRenew)
		<-l.renewDone

		ctx := context.Background()
		_, err := releaseLockScript.Run(ctx, l.client.rdb, []string{lockKey(l.resource)}, l.token).Result()
		if err != nil && err != redis.Nil {
			l.releaseErr = fmt.Errorf("release lock %s: %w", l.resource, err)
		}
	})
	return l.releaseErr
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
