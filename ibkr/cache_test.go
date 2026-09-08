package ibkr_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"ibkr-go/ibkr"
	"ibkr-go/internal/testsupport"
)

func TestRedisCacheSharedAcrossClients(t *testing.T) {
	server := testsupport.NewServer(t)
	cache := miniredis.RunT(t)
	server.Config.Cache.Mode = "redis"
	server.Config.Cache.RedisURL = "redis://" + cache.Addr()
	for index := 0; index < 2; index++ {
		client, err := ibkr.NewClient(server.Config)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.Tickle(context.Background()); err != nil {
			t.Fatal(err)
		}
		_ = client.Close()
	}
	if server.SessionCount() != 1 {
		t.Fatal(server.SessionCount())
	}
	keys := cache.Keys()
	if len(keys) != 1 {
		t.Fatal(keys)
	}
	if strings.Contains(keys[0], server.Config.ConsumerKey) || strings.Contains(keys[0], server.Config.AccessToken) {
		t.Fatal("raw identifiers in key")
	}
	if cache.TTL(keys[0]) <= 0 || cache.TTL(keys[0]) >= time.Hour {
		t.Fatal(cache.TTL(keys[0]))
	}
	cache.Set(keys[0], "malformed payload")
	client, _ := ibkr.NewClient(server.Config)
	defer client.Close()
	if _, err := client.Tickle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if server.SessionCount() != 2 {
		t.Fatal("corrupt cache not refreshed")
	}
}
func TestRedisFailureDoesNotFallBackToOAuth(t *testing.T) {
	server := testsupport.NewServer(t)
	cache := miniredis.RunT(t)
	address := cache.Addr()
	cache.Close()
	server.Config.Cache.Mode = "redis"
	server.Config.Cache.RedisURL = "redis://" + address
	client, err := ibkr.NewClient(server.Config)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := client.Tickle(ctx); err == nil {
		t.Fatal("expected cache error")
	}
	if server.SessionCount() != 0 {
		t.Fatal("unexpected OAuth fallback")
	}
}
