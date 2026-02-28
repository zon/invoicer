package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zon/invoicer/internal/config"
)

func TestRunSetConfig_CreatesConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cmd := &SetConfigCmd{
		Vendor:   "Test Vendor",
		Customer: "Test Client",
		Rate:     120,
		Hours:    40,
		PDF:      boolPtr(true),
		Model:    "anthropic/claude-haiku-4-5",
	}

	if err := RunSetConfig(cmd, path); err != nil {
		t.Fatalf("RunSetConfig: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Vendor != "Test Vendor" {
		t.Errorf("Vendor: got %q, want %q", cfg.Vendor, "Test Vendor")
	}
	if cfg.Customer != "Test Client" {
		t.Errorf("Customer: got %q, want %q", cfg.Customer, "Test Client")
	}
	if cfg.Rate != 120 {
		t.Errorf("Rate: got %v, want 120", cfg.Rate)
	}
	if cfg.Hours != 40 {
		t.Errorf("Hours: got %v, want 40", cfg.Hours)
	}
	if cfg.PDF == nil || !*cfg.PDF {
		t.Errorf("PDF: got %v, want true", cfg.PDF)
	}
	if cfg.Model != "anthropic/claude-haiku-4-5" {
		t.Errorf("Model: got %q, want %q", cfg.Model, "anthropic/claude-haiku-4-5")
	}
}

func TestRunSetConfig_OnlyUpdatesSpecifiedFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	// Write initial config.
	initial := `vendor: Initial Vendor
customer: Initial Customer
rate: 80
hours: 30
`
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatalf("writing initial config: %v", err)
	}

	// Only update vendor.
	cmd := &SetConfigCmd{
		Vendor: "Updated Vendor",
	}
	if err := RunSetConfig(cmd, path); err != nil {
		t.Fatalf("RunSetConfig: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Vendor != "Updated Vendor" {
		t.Errorf("Vendor: got %q, want Updated Vendor", cfg.Vendor)
	}
	// Unchanged fields should persist.
	if cfg.Customer != "Initial Customer" {
		t.Errorf("Customer: expected unchanged; got %q", cfg.Customer)
	}
	if cfg.Rate != 80 {
		t.Errorf("Rate: expected unchanged; got %v", cfg.Rate)
	}
	if cfg.Hours != 30 {
		t.Errorf("Hours: expected unchanged; got %v", cfg.Hours)
	}
}

func TestRunSetConfig_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "config.yaml")

	cmd := &SetConfigCmd{
		Vendor: "Test Vendor",
	}
	if err := RunSetConfig(cmd, path); err != nil {
		t.Fatalf("RunSetConfig: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected config file to exist: %v", err)
	}
}
