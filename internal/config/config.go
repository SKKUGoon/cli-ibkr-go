package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"ibkr-go/ibkr"
)

type Environment map[string]string

func LoadEnvironment(envFile string) (Environment, error) {
	environment := Environment{}
	if envFile == "" {
		var err error
		envFile, err = DefaultEnvFile()
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(envFile); os.IsNotExist(err) {
			envFile = ""
		} else if err != nil {
			return nil, err
		}
	}
	if envFile != "" {
		values, err := godotenv.Read(envFile)
		if err != nil {
			return nil, fmt.Errorf("cannot read or parse dotenv file: %s", envFile)
		}
		for key, value := range values {
			environment[key] = value
		}
	}
	for _, entry := range os.Environ() {
		key, value, _ := strings.Cut(entry, "=")
		environment[key] = value
	}
	return environment, nil
}
func (env Environment) OAuth(timeoutOverride *uint64) (ibkr.Config, error) {
	config := ibkr.DefaultConfig()
	for _, field := range []struct {
		key    string
		target *string
	}{
		{"IBKR_BASE_URL", &config.BaseURL}, {"IBKR_CONSUMER_KEY", &config.ConsumerKey}, {"IBKR_REALM", &config.Realm},
		{"IBKR_ACCESS_TOKEN", &config.AccessToken}, {"IBKR_ACCESS_TOKEN_SECRET", &config.AccessTokenSecret},
		{"IBKR_SIGNATURE_KEY_PATH", &config.SignatureKeyPath}, {"IBKR_ENCRYPTION_KEY_PATH", &config.EncryptionKeyPath}, {"IBKR_DH_PARAM_PATH", &config.DHParamPath},
		{"IBKR_LST_CACHE_MODE", &config.Cache.Mode}, {"IBKR_REDIS_URL", &config.Cache.RedisURL}, {"IBKR_REDIS_KEY_PREFIX", &config.Cache.KeyPrefix},
	} {
		if value, ok := env[field.key]; ok {
			*field.target = value
		}
	}
	for _, field := range []struct {
		key    string
		target *time.Duration
	}{{"IBKR_TIMEOUT_SECONDS", &config.Timeout}, {"IBKR_LST_REFRESH_SKEW_SECONDS", &config.Cache.RefreshSkew}, {"IBKR_LST_LOCK_TTL_SECONDS", &config.Cache.LockTTL}} {
		if value, ok := env[field.key]; ok {
			seconds, err := strconv.ParseUint(value, 10, 64)
			if err != nil || seconds > uint64((1<<63-1)/time.Second) {
				return config, fmt.Errorf("invalid %s", field.key)
			}
			*field.target = time.Duration(seconds) * time.Second
		}
	}
	if timeoutOverride != nil {
		if *timeoutOverride > uint64((1<<63-1)/time.Second) {
			return config, fmt.Errorf("invalid timeout-seconds")
		}
		config.Timeout = time.Duration(*timeoutOverride) * time.Second
	}
	config.Cache.Mode = strings.ToLower(strings.TrimSpace(config.Cache.Mode))
	return config, config.Validate()
}

var ReportKeys = []string{"IBKR_BASE_URL", "IBKR_CONSUMER_KEY", "IBKR_REALM", "IBKR_ACCESS_TOKEN", "IBKR_ACCESS_TOKEN_SECRET", "IBKR_SIGNATURE_KEY_PATH", "IBKR_ENCRYPTION_KEY_PATH", "IBKR_DH_PARAM_PATH", "IBKR_TIMEOUT_SECONDS", "IBKR_ACCOUNT_ID", "IBKR_QUICK_ORDER_PREFIX", "IBKR_DATABASE", "IBKR_LST_CACHE_MODE", "IBKR_REDIS_URL", "IBKR_REDIS_KEY_PREFIX", "IBKR_LST_REFRESH_SKEW_SECONDS", "IBKR_LST_LOCK_TTL_SECONDS", "IBKR_ORDERS_ANSWER_JSON"}

func (env Environment) Report() string {
	var report strings.Builder
	for _, key := range ReportKeys {
		value, ok := env[key]
		if !ok {
			value = "<unset>"
		} else {
			switch key {
			case "IBKR_CONSUMER_KEY", "IBKR_ACCESS_TOKEN", "IBKR_ACCESS_TOKEN_SECRET", "IBKR_DATABASE", "IBKR_REDIS_URL":
				value = "[REDACTED]"
			}
		}
		fmt.Fprintf(&report, "%s=%s\n", key, value)
	}
	return report.String()
}

// DefaultEnvFile is independent of the working directory.
func DefaultEnvFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "ibkr", ".env"), nil
}
