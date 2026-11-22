package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/zon/invoicer/internal/config"
)

// TestCLIParsesWithoutPanic verifies that the CLI struct can be parsed by kong
// without duplicate flag errors. This guards against regressions where sibling
// subcommands happen to share flag names.
func TestCLIParsesWithoutPanic(t *testing.T) {
	var cmd CLI
	_, err := kong.New(&cmd,
		kong.Name("invoicer"),
		kong.Description("Generate invoices for an hourly contractor."),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New panicked or failed: %v", err)
	}
}

// TestCLIGenerateIsDefault verifies the generate subcommand runs by default
// (without explicitly typing 'generate').
func TestCLIGenerateIsDefault(t *testing.T) {
	var cmd CLI
	p, err := kong.New(&cmd,
		kong.Name("invoicer"),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New failed: %v", err)
	}
	ctx, err := p.Parse([]string{"january"})
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if !strings.Contains(ctx.Command(), "generate") {
		t.Errorf("expected default command to be 'generate', got %q", ctx.Command())
	}
	if cmd.Generate.Month != "january" {
		t.Errorf("Month: got %q, want %q", cmd.Generate.Month, "january")
	}
}

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
	c := &GenerateCmd{
		Vendor:   "My Vendor",
		Customer: "My Customer",
		Project:  "My Project",
		Rate:     100,
		Hours:    40,
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
	if opts.Project != "My Project" {
		t.Errorf("Project: got %q, want %q", opts.Project, "My Project")
	}
	if opts.Rate != 100 {
		t.Errorf("Rate: got %v, want 100", opts.Rate)
	}
	if opts.Hours != 40 {
		t.Errorf("Hours: got %v, want 40", opts.Hours)
	}
}

