package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zon/invoicer/internal/config"
)

func boolPtr(b bool) *bool { return &b }

func TestLoad_FileNotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	// All fields should be zero values.
	if cfg.Vendor != "" || cfg.Customer != "" || cfg.Rate != 0 || cfg.Hours != 0 || cfg.PDF != nil || cfg.Model != "" {
		t.Errorf("expected empty config, got: %+v", cfg)
	}
}

func TestLoad_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `vendor: Acme Corp
customer: Big Client
rate: 150.5
hours: 40
pdf: true
model: anthropic/claude-haiku-4-5
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Vendor != "Acme Corp" {
		t.Errorf("Vendor: got %q, want %q", cfg.Vendor, "Acme Corp")
	}
	if cfg.Customer != "Big Client" {
		t.Errorf("Customer: got %q, want %q", cfg.Customer, "Big Client")
	}
	if cfg.Rate != 150.5 {
		t.Errorf("Rate: got %v, want 150.5", cfg.Rate)
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

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("rate: [not a number"), 0o600); err != nil {
		t.Fatalf("writing test config: %v", err)
	}
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestSave_CreatesFileAndDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "config.yaml")

	updates := &config.Config{
		Vendor:   "My Company",
		Customer: "Client Inc",
		Rate:     100,
		Hours:    35,
		PDF:      boolPtr(false),
		Model:    "anthropic/claude-haiku-4-5",
	}

	if err := config.Save(path, updates); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file to exist: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if cfg.Vendor != "My Company" {
		t.Errorf("Vendor: got %q, want %q", cfg.Vendor, "My Company")
	}
	if cfg.Customer != "Client Inc" {
		t.Errorf("Customer: got %q, want %q", cfg.Customer, "Client Inc")
	}
	if cfg.Rate != 100 {
		t.Errorf("Rate: got %v, want 100", cfg.Rate)
	}
	if cfg.Hours != 35 {
		t.Errorf("Hours: got %v, want 35", cfg.Hours)
	}
	if cfg.PDF == nil || *cfg.PDF != false {
		t.Errorf("PDF: got %v, want false", cfg.PDF)
	}
	if cfg.Model != "anthropic/claude-haiku-4-5" {
		t.Errorf("Model: got %q, want %q", cfg.Model, "anthropic/claude-haiku-4-5")
	}
}

func TestSave_MergesWithExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	// Write initial config.
	initial := &config.Config{
		Vendor:   "Original Vendor",
		Customer: "Original Client",
		Rate:     80,
		Hours:    40,
	}
	if err := config.Save(path, initial); err != nil {
		t.Fatalf("initial Save: %v", err)
	}

	// Update only vendor and rate.
	updates := &config.Config{
		Vendor: "New Vendor",
		Rate:   120,
	}
	if err := config.Save(path, updates); err != nil {
		t.Fatalf("update Save: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load after update: %v", err)
	}

	// Updated fields.
	if cfg.Vendor != "New Vendor" {
		t.Errorf("Vendor: got %q, want %q", cfg.Vendor, "New Vendor")
	}
	if cfg.Rate != 120 {
		t.Errorf("Rate: got %v, want 120", cfg.Rate)
	}
	// Unchanged fields.
	if cfg.Customer != "Original Client" {
		t.Errorf("Customer: got %q, want %q (should be unchanged)", cfg.Customer, "Original Client")
	}
	if cfg.Hours != 40 {
		t.Errorf("Hours: got %v, want 40 (should be unchanged)", cfg.Hours)
	}
}

func TestSave_OnlyUpdatesSpecifiedFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	// Write initial config with all fields.
	initial := &config.Config{
		Vendor:   "V1",
		Customer: "C1",
		Rate:     50,
		Hours:    20,
		PDF:      boolPtr(true),
		Model:    "anthropic/claude-haiku-4-5",
	}
	if err := config.Save(path, initial); err != nil {
		t.Fatalf("initial Save: %v", err)
	}

	// Update only PDF.
	pdfFalse := false
	if err := config.Save(path, &config.Config{PDF: &pdfFalse}); err != nil {
		t.Fatalf("update Save: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Vendor != "V1" {
		t.Errorf("Vendor should be unchanged: got %q", cfg.Vendor)
	}
	if cfg.Customer != "C1" {
		t.Errorf("Customer should be unchanged: got %q", cfg.Customer)
	}
	if cfg.Rate != 50 {
		t.Errorf("Rate should be unchanged: got %v", cfg.Rate)
	}
	if cfg.Hours != 20 {
		t.Errorf("Hours should be unchanged: got %v", cfg.Hours)
	}
	if cfg.PDF == nil || *cfg.PDF != false {
		t.Errorf("PDF should be updated to false: got %v", cfg.PDF)
	}
	if cfg.Model != "anthropic/claude-haiku-4-5" {
		t.Errorf("Model should be unchanged: got %q", cfg.Model)
	}
}
