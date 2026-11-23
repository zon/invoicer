package store_test

import (
	"errors"
	"testing"
	"time"

	"github.com/zon/invoicer/internal/store"
)

func TestSaveStoresContactEmail(t *testing.T) {
	s, _ := openTempStore(t)

	inv := testInvoice()
	inv.ContactEmail = "customer@example.com"

	id, _, err := s.Save(inv)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ContactEmail != inv.ContactEmail {
		t.Errorf("ContactEmail: got %q, want %q", got.ContactEmail, inv.ContactEmail)
	}
	if !got.EmailSentAt.IsZero() {
		t.Errorf("EmailSentAt: got %v, want no send time", got.EmailSentAt)
	}
}

func TestLoadingReturnsContactEmailAndSendTime(t *testing.T) {
	s, _ := openTempStore(t)

	inv := testInvoice()
	inv.ContactEmail = "customer@example.com"
	id, _, err := s.Save(inv)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	sentAt := time.Date(2026, time.November, 23, 9, 30, 0, 0, time.UTC)
	if err := s.SetEmailSent(2025, time.January, sentAt); err != nil {
		t.Fatalf("SetEmailSent: %v", err)
	}

	byID, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	byMonth, err := s.GetByMonth(2025, time.January)
	if err != nil {
		t.Fatalf("GetByMonth: %v", err)
	}
	listed, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("Invoices: got %d, want 1", len(listed))
	}

	checks := []struct {
		name         string
		contactEmail string
		emailSentAt  time.Time
	}{
		{name: "Get", contactEmail: byID.ContactEmail, emailSentAt: byID.EmailSentAt},
		{name: "GetByMonth", contactEmail: byMonth.ContactEmail, emailSentAt: byMonth.EmailSentAt},
		{name: "List", contactEmail: listed[0].ContactEmail, emailSentAt: listed[0].EmailSentAt},
	}
	for _, check := range checks {
		if check.contactEmail != inv.ContactEmail {
			t.Errorf("%s ContactEmail: got %q, want %q", check.name, check.contactEmail, inv.ContactEmail)
		}
		if !check.emailSentAt.Equal(sentAt) {
			t.Errorf("%s EmailSentAt: got %v, want %v", check.name, check.emailSentAt, sentAt)
		}
	}
}

func TestSaveKeepsExistingSendTime(t *testing.T) {
	s, _ := openTempStore(t)

	inv := testInvoice()
	inv.ContactEmail = "customer@example.com"
	id, _, err := s.Save(inv)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	sentAt := time.Date(2026, time.November, 23, 9, 30, 0, 0, time.UTC)
	if err := s.SetEmailSent(2025, time.January, sentAt); err != nil {
		t.Fatalf("SetEmailSent: %v", err)
	}

	updated := testInvoice()
	updated.ContactEmail = "new@example.com"
	updated.Customer = "Globex"
	if _, _, err := s.Save(updated); err != nil {
		t.Fatalf("Save updated: %v", err)
	}

	got, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ContactEmail != updated.ContactEmail {
		t.Errorf("ContactEmail: got %q, want %q", got.ContactEmail, updated.ContactEmail)
	}
	if !got.EmailSentAt.Equal(sentAt) {
		t.Errorf("EmailSentAt: got %v, want %v", got.EmailSentAt, sentAt)
	}
}

func TestSetEmailSentReportsMissingInvoice(t *testing.T) {
	s, _ := openTempStore(t)

	if _, _, err := s.Save(testInvoice()); err != nil {
		t.Fatalf("Save: %v", err)
	}

	sentAt := time.Date(2026, time.November, 23, 9, 30, 0, 0, time.UTC)
	err := s.SetEmailSent(2027, time.October, sentAt)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("SetEmailSent error: got %v, want %v", err, store.ErrNotFound)
	}

	got, err := s.GetByMonth(2025, time.January)
	if err != nil {
		t.Fatalf("GetByMonth: %v", err)
	}
	if !got.EmailSentAt.IsZero() {
		t.Errorf("EmailSentAt: got %v, want no send time", got.EmailSentAt)
	}
}
