package store_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zon/invoicer/internal/store"
)

func TestRemoveDeletesInvoiceAndWeeks(t *testing.T) {
	s, _ := openTempStore(t)

	removed, _, err := s.Save(testInvoice())
	if err != nil {
		t.Fatalf("Save removed: %v", err)
	}

	kept := testInvoice()
	kept.Month = time.February
	kept.Project = "Mobile App"
	keptID, _, err := s.Save(kept)
	if err != nil {
		t.Fatalf("Save kept: %v", err)
	}

	got, err := s.Remove(removed)
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if got.ID != removed {
		t.Errorf("ID: got %q, want %q", got.ID, removed)
	}
	if got.Month != time.January || got.Year != 2025 {
		t.Errorf("Month and Year: got %v %d, want %v 2025", got.Month, got.Year, time.January)
	}
	if len(got.Weeks) != 2 {
		t.Errorf("Weeks: got %d, want 2", len(got.Weeks))
	}

	if _, err := s.Get(removed); err == nil {
		t.Error("expected the removed invoice to be gone")
	}

	invoices, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(invoices) != 1 {
		t.Fatalf("Invoices: got %d, want 1", len(invoices))
	}
	if invoices[0].ID != keptID {
		t.Errorf("remaining invoice: got %q, want %q", invoices[0].ID, keptID)
	}
	if len(invoices[0].Weeks) != 2 {
		t.Errorf("remaining weeks: got %d, want 2", len(invoices[0].Weeks))
	}
}

func TestRemoveAcceptsUppercaseIdentifier(t *testing.T) {
	s, _ := openTempStore(t)

	id, _, err := s.Save(testInvoice())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := s.Remove(strings.ToUpper(id)); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	invoices, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(invoices) != 0 {
		t.Errorf("Invoices: got %d, want 0", len(invoices))
	}
}

func TestRemoveDeletesPaidInvoice(t *testing.T) {
	s, _ := openTempStore(t)

	id, _, err := s.Save(testInvoice())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := s.SetPaid(2025, time.January, true); err != nil {
		t.Fatalf("SetPaid: %v", err)
	}

	if _, err := s.Remove(id); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if _, err := s.Get(id); err == nil {
		t.Error("expected the paid invoice to be gone")
	}
}

func TestRemoveReportsMissingInvoice(t *testing.T) {
	s, _ := openTempStore(t)

	id, _, err := s.Save(testInvoice())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	_, err = s.Remove("missing")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Remove error: got %v, want %v", err, store.ErrNotFound)
	}

	if _, err := s.Get(id); err != nil {
		t.Errorf("expected the stored invoice to stay: %v", err)
	}
}
