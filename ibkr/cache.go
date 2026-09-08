package ibkr

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

func cacheKey(config Config) string {
	parts := []string{config.Cache.KeyPrefix}
	for _, value := range []string{config.BaseURL, config.ConsumerKey, config.AccessToken, config.Realm} {
		digest := sha256.Sum256([]byte(value))
		parts = append(parts, hex.EncodeToString(digest[:16]))
	}
	return strings.Join(parts, ":")
}
func (client *Client) getLiveSession(ctx context.Context) (liveSession, error) {
	switch client.config.Cache.Mode {
	case "disabled":
		return client.requestLiveSession(ctx)
	case "redis":
		return client.getRedisSession(ctx)
	default:
		client.mutex.Lock()
		defer client.mutex.Unlock()
		if client.session.valid(client.config.Cache.RefreshSkew) {
			return client.session, nil
		}
		session, err := client.requestLiveSession(ctx)
		if err == nil {
			client.session = session
		}
		return session, err
	}
}
func (client *Client) readRedisSession(ctx context.Context) (liveSession, bool, error) {
	raw, err := client.redis.Get(ctx, cacheKey(client.config)).Bytes()
	if errors.Is(err, redis.Nil) {
		return liveSession{}, false, nil
	}
	if err != nil {
		return liveSession{}, false, fmt.Errorf("live session cache read: %w", err)
	}
	var session liveSession
	if json.Unmarshal(raw, &session) != nil {
		return liveSession{}, false, nil
	}
	return session, session.valid(client.config.Cache.RefreshSkew), nil
}
func (client *Client) getRedisSession(ctx context.Context) (session liveSession, err error) {
	if cached, valid, readErr := client.readRedisSession(ctx); readErr != nil || valid {
		return cached, readErr
	}
	lockValue, err := randomHex(16)
	if err != nil {
		return session, err
	}
	lockKey := cacheKey(client.config) + ":lock"
	acquired, err := client.redis.SetNX(ctx, lockKey, lockValue, client.config.Cache.LockTTL).Result()
	if err != nil {
		return session, fmt.Errorf("live session cache lock: %w", err)
	}
	if !acquired {
		return client.waitRedisSession(ctx)
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), client.config.Timeout)
		defer cancel()
		_, unlockErr := client.redis.Eval(unlockCtx, `if redis.call('get',KEYS[1]) == ARGV[1] then return redis.call('del',KEYS[1]) else return 0 end`, []string{lockKey}, lockValue).Result()
		if unlockErr != nil {
			err = errors.Join(err, fmt.Errorf("live session cache unlock: %w", unlockErr))
		}
	}()
	if cached, valid, readErr := client.readRedisSession(ctx); readErr != nil || valid {
		return cached, readErr
	}
	session, err = client.requestLiveSession(ctx)
	if err != nil {
		return session, err
	}
	ttl := time.Until(time.UnixMilli(session.ExpirationMS).Add(-client.config.Cache.RefreshSkew))
	if ttl <= 0 {
		return session, fmt.Errorf("refusing to cache expired live session token")
	}
	raw, err := json.Marshal(session)
	if err != nil {
		return session, err
	}
	err = client.redis.Set(ctx, cacheKey(client.config), raw, ttl).Err()
	return session, err
}
func (client *Client) waitRedisSession(ctx context.Context) (liveSession, error) {
	timeout := time.NewTimer(client.config.Cache.LockTTL + time.Second)
	defer timeout.Stop()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return liveSession{}, ctx.Err()
		case <-timeout.C:
			return liveSession{}, fmt.Errorf("timed out waiting for live session token refresh lock")
		case <-ticker.C:
			session, valid, err := client.readRedisSession(ctx)
			if err != nil || valid {
				return session, err
			}
		}
	}
}
