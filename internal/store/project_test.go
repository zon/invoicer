package store_test

import (
	"errors"
	"testing"
	"time"

	"github.com/zon/invoicer/internal/store"
)

func TestSetProjectChangesOnlyProject(t *testing.T) {
	s, _ := openTempStore(t)

	id, _, err := s.Save(testInvoice())
	if err != nil {
		t.Fatalf("Save: %v", err)
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
}

func TestSetProjectClearsProject(t *testing.T) {
	s, _ := openTempStore(t)

	id, _, err := s.Save(testInvoice())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := s.SetProject(2025, time.January, ""); err != nil {
		t.Fatalf("SetProject: %v", err)
	}

	got, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Project != "" {
		t.Errorf("Project: got %q, want empty", got.Project)
	}
}

func TestSetProjectReportsMissingInvoice(t *testing.T) {
	s, _ := openTempStore(t)

	if _, _, err := s.Save(testInvoice()); err != nil {
		t.Fatalf("Save: %v", err)
	}

	err := s.SetProject(2027, time.October, "Mobile App")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("SetProject error: got %v, want %v", err, store.ErrNotFound)
	}
}