func TestResolveOptions_CLIOverridesConfig(t *testing.T) {
	path := writeTestConfig(t, `vendor: Config Vendor
customer: Config Customer
rate: 50
hours: 20
project: Config Project
`)
	c := &GenerateCmd{
		Vendor:   "CLI Vendor",
		Customer: "CLI Customer",
		Project:  "CLI Project",
		Rate:     150,
		Hours:    35,
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
	if opts.Project != "CLI Project" {
		t.Errorf("Project: CLI should override config; got %q", opts.Project)
	}
	if opts.Rate != 150 {
		t.Errorf("Rate: CLI should override config; got %v", opts.Rate)
	}
	if opts.Hours != 35 {
		t.Errorf("Hours: CLI should override config; got %v", opts.Hours)
	}
}

func TestResolveOptions_ConfigFallback(t *testing.T) {
	path := writeTestConfig(t, `vendor: Config Vendor
customer: Config Customer
rate: 50
hours: 20
project: Config Project
`)
	// CLI provides no values (zero values).
	c := &GenerateCmd{}
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
	if opts.Project != "Config Project" {
		t.Errorf("Project: expected config fallback; got %q", opts.Project)
	}
	if opts.Rate != 50 {
		t.Errorf("Rate: expected config fallback; got %v", opts.Rate)
	}
	if opts.Hours != 20 {
		t.Errorf("Hours: expected config fallback; got %v", opts.Hours)
	}
}

func TestResolveOptions_IgnoresRemovedConfigValues(t *testing.T) {
	path := writeTestConfig(t, `vendor: Config Vendor
customer: Config Customer
rate: 50
hours: 20
pdf: true
model: deepseek/deepseek-flash
`)
	opts, err := (&GenerateCmd{}).resolveOptions(path)
	if err != nil {
		t.Fatalf("resolveOptions: %v", err)
	}
	if opts.Vendor != "Config Vendor" || opts.Customer != "Config Customer" {
		t.Errorf("expected vendor and customer from config, got %q and %q", opts.Vendor, opts.Customer)
	}
	if opts.Rate != 50 || opts.Hours != 20 {
		t.Errorf("expected rate and hours from config, got %v and %v", opts.Rate, opts.Hours)
	}
}

func TestResolveOptions_PartialCLIOverride(t *testing.T) {
	path := writeTestConfig(t, `vendor: Config Vendor
customer: Config Customer
rate: 50
hours: 20
`)
	// CLI only provides rate and vendor.
	c := &GenerateCmd{
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

func TestResolveOptions_DatabaseFlagOverridesConfig(t *testing.T) {
	path := writeTestConfig(t, "database: /var/lib/invoicer/config.db\n")
	c := &GenerateCmd{Database: "/tmp/flag.db"}
	opts, err := c.resolveOptions(path)
	if err != nil {
		t.Fatalf("resolveOptions: %v", err)
	}
	if opts.Database != "/tmp/flag.db" {
		t.Errorf("Database: got %q, want CLI value", opts.Database)
	}
}

func TestResolveOptions_DatabaseConfigFallback(t *testing.T) {
	path := writeTestConfig(t, "database: /var/lib/invoicer/config.db\n")
	opts, err := (&GenerateCmd{}).resolveOptions(path)
	if err != nil {
		t.Fatalf("resolveOptions: %v", err)
	}
	if opts.Database != "/var/lib/invoicer/config.db" {
		t.Errorf("Database: got %q, want config value", opts.Database)
	}
}

func TestResolveOptions_DefaultDatabase(t *testing.T) {
	want, err := config.DefaultDatabasePath()
	if err != nil {
		t.Fatalf("DefaultDatabasePath: %v", err)
	}
	path := filepath.Join(t.TempDir(), "nonexistent.yaml")
	opts, err := (&GenerateCmd{}).resolveOptions(path)
	if err != nil {
		t.Fatalf("resolveOptions: %v", err)
	}
	if opts.Database != want {
		t.Errorf("Database: got %q, want default %q", opts.Database, want)
	}
}

func TestCLIParsesDatabaseFlag(t *testing.T) {
	var cmd CLI
	p, err := kong.New(&cmd, kong.Name("invoicer"), kong.Exit(func(int) {}))
	if err != nil {
		t.Fatalf("kong.New failed: %v", err)
	}
	ctx, err := p.Parse([]string{"--database", "/tmp/invoices.db"})
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if !strings.Contains(ctx.Command(), "generate") {
		t.Errorf("expected default command to be 'generate', got %q", ctx.Command())
	}
	if cmd.Generate.Database != "/tmp/invoices.db" {
		t.Errorf("Generate.Database: got %q, want %q", cmd.Generate.Database, "/tmp/invoices.db")
	}
}

func TestCLIRejectsRemovedGenerateFlags(t *testing.T) {
	for _, args := range [][]string{
		{"--pdf"},
		{"-p"},
		{"--model", "deepseek/deepseek-flash"},
		{"-m", "deepseek/deepseek-flash"},
	} {
		var cmd CLI
		p, err := kong.New(&cmd, kong.Name("invoicer"), kong.Exit(func(int) {}))
		if err != nil {
			t.Fatalf("kong.New failed: %v", err)
		}
		if _, err := p.Parse(args); err == nil {
			t.Errorf("expected generate to reject %v", args)
		}
	}
}

func TestCLIRejectsRemovedSetConfigFlags(t *testing.T) {
	for _, args := range [][]string{
		{"set", "config", "--pdf"},
		{"set", "config", "--model", "deepseek/deepseek-flash"},
	} {
		var cmd CLI
		p, err := kong.New(&cmd, kong.Name("invoicer"), kong.Exit(func(int) {}))
		if err != nil {
			t.Fatalf("kong.New failed: %v", err)
		}
		if _, err := p.Parse(args); err == nil {
			t.Errorf("expected set config to reject %v", args)
		}
	}
}

func TestCLIParsesSetConfigDatabaseFlag(t *testing.T) {
	var cmd CLI
	p, err := kong.New(&cmd, kong.Name("invoicer"), kong.Exit(func(int) {}))
	if err != nil {
		t.Fatalf("kong.New failed: %v", err)
	}
	if _, err := p.Parse([]string{"set", "config", "--database", "/tmp/invoices.db"}); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if cmd.Set.Config.Database != "/tmp/invoices.db" {
		t.Errorf("Set.Config.Database: got %q, want %q", cmd.Set.Config.Database, "/tmp/invoices.db")
	}
}
