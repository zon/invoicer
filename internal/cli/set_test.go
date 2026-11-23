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
		Project:  "Test Project",
		Database: "/var/lib/invoicer/invoices.db",
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
	if cfg.Project != "Test Project" {
		t.Errorf("Project: got %q, want %q", cfg.Project, "Test Project")
	}
	if cfg.Database != "/var/lib/invoicer/invoices.db" {
		t.Errorf("Database: got %q, want %q", cfg.Database, "/var/lib/invoicer/invoices.db")
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
database: /var/lib/invoicer/initial.db
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
	if cfg.Database != "/var/lib/invoicer/initial.db" {
		t.Errorf("Database: expected unchanged; got %q", cfg.Database)
	}
}

func TestRunSetConfig_WritesDatabase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "config.yaml")

	cmd := &SetConfigCmd{Database: "/var/lib/invoicer/invoices.db"}
	if err := RunSetConfig(cmd, path); err != nil {
		t.Fatalf("RunSetConfig: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Database != "/var/lib/invoicer/invoices.db" {
		t.Errorf("Database: got %q, want %q", cfg.Database, "/var/lib/invoicer/invoices.db")
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

func TestRunSetConfig_WritesCustomerEmail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cmd := &SetConfigCmd{CustomerEmail: "billing@acme.com"}
	if err := RunSetConfig(cmd, path); err != nil {
		t.Fatalf("RunSetConfig: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CustomerEmail != "billing@acme.com" {
		t.Errorf("CustomerEmail: got %q, want %q", cfg.CustomerEmail, "billing@acme.com")
	}
	if cfg.Resend != nil {
		t.Errorf("Resend: got %+v, want nil", cfg.Resend)
	}
}

func TestRunSetConfig_KeepsResendSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	initial := `customerEmail: old@acme.com
rate: 80
resend:
  apiKeyFile: /run/secrets/resend-api-key
  from: invoicer@example.com
`
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatalf("writing initial config: %v", err)
	}

	cmd := &SetConfigCmd{Rate: 200, CustomerEmail: "billing@acme.com"}
	if err := RunSetConfig(cmd, path); err != nil {
		t.Fatalf("RunSetConfig: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CustomerEmail != "billing@acme.com" {
		t.Errorf("CustomerEmail: got %q, want %q", cfg.CustomerEmail, "billing@acme.com")
	}
	if cfg.Rate != 200 {
		t.Errorf("Rate: got %v, want 200", cfg.Rate)
	}
	if cfg.Resend == nil || cfg.Resend.APIKeyFile != "/run/secrets/resend-api-key" || cfg.Resend.From != "invoicer@example.com" {
		t.Errorf("Resend settings were not preserved: %+v", cfg.Resend)
	}
}
