package config

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	log "github.com/rdumanski/gophish/logger"
)

var validConfig = []byte(`{
	"admin_server": {
		"listen_url": "127.0.0.1:3333",
		"use_tls": true,
		"cert_path": "gophish_admin.crt",
		"key_path": "gophish_admin.key"
	},
	"phish_server": {
		"listen_url": "0.0.0.0:8080",
		"use_tls": false,
		"cert_path": "example.crt",
		"key_path": "example.key"
	},
	"db_name": "sqlite3",
	"db_path": "gophish.db",
	"migrations_prefix": "db/db_",
	"contact_address": ""
}`)

func createTemporaryConfig(t *testing.T) *os.File {
	f, err := os.CreateTemp("", "gophish-config")
	if err != nil {
		t.Fatalf("unable to create temporary config: %v", err)
	}
	return f
}

func removeTemporaryConfig(t *testing.T, f *os.File) {
	err := f.Close()
	if err != nil {
		t.Fatalf("unable to remove temporary config: %v", err)
	}
}

func TestLoadConfig(t *testing.T) {
	f := createTemporaryConfig(t)
	defer removeTemporaryConfig(t, f)
	_, err := f.Write(validConfig)
	if err != nil {
		t.Fatalf("error writing config to temporary file: %v", err)
	}
	// Load the valid config
	conf, err := LoadConfig(f.Name())
	if err != nil {
		t.Fatalf("error loading config from temporary file: %v", err)
	}

	expectedConfig := &Config{}
	err = json.Unmarshal(validConfig, &expectedConfig)
	if err != nil {
		t.Fatalf("error unmarshaling config: %v", err)
	}
	expectedConfig.MigrationsPath = expectedConfig.MigrationsPath + expectedConfig.DBName
	expectedConfig.TestFlag = false
	expectedConfig.AdminConf.CSRFKey = ""
	expectedConfig.Logging = &log.Config{}
	if !reflect.DeepEqual(expectedConfig, conf) {
		t.Fatalf("invalid config received. expected %#v got %#v", expectedConfig, conf)
	}

	// Load an invalid config
	_, err = LoadConfig("bogusfile")
	if err == nil {
		t.Fatalf("expected error when loading invalid config, but got %v", err)
	}
}

func TestLoadConfigWithAIBlock(t *testing.T) {
	withAI := []byte(`{
		"admin_server": {"listen_url": "127.0.0.1:3333", "use_tls": false, "cert_path": "x", "key_path": "y"},
		"phish_server": {"listen_url": "0.0.0.0:8080", "use_tls": false, "cert_path": "x", "key_path": "y"},
		"db_name": "sqlite3",
		"db_path": "gophish.db",
		"migrations_prefix": "db/db_",
		"contact_address": "",
		"ai": {
			"enabled": true,
			"provider": "anthropic",
			"anthropic": {
				"api_key": "sk-ant-test",
				"model": "claude-sonnet-4-6",
				"max_tokens": 8192
			}
		}
	}`)
	f := createTemporaryConfig(t)
	defer removeTemporaryConfig(t, f)
	if _, err := f.Write(withAI); err != nil {
		t.Fatalf("write: %v", err)
	}
	conf, err := LoadConfig(f.Name())
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !conf.AI.Enabled {
		t.Errorf("AI.Enabled: got false, want true")
	}
	if conf.AI.Provider != "anthropic" {
		t.Errorf("AI.Provider: got %q, want anthropic", conf.AI.Provider)
	}
	if conf.AI.Anthropic.APIKey != "sk-ant-test" {
		t.Errorf("AI.Anthropic.APIKey: got %q, want sk-ant-test", conf.AI.Anthropic.APIKey)
	}
	if conf.AI.Anthropic.Model != "claude-sonnet-4-6" {
		t.Errorf("AI.Anthropic.Model: got %q, want claude-sonnet-4-6", conf.AI.Anthropic.Model)
	}
	if conf.AI.Anthropic.MaxTokens != 8192 {
		t.Errorf("AI.Anthropic.MaxTokens: got %d, want 8192", conf.AI.Anthropic.MaxTokens)
	}
}
