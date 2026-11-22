package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	"github.com/zon/invoicer/internal/config"
	"github.com/zon/invoicer/internal/invoice"
	"github.com/zon/invoicer/internal/store"
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

// writeAccessFiles creates the client secret and cookie key files that a web
// access configuration names.
func writeAccessFiles(t *testing.T) (secretPath, keyPath string) {
	t.Helper()
	dir := t.TempDir()
	secretPath = filepath.Join(dir, "client-secret")
	if err := os.WriteFile(secretPath, []byte("client-secret"), 0o600); err != nil {
		t.Fatalf("writing client secret: %v", err)
	}
	keyPath = filepath.Join(dir, "cookie-key")
	if err := os.WriteFile(keyPath, bytes.Repeat([]byte("k"), 32), 0o600); err != nil {
		t.Fatalf("writing cookie key: %v", err)
	}
	return secretPath, keyPath
}

// accessYAML returns a valid web access configuration that names the given
// secret files.
func accessYAML(secretPath, keyPath string) string {
	return fmt.Sprintf(`baseUrl: https://invoices.example.com
oidc:
  issuer: https://auth.example.com
  clientId: invoicer
  clientSecretFile: %s
  cookieKeyFile: %s
`, secretPath, keyPath)
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

func TestCLIParsesWebCommand(t *testing.T) {
	var cmd CLI
	p, err := kong.New(&cmd, kong.Name("invoicer"), kong.Exit(func(int) {}))
	if err != nil {
		t.Fatalf("kong.New failed: %v", err)
	}
	ctx, err := p.Parse([]string{"web", "--database", "/tmp/web.db"})
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if !strings.Contains(ctx.Command(), "web") {
		t.Errorf("expected command to be 'web', got %q", ctx.Command())
	}
	if cmd.Web.Database != "/tmp/web.db" {
		t.Errorf("Web.Database: got %q, want %q", cmd.Web.Database, "/tmp/web.db")
	}
	if cmd.Web.Address != ":8080" {
		t.Errorf("Web.Address: got %q, want default %q", cmd.Web.Address, ":8080")
	}
}

func TestCLIParsesWebAddress(t *testing.T) {
	var cmd CLI
	p, err := kong.New(&cmd, kong.Name("invoicer"), kong.Exit(func(int) {}))
	if err != nil {
		t.Fatalf("kong.New failed: %v", err)
	}
	if _, err := p.Parse([]string{"web", "--address", "127.0.0.1:9000"}); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if cmd.Web.Address != "127.0.0.1:9000" {
		t.Errorf("Web.Address: got %q, want %q", cmd.Web.Address, "127.0.0.1:9000")
	}
}

func TestWebResolveOptions_Defaults(t *testing.T) {
	want, err := config.DefaultDatabasePath()
	if err != nil {
		t.Fatalf("DefaultDatabasePath: %v", err)
	}
	secretPath, keyPath := writeAccessFiles(t)
	path := writeTestConfig(t, accessYAML(secretPath, keyPath))
	opts, err := (&WebCmd{}).resolveOptions(path)
	if err != nil {
		t.Fatalf("resolveOptions: %v", err)
	}
	if opts.Address != ":8080" {
		t.Errorf("Address: got %q, want %q", opts.Address, ":8080")
	}
	if opts.Database != want {
		t.Errorf("Database: got %q, want default %q", opts.Database, want)
	}
	if opts.Access.BaseURL != "https://invoices.example.com" {
		t.Errorf("BaseURL: got %q", opts.Access.BaseURL)
	}
	if opts.Access.Issuer != "https://auth.example.com" {
		t.Errorf("Issuer: got %q", opts.Access.Issuer)
	}
	if opts.Access.ClientID != "invoicer" {
		t.Errorf("ClientID: got %q", opts.Access.ClientID)
	}
	if opts.Access.ClientSecret != "client-secret" {
		t.Errorf("ClientSecret: got %q", opts.Access.ClientSecret)
	}
	if len(opts.Access.CookieKey) != 32 {
		t.Errorf("CookieKey length: got %d, want 32", len(opts.Access.CookieKey))
	}
	if opts.Access.AdminGroup != "invoicer-admins" {
		t.Errorf("AdminGroup: got %q, want default", opts.Access.AdminGroup)
	}
	if opts.Access.CustomerGroup != "invoicer-customers" {
		t.Errorf("CustomerGroup: got %q, want default", opts.Access.CustomerGroup)
	}
	if opts.Access.GroupsClaim != "groups" {
		t.Errorf("GroupsClaim: got %q, want default", opts.Access.GroupsClaim)
	}
}

func TestWebResolveOptions_AccessSettings(t *testing.T) {
	secretPath, keyPath := writeAccessFiles(t)
	path := writeTestConfig(t, accessYAML(secretPath, keyPath)+`  adminGroup: admins
  customerGroup: customers
  groupsClaim: roles
`)
	opts, err := (&WebCmd{}).resolveOptions(path)
	if err != nil {
		t.Fatalf("resolveOptions: %v", err)
	}
	if opts.Access.AdminGroup != "admins" {
		t.Errorf("AdminGroup: got %q, want %q", opts.Access.AdminGroup, "admins")
	}
	if opts.Access.CustomerGroup != "customers" {
		t.Errorf("CustomerGroup: got %q, want %q", opts.Access.CustomerGroup, "customers")
	}
	if opts.Access.GroupsClaim != "roles" {
		t.Errorf("GroupsClaim: got %q, want %q", opts.Access.GroupsClaim, "roles")
	}
}

func TestWebResolveOptions_InvalidAccessSettings(t *testing.T) {
	secretPath, keyPath := writeAccessFiles(t)
	valid := accessYAML(secretPath, keyPath)

	emptySecret := filepath.Join(t.TempDir(), "empty-secret")
	if err := os.WriteFile(emptySecret, nil, 0o600); err != nil {
		t.Fatalf("writing empty client secret: %v", err)
	}
	shortKey := filepath.Join(t.TempDir(), "short-key")
	if err := os.WriteFile(shortKey, bytes.Repeat([]byte("k"), 16), 0o600); err != nil {
		t.Fatalf("writing short cookie key: %v", err)
	}

	cases := []struct {
		name   string
		config string
		want   string
	}{
		{
			name:   "missing OIDC settings",
			config: "baseUrl: https://invoices.example.com\n",
			want:   "oidc",
		},
		{
			name:   "missing base URL",
			config: strings.Replace(valid, "baseUrl: https://invoices.example.com\n", "", 1),
			want:   "baseUrl is required",
		},
		{
			name:   "invalid base URL",
			config: strings.Replace(valid, "https://invoices.example.com", "invoices.example.com", 1),
			want:   "baseUrl must be an absolute HTTPS address",
		},
		{
			name:   "plain HTTP base URL",
			config: strings.Replace(valid, "https://invoices.example.com", "http://invoices.example.com", 1),
			want:   "baseUrl must use HTTPS",
		},
		{
			name:   "missing issuer",
			config: strings.Replace(valid, "  issuer: https://auth.example.com\n", "", 1),
			want:   "oidc.issuer is required",
		},
		{
			name:   "invalid issuer",
			config: strings.Replace(valid, "https://auth.example.com", "auth.example.com", 1),
			want:   "oidc.issuer must be an absolute HTTPS address",
		},
		{
			name:   "plain HTTP issuer",
			config: strings.Replace(valid, "https://auth.example.com", "http://auth.example.com", 1),
			want:   "oidc.issuer must use HTTPS",
		},
		{
			name:   "missing client ID",
			config: strings.Replace(valid, "  clientId: invoicer\n", "", 1),
			want:   "oidc.clientId is required",
		},
		{
			name:   "missing client secret file",
			config: strings.Replace(valid, "  clientSecretFile: "+secretPath+"\n", "", 1),
			want:   "oidc.clientSecretFile is required",
		},
		{
			name:   "unreadable client secret file",
			config: strings.Replace(valid, secretPath, filepath.Join(t.TempDir(), "missing"), 1),
			want:   "reading oidc.clientSecretFile",
		},
		{
			name:   "empty client secret file",
			config: strings.Replace(valid, secretPath, emptySecret, 1),
			want:   "oidc.clientSecretFile is empty",
		},
		{
			name:   "missing cookie key file",
			config: strings.Replace(valid, "  cookieKeyFile: "+keyPath+"\n", "", 1),
			want:   "oidc.cookieKeyFile is required",
		},
		{
			name:   "unreadable cookie key file",
			config: strings.Replace(valid, keyPath, filepath.Join(t.TempDir(), "missing"), 1),
			want:   "reading oidc.cookieKeyFile",
		},
		{
			name:   "short cookie key",
			config: strings.Replace(valid, keyPath, shortKey, 1),
			want:   "oidc.cookieKeyFile",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTestConfig(t, tc.config)
			_, err := (&WebCmd{}).resolveOptions(path)
			if err == nil {
				t.Fatal("resolveOptions did not report the invalid access setting")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error: got %q, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestWebResolveOptions_AllowsLoopbackHTTP(t *testing.T) {
	secretPath, keyPath := writeAccessFiles(t)
	content := strings.Replace(accessYAML(secretPath, keyPath), "https://invoices.example.com", "http://localhost:8080", 1)
	content = strings.Replace(content, "https://auth.example.com", "http://127.0.0.1:9000", 1)

	path := writeTestConfig(t, content)
	if _, err := (&WebCmd{}).resolveOptions(path); err != nil {
		t.Fatalf("resolveOptions rejected HTTP loopback addresses: %v", err)
	}
}

func TestWebResolveOptions_DatabaseFlagOverridesConfig(t *testing.T) {
	secretPath, keyPath := writeAccessFiles(t)
	path := writeTestConfig(t, "database: /var/lib/invoicer/config.db\n"+accessYAML(secretPath, keyPath))
	opts, err := (&WebCmd{Database: "/tmp/flag.db"}).resolveOptions(path)
	if err != nil {
		t.Fatalf("resolveOptions: %v", err)
	}
	if opts.Database != "/tmp/flag.db" {
		t.Errorf("Database: got %q, want CLI value", opts.Database)
	}
}

func TestWebResolveOptions_DatabaseConfigFallback(t *testing.T) {
	secretPath, keyPath := writeAccessFiles(t)
	path := writeTestConfig(t, "database: /var/lib/invoicer/config.db\n"+accessYAML(secretPath, keyPath))
	opts, err := (&WebCmd{}).resolveOptions(path)
	if err != nil {
		t.Fatalf("resolveOptions: %v", err)
	}
	if opts.Database != "/var/lib/invoicer/config.db" {
		t.Errorf("Database: got %q, want config value", opts.Database)
	}
}

func TestWebResolveOptions_CustomAddress(t *testing.T) {
	secretPath, keyPath := writeAccessFiles(t)
	path := writeTestConfig(t, accessYAML(secretPath, keyPath))
	opts, err := (&WebCmd{Address: "127.0.0.1:9000"}).resolveOptions(path)
	if err != nil {
		t.Fatalf("resolveOptions: %v", err)
	}
	if opts.Address != "127.0.0.1:9000" {
		t.Errorf("Address: got %q, want %q", opts.Address, "127.0.0.1:9000")
	}
}

func TestWebRunServesInvoicesFromResolvedDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "invoices.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	_, _, err = s.Save(&invoice.Invoice{
		Month:    time.October,
		Year:     2027,
		Vendor:   "Jane Smith",
		Customer: "Acme Corp",
		Rate:     100,
	})
	if err != nil {
		t.Fatalf("saving invoice: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("closing store: %v", err)
	}

	original := listenAndServe
	defer func() { listenAndServe = original }()

	var gotAddress string
	var gotBody string
	listenAndServe = func(server *http.Server) error {
		gotAddress = server.Addr
		recorder := httptest.NewRecorder()
		server.Handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
		gotBody = recorder.Body.String()
		return nil
	}

	secretPath, keyPath := writeAccessFiles(t)
	configPath := writeTestConfig(t, "database: "+dbPath+"\n"+accessYAML(secretPath, keyPath))
	cmd := &WebCmd{Address: "127.0.0.1:9000"}

	if err := cmd.run(configPath, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotAddress != "127.0.0.1:9000" {
		t.Errorf("address: got %q, want %q", gotAddress, "127.0.0.1:9000")
	}
	if !strings.Contains(gotBody, "Acme Corp") {
		t.Error("served page does not list the stored invoice")
	}
}

func TestWebRunReportsReadyURL(t *testing.T) {
	original := listenAndServe
	defer func() { listenAndServe = original }()

	listenAndServe = func(*http.Server) error { return nil }

	cases := []struct {
		address string
		want    string
	}{
		{":8080", "invoicer web ready at http://localhost:8080/\n"},
		{"127.0.0.1:9000", "invoicer web ready at http://127.0.0.1:9000/\n"},
		{"0.0.0.0:9000", "invoicer web ready at http://localhost:9000/\n"},
	}

	for _, tc := range cases {
		t.Run(tc.address, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "invoices.db")
			secretPath, keyPath := writeAccessFiles(t)
			configPath := writeTestConfig(t, "database: "+dbPath+"\n"+accessYAML(secretPath, keyPath))

			var out bytes.Buffer
			if err := (&WebCmd{Address: tc.address}).run(configPath, &out); err != nil {
				t.Fatalf("run: %v", err)
			}
			if got := out.String(); got != tc.want {
				t.Errorf("output: got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWebRunReportsInvalidAccessSettings(t *testing.T) {
	original := listenAndServe
	defer func() { listenAndServe = original }()

	started := false
	listenAndServe = func(*http.Server) error {
		started = true
		return nil
	}

	secretPath, keyPath := writeAccessFiles(t)
	content := strings.Replace(accessYAML(secretPath, keyPath), "  clientId: invoicer\n", "", 1)
	configPath := writeTestConfig(t, content)

	err := (&WebCmd{}).run(configPath, io.Discard)
	if err == nil {
		t.Fatal("run did not report the missing client ID")
	}
	if !strings.Contains(err.Error(), "oidc.clientId") {
		t.Errorf("error does not report the client ID: %v", err)
	}
	if started {
		t.Error("server started with invalid access settings")
	}
}

func TestWebRunStopsOnInterrupt(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "invoices.db")
	secretPath, keyPath := writeAccessFiles(t)
	configPath := writeTestConfig(t, "database: "+dbPath+"\n"+accessYAML(secretPath, keyPath))

	originalListen := listenAndServe
	defer func() { listenAndServe = originalListen }()
	originalNotify := notifyContext
	defer func() { notifyContext = originalNotify }()

	ctx, cancel := context.WithCancel(context.Background())
	notifyContext = func(context.Context, ...os.Signal) (context.Context, context.CancelFunc) {
		return ctx, func() {}
	}

	started := make(chan struct{})
	stopped := make(chan struct{})
	listenAndServe = func(server *http.Server) error {
		server.RegisterOnShutdown(func() { close(stopped) })
		close(started)
		<-stopped
		return http.ErrServerClosed
	}

	done := make(chan error, 1)
	go func() {
		done <- (&WebCmd{Address: "127.0.0.1:9000"}).run(configPath, io.Discard)
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not start")
	}

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop after the interrupt")
	}
}

func TestWebRunReportsDatabaseOpenError(t *testing.T) {
	original := listenAndServe
	defer func() { listenAndServe = original }()

	started := false
	listenAndServe = func(*http.Server) error {
		started = true
		return nil
	}

	notADirectory := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(notADirectory, []byte("file"), 0o600); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	secretPath, keyPath := writeAccessFiles(t)
	configPath := writeTestConfig(t, accessYAML(secretPath, keyPath))
	err := (&WebCmd{Database: filepath.Join(notADirectory, "invoices.db")}).run(configPath, io.Discard)

	if err == nil {
		t.Fatal("run did not report the database error")
	}
	if !strings.Contains(err.Error(), "opening invoice database") {
		t.Errorf("error does not report the database: %v", err)
	}
	if started {
		t.Error("server started without opening the database")
	}
}

func TestWebRunReportsUnavailableAddress(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening on a test address: %v", err)
	}
	defer occupied.Close()

	dbPath := filepath.Join(t.TempDir(), "invoices.db")
	secretPath, keyPath := writeAccessFiles(t)
	configPath := writeTestConfig(t, accessYAML(secretPath, keyPath))

	done := make(chan error, 1)
	go func() {
		done <- (&WebCmd{Address: occupied.Addr().String(), Database: dbPath}).run(configPath, io.Discard)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("run did not report the unavailable address")
		}
		if !strings.Contains(err.Error(), "starting web server") {
			t.Errorf("error does not report the server: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run did not report the unavailable address")
	}
}
