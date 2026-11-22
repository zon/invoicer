package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zon/invoicer/internal/config"
)

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
	if cfg.Vendor != "" || cfg.Customer != "" || cfg.Rate != 0 || cfg.Hours != 0 || cfg.Project != "" || cfg.Database != "" {
		t.Errorf("expected empty config, got: %+v", cfg)
	}
	if cfg.BaseURL != "" || cfg.OIDC != nil {
		t.Errorf("expected no access settings, got: %+v", cfg)
	}
}

func TestLoad_AccessSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `baseUrl: https://invoices.example.com
oidc:
  issuer: https://auth.example.com
  clientId: invoicer
  clientSecretFile: /run/secrets/oidc-client-secret
  cookieKeyFile: /run/secrets/cookie-key
  adminGroup: admins
  customerGroup: customers
  groupsClaim: roles
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BaseURL != "https://invoices.example.com" {
		t.Errorf("BaseURL: got %q", cfg.BaseURL)
	}
	if cfg.OIDC == nil {
		t.Fatal("OIDC settings are nil")
	}
	if cfg.OIDC.Issuer != "https://auth.example.com" {
		t.Errorf("Issuer: got %q", cfg.OIDC.Issuer)
	}
	if cfg.OIDC.ClientID != "invoicer" {
		t.Errorf("ClientID: got %q", cfg.OIDC.ClientID)
	}
	if cfg.OIDC.ClientSecretFile != "/run/secrets/oidc-client-secret" {
		t.Errorf("ClientSecretFile: got %q", cfg.OIDC.ClientSecretFile)
	}
	if cfg.OIDC.CookieKeyFile != "/run/secrets/cookie-key" {
		t.Errorf("CookieKeyFile: got %q", cfg.OIDC.CookieKeyFile)
	}
	if cfg.OIDC.AdminGroup != "admins" {
		t.Errorf("AdminGroup: got %q", cfg.OIDC.AdminGroup)
	}
	if cfg.OIDC.CustomerGroup != "customers" {
		t.Errorf("CustomerGroup: got %q", cfg.OIDC.CustomerGroup)
	}
	if cfg.OIDC.GroupsClaim != "roles" {
		t.Errorf("GroupsClaim: got %q", cfg.OIDC.GroupsClaim)
	}
}

func TestSave_PreservesAccessSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `vendor: Acme Corp
baseUrl: https://invoices.example.com
oidc:
  issuer: https://auth.example.com
  clientId: invoicer
  clientSecretFile: /run/secrets/oidc-client-secret
  cookieKeyFile: /run/secrets/cookie-key
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	if err := config.Save(path, &config.Config{Rate: 200}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Rate != 200 {
		t.Errorf("Rate: got %v, want 200", cfg.Rate)
	}
	if cfg.BaseURL != "https://invoices.example.com" {
		t.Errorf("BaseURL was not preserved: got %q", cfg.BaseURL)
	}
	if cfg.OIDC == nil || cfg.OIDC.Issuer != "https://auth.example.com" {
		t.Errorf("OIDC settings were not preserved: %+v", cfg.OIDC)
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
model: deepseek/deepseek-flash
project: Website Redesign
database: /var/lib/invoicer/invoices.db
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
	if cfg.Project != "Website Redesign" {
		t.Errorf("Project: got %q, want %q", cfg.Project, "Website Redesign")
	}
	if cfg.Database != "/var/lib/invoicer/invoices.db" {
		t.Errorf("Database: got %q, want %q", cfg.Database, "/var/lib/invoicer/invoices.db")
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

func TestSave_SavesProject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := config.Save(path, &config.Config{Project: "Mobile App"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Project != "Mobile App" {
		t.Errorf("Project: got %q, want %q", cfg.Project, "Mobile App")
	}
}

func TestSave_SavesDatabase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := config.Save(path, &config.Config{Database: "/var/lib/invoicer/invoices.db"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Database != "/var/lib/invoicer/invoices.db" {
		t.Errorf("Database: got %q, want %q", cfg.Database, "/var/lib/invoicer/invoices.db")
	}
}

func TestDefaultDatabasePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}

	got, err := config.DefaultDatabasePath()
	if err != nil {
		t.Fatalf("DefaultDatabasePath: %v", err)
	}
	want := filepath.Join(home, ".invoicer", "invoices.db")
	if got != want {
		t.Errorf("DefaultDatabasePath: got %q, want %q", got, want)
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
		Database: "/var/lib/invoicer/initial.db",
	}
	if err := config.Save(path, initial); err != nil {
		t.Fatalf("initial Save: %v", err)
	}

	// Update only the project.
	if err := config.Save(path, &config.Config{Project: "Mobile App"}); err != nil {
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
	if cfg.Database != "/var/lib/invoicer/initial.db" {
		t.Errorf("Database should be unchanged: got %q", cfg.Database)
	}
	if cfg.Project != "Mobile App" {
		t.Errorf("Project should be updated: got %q", cfg.Project)
	}
}

func TestSave_DropsRemovedValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `vendor: V1
pdf: true
model: deepseek/deepseek-flash
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	if err := config.Save(path, &config.Config{Rate: 100}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got := string(data)
	if strings.Contains(got, "pdf:") {
		t.Errorf("saved config still has pdf: %q", got)
	}
	if strings.Contains(got, "model:") {
		t.Errorf("saved config still has model: %q", got)
	}
}
