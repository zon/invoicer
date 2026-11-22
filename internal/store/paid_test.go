package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStoreRecordsPaidStatus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invoices.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	record := &Invoice{
		ID:       "paid-invoice",
		Month:    int(time.October),
		Year:     2027,
		Vendor:   "Vendor",
		Customer: "Customer",
		Rate:     100,
		Paid:     true,
	}
	if err := s.db.Create(record).Error; err != nil {
		t.Fatalf("create paid invoice: %v", err)
	}

	got, err := s.Get(record.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.Paid {
		t.Error("expected Get to report the invoice paid")
	}

	listed, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("Invoices: got %d, want 1", len(listed))
	}
	if !listed[0].Paid {
		t.Error("expected List to report the invoice paid")
	}

	byMonth, err := s.GetByMonth(2027, time.October)
	if err != nil {
		t.Fatalf("GetByMonth: %v", err)
	}
	if !byMonth.Paid {
		t.Error("expected GetByMonth to report the invoice paid")
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() {
		if err := reopened.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	persisted, err := reopened.Get(record.ID)
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if !persisted.Paid {
		t.Error("expected the paid status to survive a reopen")
	}
}
