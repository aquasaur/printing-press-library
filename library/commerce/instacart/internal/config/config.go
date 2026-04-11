package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	AppName = "instacart"
)

type Config struct {
	UserAgent  string  `json:"user_agent"`
	PostalCode string  `json:"postal_code,omitempty"`
	AddressID  string  `json:"address_id,omitempty"`
	Latitude   float64 `json:"latitude,omitempty"`
	Longitude  float64 `json:"longitude,omitempty"`
	// ZoneID is Instacart's user-level delivery zone id. It is not returned
	// by ShopCollectionScoped and not encoded in the inventory token (field
	// [7] of the token is always "0" in practice). The Items GraphQL
	// operation requires it as a non-null variable, so the CLI reads it
	// from config with "38" as a fallback. The value appears to be per-user
	// (derived from postal code), not per-retailer -- "38" worked across
	// multiple retailers at the same address.
	ZoneID          string    `json:"zone_id,omitempty"`
	DefaultRetailer string    `json:"default_retailer,omitempty"`
	BundleSHA       string    `json:"bundle_sha,omitempty"`
	UpdatedAt       time.Time `json:"updated_at,omitempty"`
}

// EffectiveZoneID returns the user's configured zone id, defaulting to "38"
// when unset. "38" is observed to work for the original developer's postal code; users in
// other regions should run `instacart config set zone_id <value>` once.
func (c *Config) EffectiveZoneID() string {
	if c.ZoneID == "" {
		return "38"
	}
	return c.ZoneID
}

func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, AppName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaultConfig(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if c.UserAgent == "" {
		c.UserAgent = defaultUserAgent()
	}
	return &c, nil
}

func (c *Config) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}
	c.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func defaultConfig() *Config {
	return &Config{UserAgent: defaultUserAgent()}
}

func defaultUserAgent() string {
	return "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36"
}
