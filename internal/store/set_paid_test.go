package store_test

import (
	"errors"
	"testing"
	"time"

	"github.com/zon/invoicer/internal/store"
)

func TestSetPaidChangesOnlyPaidStatus(t *testing.T) {
	s, _ := openTempStore(t)

	id, _, err := s.Save(testInvoice())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := s.SetPaid(2025, time.January, true); err != nil {
		t.Fatalf("SetPaid: %v", err)
	}

	got, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.Paid {
		t.Error("expected the invoice to be paid")
	}
	if got.Project != "Website Redesign" {
		t.Errorf("Project: got %q, want %q", got.Project, "Website Redesign")
	}
	if got.Vendor != "Jane Contractor" {
		t.Errorf("Vendor: got %q, want %q", got.Vendor, "Jane Contractor")
	}
	if got.Customer != "Acme Corp" {
		t.Errorf("Customer: got %q, want %q", got.Customer, "Acme Corp")
	}
	if got.Year != 2025 || got.Month != time.January {
		t.Errorf("Month and Year: got %v %d, want %v 2025", got.Month, got.Year, time.January)
	}
	if got.Rate != 150 {
		t.Errorf("Rate: got %v, want 150", got.Rate)
	}
	if len(got.Weeks) != 2 {
		t.Fatalf("Weeks: got %d, want 2", len(got.Weeks))
	}
	if got.Weeks[0].Hours != 32 || got.Weeks[1].Hours != 40 {
		t.Errorf("Week Hours: got %v and %v, want 32 and 40", got.Weeks[0].Hours, got.Weeks[1].Hours)
	}
	if got.Total() != 10800 {
		t.Errorf("Total: got %v, want 10800", got.Total())
	}

	listed, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("Invoices: got %d, want 1", len(listed))
	}
}

func TestSetPaidMarksInvoiceUnpaid(t *testing.T) {
	s, _ := openTempStore(t)

	id, _, err := s.Save(testInvoice())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := s.SetPaid(2025, time.January, true); err != nil {
		t.Fatalf("SetPaid paid: %v", err)
	}
	if err := s.SetPaid(2025, time.January, false); err != nil {
		t.Fatalf("SetPaid unpaid: %v", err)
	}

	got, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Paid {
		t.Error("expected the invoice to be unpaid")
	}
	if len(got.Weeks) != 2 {
		t.Fatalf("Weeks: got %d, want 2", len(got.Weeks))
	}
	if got.Total() != 10800 {
		t.Errorf("Total: got %v, want 10800", got.Total())
	}
}

func TestSetPaidReportsMissingInvoice(t *testing.T) {
	s, _ := openTempStore(t)

	if _, _, err := s.Save(testInvoice()); err != nil {
		t.Fatalf("Save: %v", err)
	}

	err := s.SetPaid(2027, time.October, true)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("SetPaid error: got %v, want %v", err, store.ErrNotFound)
	}

	got, err := s.GetByMonth(2025, time.January)
	if err != nil {
		t.Fatalf("GetByMonth: %v", err)
	}
	if got.Paid {
		t.Error("expected the stored invoice to stay unpaid")
	}
}
