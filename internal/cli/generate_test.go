package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zon/invoicer/internal/invoice"
	"github.com/zon/invoicer/internal/store"
)

func outputID(t *testing.T, out string) string {
	t.Helper()
	_, id, ok := strings.Cut(strings.TrimSpace(out), ": ")
	if !ok || id == "" {
		t.Fatalf("expected an invoice identifier in output %q", out)
	}
	return id
}

func TestGenerateSavesInvoiceAndPrintsIdentifier(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	dbPath := filepath.Join(dir, "data", "invoices.db")

	cmd := &GenerateCmd{
		Month:    "january",
		Year:     2025,
		Vendor:   "Jane Contractor",
		Customer: "Acme Corp",
		Project:  "Website Redesign",
		Rate:     150,
		Hours:    40,
		Database: dbPath,
	}

	var out bytes.Buffer
	if err := cmd.run(filepath.Join(dir, "config.yaml"), &out); err != nil {
		t.Fatalf("run: %v", err)
	}

	if !strings.HasPrefix(out.String(), "Created invoice for January 2025: ") {
		t.Errorf("output: got %q, want created invoice with month, year, and identifier", out.String())
	}
	id := outputID(t, out.String())
	if id != strings.ToUpper(id) {
		t.Errorf("identifier is not uppercase: %q", id)
	}

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open store: %v", err)
	}
	defer s.Close()

	got, err := s.Get(strings.ToLower(id))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Vendor != cmd.Vendor {
		t.Errorf("Vendor: got %q, want %q", got.Vendor, cmd.Vendor)
	}
	if got.Customer != cmd.Customer {
		t.Errorf("Customer: got %q, want %q", got.Customer, cmd.Customer)
	}
	if got.Project != cmd.Project {
		t.Errorf("Project: got %q, want %q", got.Project, cmd.Project)
	}
	if got.Rate != cmd.Rate {
		t.Errorf("Rate: got %v, want %v", got.Rate, cmd.Rate)
	}
	if got.Month != time.January || got.Year != 2025 {
		t.Errorf("Month/Year: got %v %d, want January 2025", got.Month, got.Year)
	}
	if len(got.Weeks) != len(invoice.WeeksForMonth(2025, time.January, cmd.Hours)) {
		t.Errorf("Weeks: got %d, want %d", len(got.Weeks), len(invoice.WeeksForMonth(2025, time.January, cmd.Hours)))
	}
}

func TestGenerateWritesNoInvoiceFiles(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	cmd := &GenerateCmd{
		Month:    "january",
		Year:     2025,
		Vendor:   "Jane Contractor",
		Customer: "Acme Corp",
		Rate:     150,
		Hours:    40,
		Database: filepath.Join(dir, "data", "invoices.db"),
	}

	var out bytes.Buffer
	if err := cmd.run(filepath.Join(dir, "config.yaml"), &out); err != nil {
		t.Fatalf("run: %v", err)
	}

	for _, path := range []string{"invoices", "invoice-acme-corp-2025-01.html", "invoice-acme-corp-2025-01.pdf"} {
		if _, err := os.Stat(filepath.Join(dir, path)); !os.IsNotExist(err) {
			t.Errorf("expected %q not to exist, got err %v", path, err)
		}
	}
	if strings.Contains(out.String(), ".html") || strings.Contains(out.String(), ".pdf") {
		t.Errorf("output reports a file path: %q", out.String())
	}
}

func TestGenerateReusesIdentifierForSameInvoice(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "invoices.db")
	configPath := filepath.Join(dir, "config.yaml")

	first := &GenerateCmd{
		Month:    "january",
		Year:     2025,
		Vendor:   "Jane Contractor",
		Customer: "Acme Corp",
		Rate:     150,
		Hours:    40,
		Database: dbPath,
	}
	var firstOut bytes.Buffer
	if err := first.run(configPath, &firstOut); err != nil {
		t.Fatalf("first run: %v", err)
	}

	second := *first
	second.Rate = 200
	var secondOut bytes.Buffer
	if err := second.run(configPath, &secondOut); err != nil {
		t.Fatalf("second run: %v", err)
	}

	if !strings.HasPrefix(firstOut.String(), "Created invoice for January 2025: ") {
		t.Errorf("first output: got %q, want a created invoice", firstOut.String())
	}
	if !strings.HasPrefix(secondOut.String(), "Edited invoice for January 2025: ") {
		t.Errorf("second output: got %q, want an edited invoice", secondOut.String())
	}
	firstID := outputID(t, firstOut.String())
	secondID := outputID(t, secondOut.String())
	if firstID != secondID {
		t.Fatalf("identifier changed: got %q, want %q", secondID, firstID)
	}

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open store: %v", err)
	}
	defer s.Close()

	got, err := s.Get(strings.ToLower(firstID))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Rate != second.Rate {
		t.Errorf("Rate: got %v, want %v", got.Rate, second.Rate)
	}
}

