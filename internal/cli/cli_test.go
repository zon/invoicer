package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func boolPtr(b bool) *bool { return &b }

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing test config: %v", err)
	}
	return path
}

func TestResolveOptions_NoConfig(t *testing.T) {
	c := &CLI{
		Vendor:   "My Vendor",
		Customer: "My Customer",
		Rate:     100,
		Hours:    40,
		PDF:      true,
		Model:    "anthropic/claude-haiku-4-5",
	}
	path := filepath.Join(t.TempDir(), "nonexistent.yaml")
	opts, err := c.resolveOptions(path)
	if err != nil {
		t.Fatalf("resolveOptions: %v", err)
	}
	if opts.Vendor != "My Vendor" {
		t.Errorf("Vendor: got %q, want %q", opts.Vendor, "My Vendor")
	}
	if opts.Customer != "My Customer" {
		t.Errorf("Customer: got %q, want %q", opts.Customer, "My Customer")
	}
	if opts.Rate != 100 {
		t.Errorf("Rate: got %v, want 100", opts.Rate)
	}
	if opts.Hours != 40 {
		t.Errorf("Hours: got %v, want 40", opts.Hours)
	}
	if !opts.PDF {
		t.Error("PDF: got false, want true")
	}
	if opts.Model != "anthropic/claude-haiku-4-5" {
		t.Errorf("Model: got %q, want %q", opts.Model, "anthropic/claude-haiku-4-5")
	}
}

func TestResolveOptions_CLIOverridesConfig(t *testing.T) {
	path := writeTestConfig(t, `vendor: Config Vendor
customer: Config Customer
rate: 50
hours: 20
pdf: false
model: anthropic/claude-haiku-4-5
`)
	c := &CLI{
		Vendor:   "CLI Vendor",
		Customer: "CLI Customer",
		Rate:     150,
		Hours:    35,
		PDF:      true,
		Model:    "anthropic/claude-sonnet-4-6",
	}
	opts, err := c.resolveOptions(path)
	if err != nil {
		t.Fatalf("resolveOptions: %v", err)
	}
	if opts.Vendor != "CLI Vendor" {
		t.Errorf("Vendor: CLI should override config; got %q", opts.Vendor)
	}
	if opts.Customer != "CLI Customer" {
		t.Errorf("Customer: CLI should override config; got %q", opts.Customer)
	}
	if opts.Rate != 150 {
		t.Errorf("Rate: CLI should override config; got %v", opts.Rate)
	}
	if opts.Hours != 35 {
		t.Errorf("Hours: CLI should override config; got %v", opts.Hours)
	}
	if !opts.PDF {
		t.Error("PDF: CLI true should override config false")
	}
	if opts.Model != "anthropic/claude-sonnet-4-6" {
		t.Errorf("Model: CLI should override config; got %q", opts.Model)
	}
}

func TestResolveOptions_ConfigFallback(t *testing.T) {
	path := writeTestConfig(t, `vendor: Config Vendor
customer: Config Customer
rate: 50
hours: 20
pdf: true
model: anthropic/claude-haiku-4-5
`)
	// CLI provides no values (zero values).
	c := &CLI{}
	opts, err := c.resolveOptions(path)
	if err != nil {
		t.Fatalf("resolveOptions: %v", err)
	}
	if opts.Vendor != "Config Vendor" {
		t.Errorf("Vendor: expected config fallback; got %q", opts.Vendor)
	}
	if opts.Customer != "Config Customer" {
		t.Errorf("Customer: expected config fallback; got %q", opts.Customer)
	}
	if opts.Rate != 50 {
		t.Errorf("Rate: expected config fallback; got %v", opts.Rate)
	}
	if opts.Hours != 20 {
		t.Errorf("Hours: expected config fallback; got %v", opts.Hours)
	}
	if !opts.PDF {
		t.Error("PDF: expected config fallback true; got false")
	}
	// Model: c.Model is "" (zero) so config is used, but actually since kong sets
	// the default "anthropic/claude-haiku-4-5" at parse time, in unit tests c.Model
	// is "" — so here we expect the config value.
	if opts.Model != "anthropic/claude-haiku-4-5" {
		t.Errorf("Model: expected config fallback; got %q", opts.Model)
	}
}

func TestResolveOptions_PartialCLIOverride(t *testing.T) {
	path := writeTestConfig(t, `vendor: Config Vendor
customer: Config Customer
rate: 50
hours: 20
`)
	// CLI only provides rate and vendor.
	c := &CLI{
		Vendor: "CLI Vendor",
		Rate:   200,
	}
	opts, err := c.resolveOptions(path)
	if err != nil {
		t.Fatalf("resolveOptions: %v", err)
	}
	// CLI-provided.
	if opts.Vendor != "CLI Vendor" {
		t.Errorf("Vendor: got %q, want CLI Vendor", opts.Vendor)
	}
	if opts.Rate != 200 {
		t.Errorf("Rate: got %v, want 200", opts.Rate)
	}
	// Config fallback.
	if opts.Customer != "Config Customer" {
		t.Errorf("Customer: expected config fallback; got %q", opts.Customer)
	}
	if opts.Hours != 20 {
		t.Errorf("Hours: expected config fallback; got %v", opts.Hours)
	}
}
