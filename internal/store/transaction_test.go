package store

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/zon/invoicer/internal/invoice"
)

func TestSaveFailureLeavesStoredInvoiceUnchanged(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "invoices.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	original := &invoice.Invoice{
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
		},
	}
	id, _, err := s.Save(original)
	if err != nil {
		t.Fatalf("Save original: %v", err)
	}

	weeksTable := s.db.Config.NamingStrategy.TableName("Week")
	trigger := fmt.Sprintf(
		`CREATE TRIGGER fail_week_insert BEFORE INSERT ON %s BEGIN SELECT RAISE(ABORT, 'forced failure'); END;`,
		weeksTable,
	)
	if err := s.db.Exec(trigger).Error; err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	replacement := &invoice.Invoice{
		Month:    original.Month,
		Year:     original.Year,
		Vendor:   "New Vendor",
		Customer: original.Customer,
		Project:  original.Project,
		Rate:     200,
		Weeks: []invoice.Week{
			{
				Start: time.Date(2025, time.January, 6, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2025, time.January, 12, 0, 0, 0, 0, time.UTC),
				Hours: 40,
			},
		},
	}
	if _, _, err := s.Save(replacement); err == nil {
		t.Fatal("expected save to fail")
	}

	got, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Vendor != original.Vendor {
		t.Errorf("Vendor: got %q, want %q", got.Vendor, original.Vendor)
	}
	if got.Rate != original.Rate {
		t.Errorf("Rate: got %v, want %v", got.Rate, original.Rate)
	}
	if len(got.Weeks) != len(original.Weeks) {
		t.Fatalf("Weeks: got %d, want %d", len(got.Weeks), len(original.Weeks))
	}
	if !got.Weeks[0].Start.Equal(original.Weeks[0].Start) {
		t.Errorf("Week Start: got %v, want %v", got.Weeks[0].Start, original.Weeks[0].Start)
	}
	if !got.Weeks[0].End.Equal(original.Weeks[0].End) {
		t.Errorf("Week End: got %v, want %v", got.Weeks[0].End, original.Weeks[0].End)
	}
	if got.Weeks[0].Hours != original.Weeks[0].Hours {
		t.Errorf("Week Hours: got %v, want %v", got.Weeks[0].Hours, original.Weeks[0].Hours)
	}
}

func TestSetProjectFailureLeavesStoredProjectUnchanged(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "invoices.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	original := &invoice.Invoice{
		Month:    time.October,
		Year:     2027,
		Vendor:   "Jane Contractor",
		Customer: "Acme Corp",
		Project:  "Website Redesign",
		Rate:     150,
		Weeks: []invoice.Week{
			{
				Start: time.Date(2027, time.September, 27, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2027, time.October, 3, 0, 0, 0, 0, time.UTC),
				Hours: 32,
			},
		},
	}
	id, _, err := s.Save(original)
	if err != nil {
		t.Fatalf("Save original: %v", err)
	}

	invoicesTable := s.db.Config.NamingStrategy.TableName("Invoice")
	trigger := fmt.Sprintf(
		`CREATE TRIGGER fail_invoice_update BEFORE UPDATE ON %s BEGIN SELECT RAISE(ABORT, 'forced failure'); END;`,
		invoicesTable,
	)
	if err := s.db.Exec(trigger).Error; err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	if err := s.SetProject(2027, time.October, "Mobile App"); err == nil {
		t.Fatal("expected SetProject to fail")
	}

	got, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Project != original.Project {
		t.Errorf("Project: got %q, want %q", got.Project, original.Project)
	}
	if len(got.Weeks) != len(original.Weeks) {
		t.Fatalf("Weeks: got %d, want %d", len(got.Weeks), len(original.Weeks))
	}
	if got.Total() != original.Total() {
		t.Errorf("Total: got %v, want %v", got.Total(), original.Total())
	}
}