func TestGenerateRefusesPaidInvoice(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "invoices.db")

	original := &invoice.Invoice{
		Month:    time.January,
		Year:     2025,
		Vendor:   "Jane Contractor",
		Customer: "Acme Corp",
		Project:  "Website Redesign",
		Rate:     150,
		Weeks:    invoice.WeeksForMonth(2025, time.January, 40),
	}

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	id, _, err := s.Save(original)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := s.SetPaid(2025, time.January, true); err != nil {
		t.Fatalf("SetPaid: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("closing store: %v", err)
	}

	cmd := &GenerateCmd{
		Month:    "january",
		Year:     2025,
		Vendor:   "New Vendor",
		Customer: "Globex",
		Project:  "Mobile App",
		Rate:     200,
		Hours:    10,
		Database: dbPath,
	}

	var out bytes.Buffer
	err = cmd.run(filepath.Join(dir, "config.yaml"), &out)
	if !errors.Is(err, store.ErrPaid) {
		t.Fatalf("run error: got %v, want %v", err, store.ErrPaid)
	}
	if !strings.Contains(err.Error(), "paid") {
		t.Errorf("run error does not report that the invoice is paid: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output, got %q", out.String())
	}

	s, err = store.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer s.Close()

	got, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.Paid {
		t.Error("expected the invoice to stay paid")
	}
	if got.Vendor != original.Vendor {
		t.Errorf("Vendor: got %q, want %q", got.Vendor, original.Vendor)
	}
	if got.Customer != original.Customer {
		t.Errorf("Customer: got %q, want %q", got.Customer, original.Customer)
	}
	if got.Project != original.Project {
		t.Errorf("Project: got %q, want %q", got.Project, original.Project)
	}
	if got.Rate != original.Rate {
		t.Errorf("Rate: got %v, want %v", got.Rate, original.Rate)
	}
	if len(got.Weeks) != len(original.Weeks) {
		t.Fatalf("Weeks: got %d, want %d", len(got.Weeks), len(original.Weeks))
	}
	if got.Total() != original.Total() {
		t.Errorf("Total: got %v, want %v", got.Total(), original.Total())
	}
}

func TestGenerateInvalidInputStoresNothing(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "invoices.db")

	cmd := &GenerateCmd{
		Month:    "notamonth",
		Vendor:   "Jane Contractor",
		Customer: "Acme Corp",
		Rate:     150,
		Hours:    40,
		Database: dbPath,
	}

	var out bytes.Buffer
	if err := cmd.run(filepath.Join(dir, "config.yaml"), &out); err == nil {
		t.Fatal("expected an error for an invalid month")
	}
	if out.Len() != 0 {
		t.Errorf("expected no output, got %q", out.String())
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Errorf("expected no database, got err %v", err)
	}
}

func TestGenerateMissingRequiredValueStoresNothing(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "invoices.db")

	cmd := &GenerateCmd{
		Month:    "january",
		Year:     2025,
		Customer: "Acme Corp",
		Rate:     150,
		Hours:    40,
		Database: dbPath,
	}

	var out bytes.Buffer
	if err := cmd.run(filepath.Join(dir, "config.yaml"), &out); err == nil {
		t.Fatal("expected an error for a missing vendor")
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Errorf("expected no database, got err %v", err)
	}
}

func TestGenerateStoresContactEmailFromFlag(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "invoices.db")
	configPath := writeTestConfig(t, "customerEmail: config@acme.com\n")

	cmd := &GenerateCmd{
		Month:         "january",
		Year:          2025,
		Vendor:        "Jane Contractor",
		Customer:      "Acme Corp",
		CustomerEmail: "flag@acme.com",
		Rate:          150,
		Hours:         40,
		Database:      dbPath,
	}

	var out bytes.Buffer
	if err := cmd.run(configPath, &out); err != nil {
		t.Fatalf("run: %v", err)
	}

	got := loadInvoice(t, dbPath, outputID(t, out.String()))
	if got.ContactEmail != "flag@acme.com" {
		t.Errorf("ContactEmail: got %q, want the flag value", got.ContactEmail)
	}
}

func TestGenerateStoresContactEmailFromConfig(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "invoices.db")
	configPath := writeTestConfig(t, "customerEmail: config@acme.com\n")

	cmd := &GenerateCmd{
		Month:    "january",
		Year:     2025,
		Vendor:   "Jane Contractor",
		Customer: "Acme Corp",
		Rate:     150,
		Hours:    40,
		Database: dbPath,
	}

	var out bytes.Buffer
	if err := cmd.run(configPath, &out); err != nil {
		t.Fatalf("run: %v", err)
	}

	got := loadInvoice(t, dbPath, outputID(t, out.String()))
	if got.ContactEmail != "config@acme.com" {
		t.Errorf("ContactEmail: got %q, want the config value", got.ContactEmail)
	}
}

