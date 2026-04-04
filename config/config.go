package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type AuthConfig struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	Email        string    `json:"email"`
}

type Config struct {
	APIURL string      `json:"api_url"`
	Auth   *AuthConfig `json:"auth,omitempty"`
}

func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".yertle", "config.json")
}

// Load reads the config from ~/.yertle/config.json.
// Returns a default config if the file doesn't exist.
func Load() (*Config, error) {
	cfg := &Config{
		APIURL: "http://localhost:8000",
	}

	data, err := os.ReadFile(DefaultConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Save writes the config to ~/.yertle/config.json.
func (c *Config) Save() error {
	path := DefaultConfigPath()
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

func (c *Config) IsAuthenticated() bool {
	return c.Auth != nil && c.Auth.AccessToken != ""
}

func (c *Config) IsTokenExpired() bool {
	if c.Auth == nil {
		return true
	}
	return c.Auth.ExpiresAt.Before(time.Now())
}

// Resolve applies precedence: flag > env var > config > default.
// Returns the resolved API URL and org ID.
func (c *Config) Resolve(flagAPIURL, flagOrg string) (apiURL, org string) {
	// API URL
	apiURL = c.APIURL
	if v := os.Getenv("YERTLE_API_URL"); v != "" {
		apiURL = v
	}
	if flagAPIURL != "" {
		apiURL = flagAPIURL
	}

	// Org (flag or env only — no config default)
	if v := os.Getenv("YERTLE_ORG"); v != "" {
		org = v
	}
	if flagOrg != "" {
		org = flagOrg
	}

	return apiURL, org
}
