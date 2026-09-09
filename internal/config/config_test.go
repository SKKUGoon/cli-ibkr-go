package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDotenvPrecedenceAndMissingFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "test.env")
	if err := os.WriteFile(file, []byte("IBKR_CONSUMER_KEY=from-file\nIBKR_REALM=limited_poa\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("IBKR_CONSUMER_KEY", "from-process")
	env, err := LoadEnvironment(file)
	if err != nil || env["IBKR_CONSUMER_KEY"] != "from-process" {
		t.Fatal(env, err)
	}
	if _, err = LoadEnvironment(file + "missing"); err == nil {
		t.Fatal("missing explicit dotenv accepted")
	}
}
func TestConfigurationDefaultsValidationAndRedaction(t *testing.T) {
	env := Environment{"IBKR_CONSUMER_KEY": "consumer", "IBKR_ACCESS_TOKEN": "access", "IBKR_ACCESS_TOKEN_SECRET": "secret", "IBKR_SIGNATURE_KEY_PATH": "sig.pem", "IBKR_ENCRYPTION_KEY_PATH": "enc.pem", "IBKR_DH_PARAM_PATH": "dh.pem"}
	config, err := env.OAuth(nil)
	if err != nil || config.Timeout != 30*time.Second || config.Cache.Mode != "memory" {
		t.Fatal(config.Timeout, err)
	}
	for _, key := range []string{"IBKR_TIMEOUT_SECONDS", "IBKR_LST_REFRESH_SKEW_SECONDS", "IBKR_LST_LOCK_TTL_SECONDS"} {
		env[key] = "bad"
		if _, err = env.OAuth(nil); err == nil {
			t.Fatal(key)
		}
		delete(env, key)
	}
	env["IBKR_LST_CACHE_MODE"] = "redis"
	if _, err = env.OAuth(nil); err == nil {
		t.Fatal("redis without URL")
	}
	delete(env, "IBKR_LST_CACHE_MODE")
	if strings.Contains(env.Report(), "=secret") || strings.Contains(env.Report(), "=access") {
		t.Fatal("secret leaked")
	}
	delete(env, "IBKR_ACCESS_TOKEN")
	if _, err = env.OAuth(nil); err == nil {
		t.Fatal("missing token")
	}
}

func TestDefaultConfigurationIgnoresWorkingDirectoryAndParents(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	parent := t.TempDir()
	child := filepath.Join(parent, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{parent, child} {
		if err := os.WriteFile(filepath.Join(directory, ".env"), []byte("IBKR_REALM=unwanted\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(child)
	environment, err := LoadEnvironment("")
	if err != nil || environment["IBKR_REALM"] == "unwanted" {
		t.Fatal("loaded unrelated dotenv", err)
	}
	destination, err := DefaultEnvFile()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("IBKR_REALM=dedicated\n"), 0600); err != nil {
		t.Fatal(err)
	}
	environment, err = LoadEnvironment("")
	if err != nil || environment["IBKR_REALM"] != "dedicated" {
		t.Fatal("dedicated configuration not loaded", err)
	}
	explicit := filepath.Join(parent, ".env")
	environment, err = LoadEnvironment(explicit)
	if err != nil || environment["IBKR_REALM"] != "unwanted" {
		t.Fatal("explicit dotenv not loaded", err)
	}
}