func TestGenerateEditStoresNewContactEmail(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "invoices.db")
	configPath := filepath.Join(dir, "config.yaml")

	first := &GenerateCmd{
		Month:         "january",
		Year:          2025,
		Vendor:        "Jane Contractor",
		Customer:      "Acme Corp",
		CustomerEmail: "first@acme.com",
		Rate:          150,
		Hours:         40,
		Database:      dbPath,
	}
	var firstOut bytes.Buffer
	if err := first.run(configPath, &firstOut); err != nil {
		t.Fatalf("first run: %v", err)
	}

	second := *first
	second.CustomerEmail = "second@acme.com"
	var secondOut bytes.Buffer
	if err := second.run(configPath, &secondOut); err != nil {
		t.Fatalf("second run: %v", err)
	}

	if !strings.HasPrefix(secondOut.String(), "Edited invoice for January 2025: ") {
		t.Errorf("second output: got %q, want an edited invoice", secondOut.String())
	}
	id := outputID(t, secondOut.String())
	if id != outputID(t, firstOut.String()) {
		t.Fatalf("identifier changed: got %q, want %q", id, outputID(t, firstOut.String()))
	}
	got := loadInvoice(t, dbPath, id)
	if got.ContactEmail != "second@acme.com" {
		t.Errorf("ContactEmail: got %q, want the new address", got.ContactEmail)
	}
}

func TestGenerateWithoutResendSettingsSavesAndSendsNothing(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "invoices.db")
	configPath := writeTestConfig(t, `customerEmail: billing@acme.com
baseUrl: https://invoices.example.com
oidc:
  issuer: https://auth.example.com
  clientId: invoicer
  clientSecretFile: /run/secrets/oidc-client-secret
  cookieKeyFile: /run/secrets/cookie-key
`)

	cmd := &GenerateCmd{
		Month:    "january",
		Year:     2025,
		Vendor:   "Jane Contractor",
		Customer: "Acme Corp",
		Rate:     150,
		Hours:    40,
		Database: dbPath,
	}

	var out bytes.Buffer
	if err := cmd.run(configPath, &out); err != nil {
		t.Fatalf("run: %v", err)
	}

	id := outputID(t, out.String())
	want := "Created invoice for January 2025: " + id + "\n"
	if out.String() != want {
		t.Errorf("output: got %q, want %q", out.String(), want)
	}

	got := loadInvoice(t, dbPath, id)
	if got.ContactEmail != "billing@acme.com" {
		t.Errorf("ContactEmail: got %q, want the config value", got.ContactEmail)
	}
	if !got.EmailSentAt.IsZero() {
		t.Errorf("EmailSentAt: got %v, want no send time", got.EmailSentAt)
	}
}

func TestGenerateWithEmptyResendBlockSavesAndSendsNothing(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "invoices.db")
	configPath := writeTestConfig(t, "resend: {}\n")

	cmd := &GenerateCmd{
		Month:    "january",
		Year:     2025,
		Vendor:   "Jane Contractor",
		Customer: "Acme Corp",
		Rate:     150,
		Hours:    40,
		Database: dbPath,
	}

	var out bytes.Buffer
	if err := cmd.run(configPath, &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.HasPrefix(out.String(), "Created invoice for January 2025: ") {
		t.Errorf("output: got %q, want a created invoice", out.String())
	}
}

func TestGenerateWithIncompleteResendSettingsStoresNothing(t *testing.T) {
	cases := []struct {
		name   string
		config string
		want   string
	}{
		{
			name: "missing contact email",
			config: `baseUrl: https://invoices.example.com
resend:
  apiKeyFile: /run/secrets/resend-api-key
  from: invoicer@example.com
`,
			want: "customerEmail",
		},
		{
			name: "missing base URL",
			config: `customerEmail: billing@acme.com
resend:
  apiKeyFile: /run/secrets/resend-api-key
  from: invoicer@example.com
`,
			want: "baseUrl",
		},
		{
			name: "missing API key file",
			config: `customerEmail: billing@acme.com
baseUrl: https://invoices.example.com
resend:
  from: invoicer@example.com
`,
			want: "resend.apiKeyFile",
		},
		{
			name: "missing sender",
			config: `customerEmail: billing@acme.com
baseUrl: https://invoices.example.com
resend:
  apiKeyFile: /run/secrets/resend-api-key
`,
			want: "resend.from",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			dbPath := filepath.Join(dir, "invoices.db")
			configPath := writeTestConfig(t, tc.config)

			cmd := &GenerateCmd{
				Month:    "january",
				Year:     2025,
				Vendor:   "Jane Contractor",
				Customer: "Acme Corp",
				Rate:     150,
				Hours:    40,
				Database: dbPath,
			}

			var out bytes.Buffer
			err := cmd.run(configPath, &out)
			if err == nil {
				t.Fatal("expected an error for the incomplete email settings")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error: got %q, want it to contain %q", err, tc.want)
			}
			if out.Len() != 0 {
				t.Errorf("expected no output, got %q", out.String())
			}
			if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
				t.Errorf("expected no database, got err %v", err)
			}
		})
	}
}

// loadInvoice opens the store at dbPath and loads the invoice with id.
func loadInvoice(t *testing.T, dbPath, id string) *invoice.Invoice {
	t.Helper()
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open store: %v", err)
	}
	defer s.Close()

	got, err := s.Get(strings.ToLower(id))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	return got
}
