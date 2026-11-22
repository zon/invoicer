package store_test

import (
	"errors"
	"testing"
	"time"

	"github.com/zon/invoicer/internal/store"
)

func TestSaveRefusesPaidInvoice(t *testing.T) {
	s, _ := openTempStore(t)

	original := testInvoice()
	id, _, err := s.Save(original)
	if err != nil {
		t.Fatalf("Save original: %v", err)
	}
	if err := s.SetPaid(2025, time.January, true); err != nil {
		t.Fatalf("SetPaid: %v", err)
	}

	replacement := testInvoice()
	replacement.Vendor = "New Vendor"
	replacement.Customer = "Globex"
	replacement.Project = "Mobile App"
	replacement.Rate = 200
	replacement.Weeks = replacement.Weeks[:1]

	if _, _, err := s.Save(replacement); !errors.Is(err, store.ErrPaid) {
		t.Fatalf("Save error: got %v, want %v", err, store.ErrPaid)
	}

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

func TestSaveReplacesInvoiceAfterItIsMarkedUnpaid(t *testing.T) {
	s, _ := openTempStore(t)

	id, _, err := s.Save(testInvoice())
	if err != nil {
		t.Fatalf("Save original: %v", err)
	}
	if err := s.SetPaid(2025, time.January, true); err != nil {
		t.Fatalf("SetPaid paid: %v", err)
	}
	if err := s.SetPaid(2025, time.January, false); err != nil {
		t.Fatalf("SetPaid unpaid: %v", err)
	}

	replacement := testInvoice()
	replacement.Project = "Mobile App"

	updatedID, created, err := s.Save(replacement)
	if err != nil {
		t.Fatalf("Save replacement: %v", err)
	}
	if created {
		t.Error("expected the replacement to report not created")
	}
	if updatedID != id {
		t.Fatalf("identifier changed: got %q, want %q", updatedID, id)
	}

	got, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Project != replacement.Project {
		t.Errorf("Project: got %q, want %q", got.Project, replacement.Project)
	}
}

func TestSetProjectRefusesPaidInvoice(t *testing.T) {
	s, _ := openTempStore(t)

	original := testInvoice()
	id, _, err := s.Save(original)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := s.SetPaid(2025, time.January, true); err != nil {
		t.Fatalf("SetPaid: %v", err)
	}

	if err := s.SetProject(2025, time.January, "Mobile App"); !errors.Is(err, store.ErrPaid) {
		t.Fatalf("SetProject error: got %v, want %v", err, store.ErrPaid)
	}

	got, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.Paid {
		t.Error("expected the invoice to stay paid")
	}
	if got.Project != original.Project {
		t.Errorf("Project: got %q, want %q", got.Project, original.Project)
	}
	if got.Vendor != original.Vendor {
		t.Errorf("Vendor: got %q, want %q", got.Vendor, original.Vendor)
	}
	if got.Customer != original.Customer {
		t.Errorf("Customer: got %q, want %q", got.Customer, original.Customer)
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

func TestSetProjectChangesPaidInvoiceAfterItIsMarkedUnpaid(t *testing.T) {
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

	if err := s.SetProject(2025, time.January, "Mobile App"); err != nil {
		t.Fatalf("SetProject: %v", err)
	}

	got, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Project != "Mobile App" {
		t.Errorf("Project: got %q, want %q", got.Project, "Mobile App")
	}
}
