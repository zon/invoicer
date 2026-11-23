package cli

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	"github.com/zon/invoicer/internal/invoice"
	"github.com/zon/invoicer/internal/store"
)

func TestCLIParsesRemoveCommand(t *testing.T) {
	var cmd CLI
	p, err := kong.New(&cmd, kong.Name("invoicer"), kong.Exit(func(int) {}))
	if err != nil {
		t.Fatalf("kong.New failed: %v", err)
	}
	if _, err := p.Parse([]string{"remove", "3F9K2M8Q1X", "--database", "/tmp/remove.db"}); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if cmd.Remove.ID != "3F9K2M8Q1X" {
		t.Errorf("Remove.ID: got %q, want %q", cmd.Remove.ID, "3F9K2M8Q1X")
	}
	if cmd.Remove.Database != "/tmp/remove.db" {
		t.Errorf("Remove.Database: got %q, want %q", cmd.Remove.Database, "/tmp/remove.db")
	}
}

func TestRemoveRunDeletesInvoice(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "invoices.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	id, _, err := s.Save(&invoice.Invoice{
		Month:    time.October,
		Year:     2026,
		Vendor:   "Jane Smith",
		Customer: "Acme Corp",
		Rate:     100,
		Weeks: []invoice.Week{
			{
				Start: time.Date(2026, time.September, 28, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2026, time.October, 4, 0, 0, 0, 0, time.UTC),
				Hours: 20,
			},
		},
	})
	if err != nil {
		t.Fatalf("saving invoice: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("closing store: %v", err)
	}

	configPath := writeTestConfig(t, "database: "+dbPath+"\n")

	var out bytes.Buffer
	if err := (&RemoveCmd{ID: strings.ToUpper(id)}).run(configPath, &out); err != nil {
		t.Fatalf("run: %v", err)
	}

	want := "Removed invoice for October 2026: " + strings.ToUpper(id) + "\n"
	if got := out.String(); got != want {
		t.Errorf("output: got %q, want %q", got, want)
	}

	reopened, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("reopening store: %v", err)
	}
	defer reopened.Close()

	invoices, err := reopened.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(invoices) != 0 {
		t.Errorf("Invoices: got %d, want 0", len(invoices))
	}
}

func TestRemoveRunReportsMissingInvoice(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "invoices.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	keptID, _, err := s.Save(&invoice.Invoice{
		Month:    time.October,
		Year:     2026,
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

	configPath := writeTestConfig(t, "database: "+dbPath+"\n")

	var out bytes.Buffer
	err = (&RemoveCmd{ID: "missing"}).run(configPath, &out)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("run error: got %v, want %v", err, store.ErrNotFound)
	}
	if out.Len() != 0 {
		t.Errorf("output: got %q, want no output", out.String())
	}

	reopened, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("reopening store: %v", err)
	}
	defer reopened.Close()

	if _, err := reopened.Get(keptID); err != nil {
		t.Errorf("expected the stored invoice to stay: %v", err)
	}
}
