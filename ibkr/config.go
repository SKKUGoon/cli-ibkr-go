// Package ibkr implements the OAuth-only IBKR REST client. It has no database or CLI dependency.
package ibkr

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

type CacheConfig struct {
	Mode, RedisURL, KeyPrefix string
	RefreshSkew, LockTTL      time.Duration
}
type Config struct {
	BaseURL, ConsumerKey, Realm, AccessToken, AccessTokenSecret string
	SignatureKeyPath, EncryptionKeyPath, DHParamPath            string
	Timeout                                                     time.Duration
	Cache                                                       CacheConfig
}

func DefaultConfig() Config {
	return Config{BaseURL: "https://api.ibkr.com/v1/api", Realm: "limited_poa", Timeout: 30 * time.Second,
		Cache: CacheConfig{Mode: "memory", KeyPrefix: "ibkr:oauth:lst", RefreshSkew: 60 * time.Second, LockTTL: 15 * time.Second}}
}
func (config Config) Validate() error {
	for _, field := range []struct{ name, value string }{
		{"IBKR_CONSUMER_KEY", config.ConsumerKey}, {"IBKR_REALM", config.Realm},
		{"IBKR_ACCESS_TOKEN", config.AccessToken}, {"IBKR_ACCESS_TOKEN_SECRET", config.AccessTokenSecret},
		{"IBKR_SIGNATURE_KEY_PATH", config.SignatureKeyPath}, {"IBKR_ENCRYPTION_KEY_PATH", config.EncryptionKeyPath}, {"IBKR_DH_PARAM_PATH", config.DHParamPath}} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("missing configuration: %s", field.name)
		}
	}
	endpoint, err := url.Parse(config.BaseURL)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "https" && endpoint.Scheme != "http") || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return fmt.Errorf("invalid IBKR_BASE_URL")
	}
	if config.Timeout <= 0 || config.Cache.RefreshSkew < 0 || config.Cache.LockTTL <= 0 {
		return fmt.Errorf("invalid timeout or cache duration")
	}
	switch config.Cache.Mode {
	case "memory", "disabled":
	case "redis":
		if config.Cache.RedisURL == "" {
			return fmt.Errorf("missing configuration: IBKR_REDIS_URL")
		}
	default:
		return fmt.Errorf("invalid cache mode: %s", config.Cache.Mode)
	}
	return nil
}
func (config Config) endpointURL(path string) string {
	return strings.TrimRight(config.BaseURL, "/") + "/" + strings.TrimLeft(path, "/")
}
