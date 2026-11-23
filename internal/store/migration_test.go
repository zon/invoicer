package store

import (
	"path/filepath"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestOpenAddsUnpaidStatusToExistingInvoices(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invoices.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open existing database: %v", err)
	}

	statements := []string{
		`CREATE TABLE invoices (id text PRIMARY KEY, month integer, year integer, vendor text, customer text, project text, rate real)`,
		`INSERT INTO invoices (id, month, year, vendor, customer, project, rate)
			VALUES ('existing', 10, 2027, 'Vendor', 'Customer', 'Project', 100)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("prepare existing database: %v", err)
		}
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get database connection: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close existing database: %v", err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	got, err := s.Get("existing")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Paid {
		t.Error("expected an existing invoice to be unpaid")
	}
	if got.Customer != "Customer" {
		t.Errorf("Customer: got %q, want %q", got.Customer, "Customer")
	}
}

func TestOpenAddsEmailFieldsToExistingInvoices(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invoices.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open existing database: %v", err)
	}

	statements := []string{
		`CREATE TABLE invoices (id text PRIMARY KEY, month integer, year integer, vendor text, customer text, project text, rate real, paid numeric NOT NULL DEFAULT false)`,
		`INSERT INTO invoices (id, month, year, vendor, customer, project, rate, paid)
			VALUES ('existing', 10, 2027, 'Vendor', 'Customer', 'Project', 100, false)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("prepare existing database: %v", err)
		}
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get database connection: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close existing database: %v", err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	got, err := s.Get("existing")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ContactEmail != "" {
		t.Errorf("ContactEmail: got %q, want empty", got.ContactEmail)
	}
	if !got.EmailSentAt.IsZero() {
		t.Errorf("EmailSentAt: got %v, want no send time", got.EmailSentAt)
	}
	if got.Vendor != "Vendor" || got.Customer != "Customer" || got.Project != "Project" {
		t.Errorf("stored invoice changed: got %q, %q, %q", got.Vendor, got.Customer, got.Project)
	}
	if got.Rate != 100 {
		t.Errorf("Rate: got %v, want 100", got.Rate)
	}
}

func TestOpenKeepsNewestInvoicePerMonthAndRemovesOthers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invoices.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.db.Exec("DROP INDEX IF EXISTS " + monthIndex).Error; err != nil {
		t.Fatalf("drop index: %v", err)
	}

	records := []struct {
		id       string
		month    int
		customer string
	}{
		{id: "jan-first", month: 1, customer: "Jan First"},
		{id: "jan-second", month: 1, customer: "Jan Second"},
		{id: "jan-newest", month: 1, customer: "Jan Newest"},
		{id: "feb-first", month: 2, customer: "Feb First"},
		{id: "feb-newest", month: 2, customer: "Feb Newest"},
	}
	for _, record := range records {
		invoice := &Invoice{
			ID:       record.id,
			Month:    record.month,
			Year:     2025,
			Vendor:   "Vendor",
			Customer: record.customer,
			Project:  "Project",
			Rate:     100,
		}
		if err := s.db.Create(invoice).Error; err != nil {
			t.Fatalf("create invoice %q: %v", record.id, err)
		}
		week := &Week{
			InvoiceID: record.id,
			Start:     time.Date(2025, time.January, 6, 0, 0, 0, 0, time.UTC),
			End:       time.Date(2025, time.January, 12, 0, 0, 0, 0, time.UTC),
			Hours:     10,
		}
		if err := s.db.Create(week).Error; err != nil {
			t.Fatalf("create week for %q: %v", record.id, err)
		}
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

	kept := []struct {
		id       string
		customer string
	}{
		{id: "jan-newest", customer: "Jan Newest"},
		{id: "feb-newest", customer: "Feb Newest"},
	}
	for _, want := range kept {
		got, err := reopened.Get(want.id)
		if err != nil {
			t.Fatalf("Get kept invoice %q: %v", want.id, err)
		}
		if got.Customer != want.customer {
			t.Errorf("%s Customer: got %q, want %q", want.id, got.Customer, want.customer)
		}
		if len(got.Weeks) != 1 {
			t.Errorf("%s Weeks: got %d, want 1", want.id, len(got.Weeks))
		}
	}

	removed := []string{"jan-first", "jan-second", "feb-first"}
	weeksTable := reopened.db.Config.NamingStrategy.TableName("Week")
	for _, id := range removed {
		if _, err := reopened.Get(id); err == nil {
			t.Errorf("expected invoice %q to be removed", id)
		}
		var count int64
		if err := reopened.db.Table(weeksTable).Where("invoice_id = ?", id).Count(&count).Error; err != nil {
			t.Fatalf("count weeks for %q: %v", id, err)
		}
		if count != 0 {
			t.Errorf("%s Weeks: got %d, want 0", id, count)
		}
	}

	for _, month := range []int{1, 2} {
		var count int64
		if err := reopened.db.Model(&Invoice{}).Where("year = ? AND month = ?", 2025, month).Count(&count).Error; err != nil {
			t.Fatalf("count invoices for month %d: %v", month, err)
		}
		if count != 1 {
			t.Errorf("month %d invoices: got %d, want 1", month, count)
		}
	}

	duplicate := &Invoice{
		ID:       "jan-duplicate",
		Month:    1,
		Year:     2025,
		Vendor:   "Vendor",
		Customer: "Duplicate",
		Rate:     100,
	}
	if err := reopened.db.Create(duplicate).Error; err == nil {
		t.Error("expected a second invoice for the same month to be rejected")
	}
}
