package store_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zon/invoicer/internal/invoice"
	"github.com/zon/invoicer/internal/store"
)

func testInvoice() *invoice.Invoice {
	return &invoice.Invoice{
		Month:    time.January,
		Year:     2025,
		Vendor:   "Jane Contractor",
		Customer: "Acme Corp",
		Project:  "Website Redesign",
		Rate:     150,
		Weeks: []invoice.Week{
			{
				Start: time.Date(2024, time.December, 30, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2025, time.January, 5, 0, 0, 0, 0, time.UTC),
				Hours: 32,
			},
			{
				Start: time.Date(2025, time.January, 6, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2025, time.January, 12, 0, 0, 0, 0, time.UTC),
				Hours: 40,
			},
		},
	}
}

func openTempStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "invoices.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return s, path
}

func TestOpenCreatesDatabaseAndParentDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "data", "invoices.db")

	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected database file to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Errorf("expected database directory to exist: %v", err)
	}
}

func TestSaveStoresInvoiceAndWeeks(t *testing.T) {
	s, _ := openTempStore(t)
	inv := testInvoice()

	id, err := s.Save(inv)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if id == "" {
		t.Fatal("expected a non-empty invoice identifier")
	}

	got, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got.Month != inv.Month {
		t.Errorf("Month: got %v, want %v", got.Month, inv.Month)
	}
	if got.Year != inv.Year {
		t.Errorf("Year: got %d, want %d", got.Year, inv.Year)
	}
	if got.Vendor != inv.Vendor {
		t.Errorf("Vendor: got %q, want %q", got.Vendor, inv.Vendor)
	}
	if got.Customer != inv.Customer {
		t.Errorf("Customer: got %q, want %q", got.Customer, inv.Customer)
	}
	if got.Project != inv.Project {
		t.Errorf("Project: got %q, want %q", got.Project, inv.Project)
	}
	if got.Rate != inv.Rate {
		t.Errorf("Rate: got %v, want %v", got.Rate, inv.Rate)
	}
	if len(got.Weeks) != len(inv.Weeks) {
		t.Fatalf("Weeks: got %d, want %d", len(got.Weeks), len(inv.Weeks))
	}
	for i := range inv.Weeks {
		if !got.Weeks[i].Start.Equal(inv.Weeks[i].Start) {
			t.Errorf("Week %d Start: got %v, want %v", i, got.Weeks[i].Start, inv.Weeks[i].Start)
		}
		if !got.Weeks[i].End.Equal(inv.Weeks[i].End) {
			t.Errorf("Week %d End: got %v, want %v", i, got.Weeks[i].End, inv.Weeks[i].End)
		}
		if got.Weeks[i].Hours != inv.Weeks[i].Hours {
			t.Errorf("Week %d Hours: got %v, want %v", i, got.Weeks[i].Hours, inv.Weeks[i].Hours)
		}
	}
}

func TestSaveKeepsIdentifierAndReplacesStoredInvoice(t *testing.T) {
	s, _ := openTempStore(t)

	id, err := s.Save(testInvoice())
	if err != nil {
		t.Fatalf("Save original: %v", err)
	}

	updated := testInvoice()
	updated.Vendor = "New Vendor"
	updated.Rate = 200
	updated.Weeks = []invoice.Week{
		{
			Start: time.Date(2025, time.January, 13, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2025, time.January, 19, 0, 0, 0, 0, time.UTC),
			Hours: 12,
		},
	}

	updatedID, err := s.Save(updated)
	if err != nil {
		t.Fatalf("Save updated: %v", err)
	}
	if updatedID != id {
		t.Fatalf("identifier changed: got %q, want %q", updatedID, id)
	}

	got, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Vendor != updated.Vendor {
		t.Errorf("Vendor: got %q, want %q", got.Vendor, updated.Vendor)
	}
	if got.Rate != updated.Rate {
		t.Errorf("Rate: got %v, want %v", got.Rate, updated.Rate)
	}
	if len(got.Weeks) != len(updated.Weeks) {
		t.Fatalf("Weeks: got %d, want %d", len(got.Weeks), len(updated.Weeks))
	}
	if !got.Weeks[0].Start.Equal(updated.Weeks[0].Start) {
		t.Errorf("Week Start: got %v, want %v", got.Weeks[0].Start, updated.Weeks[0].Start)
	}
	if !got.Weeks[0].End.Equal(updated.Weeks[0].End) {
		t.Errorf("Week End: got %v, want %v", got.Weeks[0].End, updated.Weeks[0].End)
	}
	if got.Weeks[0].Hours != updated.Weeks[0].Hours {
		t.Errorf("Week Hours: got %v, want %v", got.Weeks[0].Hours, updated.Weeks[0].Hours)
	}
}

func TestSavePersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invoices.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	id, err := s.Save(testInvoice())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := store.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()

	got, err := reopened.Get(id)
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if len(got.Weeks) != 2 {
		t.Errorf("Weeks after reopen: got %d, want 2", len(got.Weeks))
	}
}

func TestSaveStoresSeparateInvoices(t *testing.T) {
	s, _ := openTempStore(t)

	first := testInvoice()
	second := testInvoice()
	second.Project = "Mobile App"
	second.Weeks = second.Weeks[:1]

	firstID, err := s.Save(first)
	if err != nil {
		t.Fatalf("Save first: %v", err)
	}
	secondID, err := s.Save(second)
	if err != nil {
		t.Fatalf("Save second: %v", err)
	}
	if firstID == secondID {
		t.Fatal("expected separate invoices to have different identifiers")
	}

	gotFirst, err := s.Get(firstID)
	if err != nil {
		t.Fatalf("Get first: %v", err)
	}
	gotSecond, err := s.Get(secondID)
	if err != nil {
		t.Fatalf("Get second: %v", err)
	}
	if gotFirst.Project != first.Project {
		t.Errorf("first Project: got %q, want %q", gotFirst.Project, first.Project)
	}
	if len(gotFirst.Weeks) != 2 {
		t.Errorf("first Weeks: got %d, want 2", len(gotFirst.Weeks))
	}
	if gotSecond.Project != second.Project {
		t.Errorf("second Project: got %q, want %q", gotSecond.Project, second.Project)
	}
	if len(gotSecond.Weeks) != 1 {
		t.Errorf("second Weeks: got %d, want 1", len(gotSecond.Weeks))
	}
}
