package cli

import (
	"bytes"
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

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open store: %v", err)
	}
	defer s.Close()

	got, err := s.Get(id)
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

	got, err := s.Get(firstID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Rate != second.Rate {
		t.Errorf("Rate: got %v, want %v", got.Rate, second.Rate)
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
