// Package config handles reading and writing of ~/.invoicer/config.yaml.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds configuration values for invoicer.
// Only pointer fields are written when explicitly set; nil means "not configured".
type Config struct {
	Vendor   string  `yaml:"vendor,omitempty"`
	Customer string  `yaml:"customer,omitempty"`
	Rate     float64 `yaml:"rate,omitempty"`
	Hours    float64 `yaml:"hours,omitempty"`
	PDF      *bool   `yaml:"pdf,omitempty"`
	Model    string  `yaml:"model,omitempty"`
}

// DefaultPath returns the default path to the config file (~/.invoicer/config.yaml).
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, ".invoicer", "config.yaml"), nil
}

// Load reads the config file at the given path.
// If the file does not exist, an empty Config is returned without error.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", path, err)
	}
	return &cfg, nil
}

// Save writes the config to the given path, creating the directory and file if needed.
// It merges the provided updates into any existing config, only overwriting fields
// that are explicitly set in updates.
func Save(path string, updates *Config) error {
	// Load existing config (if any) so we only update specified fields.
	existing, err := Load(path)
	if err != nil {
		return err
	}

	// Merge: updates take precedence over existing values.
	if updates.Vendor != "" {
		existing.Vendor = updates.Vendor
	}
	if updates.Customer != "" {
		existing.Customer = updates.Customer
	}
	if updates.Rate != 0 {
		existing.Rate = updates.Rate
	}
	if updates.Hours != 0 {
		existing.Hours = updates.Hours
	}
	if updates.PDF != nil {
		existing.PDF = updates.PDF
	}
	if updates.Model != "" {
		existing.Model = updates.Model
	}

	// Ensure the directory exists.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config directory %q: %w", dir, err)
	}

	data, err := yaml.Marshal(existing)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing config file %q: %w", path, err)
	}
	return nil
}
