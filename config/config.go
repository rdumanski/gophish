package config

import (
	"encoding/json"
	"os"

	log "github.com/rdumanski/gophish/logger"
)

// AdminServer represents the Admin server configuration details
type AdminServer struct {
	ListenURL            string   `json:"listen_url"`
	UseTLS               bool     `json:"use_tls"`
	CertPath             string   `json:"cert_path"`
	KeyPath              string   `json:"key_path"`
	CSRFKey              string   `json:"csrf_key"`
	AllowedInternalHosts []string `json:"allowed_internal_hosts"`
	TrustedOrigins       []string `json:"trusted_origins"`
}

// PhishServer represents the Phish server configuration details
type PhishServer struct {
	ListenURL     string            `json:"listen_url"`
	UseTLS        bool              `json:"use_tls"`
	CertPath      string            `json:"cert_path"`
	KeyPath       string            `json:"key_path"`
	SandboxFilter PhishFilterConfig `json:"sandbox_filter"`
}

// PhishFilterConfig controls suppression of sandbox-driven opens/clicks
// (Microsoft Defender Safe Links, Proofpoint URL Defense, etc.).
//
// MinClickSeconds: any open or click that fires within this many seconds
// of the email's SendDate is recorded as an audit event but does NOT
// bump Result.Status — the email vendor pre-fetched the URL, the user
// did not interact. 0 disables the filter.
//
// SandboxIPs: list of IPs or CIDR ranges. Requests originating from any
// of these are similarly recorded as audit events without status bump.
// Admins curate this from their sandbox vendor's published ranges.
type PhishFilterConfig struct {
	MinClickSeconds int      `json:"min_click_seconds"`
	SandboxIPs      []string `json:"sandbox_ips"`
}

// AnthropicAIConfig holds Anthropic-specific AI provider settings.
type AnthropicAIConfig struct {
	APIKey    string `json:"api_key"`
	Model     string `json:"model"`      // empty → ai.DefaultAnthropicModel
	MaxTokens int    `json:"max_tokens"` // 0 → ai.DefaultAnthropicMaxTokens
}

// AIConfig holds settings for AI-assisted features (currently template
// generation). Disabled by default — admins opt in by adding an "ai"
// block to config.json with a provider API key.
type AIConfig struct {
	Enabled   bool              `json:"enabled"`
	Provider  string            `json:"provider"` // "anthropic"
	Anthropic AnthropicAIConfig `json:"anthropic"`
}

// Config represents the configuration information.
type Config struct {
	AdminConf      AdminServer `json:"admin_server"`
	PhishConf      PhishServer `json:"phish_server"`
	DBName         string      `json:"db_name"`
	DBPath         string      `json:"db_path"`
	DBSSLCaPath    string      `json:"db_sslca_path"`
	MigrationsPath string      `json:"migrations_prefix"`
	TestFlag       bool        `json:"test_flag"`
	ContactAddress string      `json:"contact_address"`
	Logging        *log.Config `json:"logging"`
	AI             AIConfig    `json:"ai"`
}

// Version contains the current gophish version
var Version = ""

// ServerName is the server type that is returned in the transparency response.
const ServerName = "gophish"

// LoadConfig loads the configuration from the specified filepath
func LoadConfig(filepath string) (*Config, error) {
	// Get the config file
	configFile, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	config := &Config{}
	err = json.Unmarshal(configFile, config)
	if err != nil {
		return nil, err
	}
	if config.Logging == nil {
		config.Logging = &log.Config{}
	}
	// Choosing the migrations directory based on the database used.
	config.MigrationsPath = config.MigrationsPath + config.DBName
	// Explicitly set the TestFlag to false to prevent config.json overrides
	config.TestFlag = false
	return config, nil
}
