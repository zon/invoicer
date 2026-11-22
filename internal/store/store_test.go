package store_test

import (
	"errors"
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

	id, created, err := s.Save(inv)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if id == "" {
		t.Fatal("expected a non-empty invoice identifier")
	}
	if !created {
		t.Error("expected a new invoice to report created")
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

	id, _, err := s.Save(testInvoice())
	if err != nil {
		t.Fatalf("Save original: %v", err)
	}

	updated := testInvoice()
	updated.Vendor = "New Vendor"
	updated.Customer = "Globex"
	updated.Project = "Mobile App"
	updated.Rate = 200
	updated.Weeks = []invoice.Week{
		{
			Start: time.Date(2025, time.January, 13, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2025, time.January, 19, 0, 0, 0, 0, time.UTC),
			Hours: 12,
		},
	}

	updatedID, created, err := s.Save(updated)
	if err != nil {
		t.Fatalf("Save updated: %v", err)
	}
	if created {
		t.Error("expected an existing invoice to report not created")
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
	if got.Customer != updated.Customer {
		t.Errorf("Customer: got %q, want %q", got.Customer, updated.Customer)
	}
	if got.Project != updated.Project {
		t.Errorf("Project: got %q, want %q", got.Project, updated.Project)
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
	id, _, err := s.Save(testInvoice())
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

func TestSaveReplacesInvoiceWhenCustomerOrProjectDiffers(t *testing.T) {
	changes := map[string]func(*invoice.Invoice){
		"customer": func(inv *invoice.Invoice) { inv.Customer = "Globex" },
		"project":  func(inv *invoice.Invoice) { inv.Project = "Mobile App" },
	}

	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			s, _ := openTempStore(t)

			id, _, err := s.Save(testInvoice())
			if err != nil {
				t.Fatalf("Save original: %v", err)
			}

			updated := testInvoice()
			change(updated)

			updatedID, created, err := s.Save(updated)
			if err != nil {
				t.Fatalf("Save updated: %v", err)
			}
			if created {
				t.Error("expected an existing invoice to report not created")
			}
			if updatedID != id {
				t.Fatalf("identifier changed: got %q, want %q", updatedID, id)
			}

			got, err := s.Get(id)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got.Customer != updated.Customer {
				t.Errorf("Customer: got %q, want %q", got.Customer, updated.Customer)
			}
			if got.Project != updated.Project {
				t.Errorf("Project: got %q, want %q", got.Project, updated.Project)
			}
		})
	}
}

func TestSaveStoresSeparateMonths(t *testing.T) {
	s, _ := openTempStore(t)

	first := testInvoice()
	second := testInvoice()
	second.Month = time.February
	second.Project = "Mobile App"
	second.Weeks = second.Weeks[:1]

	firstID, _, err := s.Save(first)
	if err != nil {
		t.Fatalf("Save first: %v", err)
	}
	secondID, _, err := s.Save(second)
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

func TestListReturnsEveryInvoiceNewestFirstWithWeeks(t *testing.T) {
	s, _ := openTempStore(t)

	older := testInvoice()
	older.Month = time.October
	older.Year = 2027
	older.Project = "Older Project"

	newer := testInvoice()
	newer.Month = time.January
	newer.Year = 2028
	newer.Project = "Newer Project"
	newer.Weeks = newer.Weeks[:1]

	newest := testInvoice()
	newest.Month = time.December
	newest.Year = 2028
	newest.Project = "Newest Project"

	for _, inv := range []*invoice.Invoice{older, newer, newest} {
		if _, _, err := s.Save(inv); err != nil {
			t.Fatalf("Save %q: %v", inv.Project, err)
		}
	}

	got, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	wantProjects := []string{"Newest Project", "Newer Project", "Older Project"}
	wantWeeks := []int{2, 1, 2}
	if len(got) != len(wantProjects) {
		t.Fatalf("Invoices: got %d, want %d", len(got), len(wantProjects))
	}
	for i, project := range wantProjects {
		if got[i].Project != project {
			t.Errorf("Invoice %d Project: got %q, want %q", i, got[i].Project, project)
		}
		if len(got[i].Weeks) != wantWeeks[i] {
			t.Errorf("%s Weeks: got %d, want %d", project, len(got[i].Weeks), wantWeeks[i])
		}
	}
}

func TestListReturnsEmptyWhenNoInvoices(t *testing.T) {
	s, _ := openTempStore(t)

	got, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Invoices: got %d, want 0", len(got))
	}
}

func TestGetByMonthReturnsInvoiceWithWeeks(t *testing.T) {
	s, _ := openTempStore(t)

	other := testInvoice()
	other.Month = time.October
	other.Year = 2027
	other.Project = "Other Project"

	otherMonth := testInvoice()
	otherMonth.Month = time.January
	otherMonth.Year = 2028
	otherMonth.Project = "Other Month"

	wanted := testInvoice()
	wanted.Month = time.October
	wanted.Year = 2028
	wanted.Project = "Wanted Project"

	for _, inv := range []*invoice.Invoice{other, otherMonth, wanted} {
		if _, _, err := s.Save(inv); err != nil {
			t.Fatalf("Save %q: %v", inv.Project, err)
		}
	}

	got, err := s.GetByMonth(2028, time.October)
	if err != nil {
		t.Fatalf("GetByMonth: %v", err)
	}
	if got.Year != wanted.Year || got.Month != wanted.Month {
		t.Errorf("Month and Year: got %v %d, want %v %d", got.Month, got.Year, wanted.Month, wanted.Year)
	}
	if got.Project != wanted.Project {
		t.Errorf("Project: got %q, want %q", got.Project, wanted.Project)
	}
	if len(got.Weeks) != len(wanted.Weeks) {
		t.Fatalf("Weeks: got %d, want %d", len(got.Weeks), len(wanted.Weeks))
	}
	for i := range wanted.Weeks {
		if !got.Weeks[i].Start.Equal(wanted.Weeks[i].Start) {
			t.Errorf("Week %d Start: got %v, want %v", i, got.Weeks[i].Start, wanted.Weeks[i].Start)
		}
		if !got.Weeks[i].End.Equal(wanted.Weeks[i].End) {
			t.Errorf("Week %d End: got %v, want %v", i, got.Weeks[i].End, wanted.Weeks[i].End)
		}
		if got.Weeks[i].Hours != wanted.Weeks[i].Hours {
			t.Errorf("Week %d Hours: got %v, want %v", i, got.Weeks[i].Hours, wanted.Weeks[i].Hours)
		}
	}
}

func TestGetByMonthReportsMissingInvoice(t *testing.T) {
	s, _ := openTempStore(t)

	_, err := s.GetByMonth(2027, time.October)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetByMonth error: got %v, want %v", err, store.ErrNotFound)
	}
}
