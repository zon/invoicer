// Package store persists invoices in a SQLite database.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/zon/invoicer/internal/invoice"
)

// Invoice is the stored form of an invoice.
type Invoice struct {
	ID       string `gorm:"primaryKey"`
	Month    int
	Year     int
	Vendor   string
	Customer string
	Project  string
	Rate     float64
	Weeks    []Week `gorm:"foreignKey:InvoiceID;constraint:OnDelete:CASCADE"`
}

// Week is the stored form of a weekly line item.
type Week struct {
	ID        uint   `gorm:"primaryKey"`
	InvoiceID string `gorm:"index;not null"`
	Start     time.Time
	End       time.Time
	Hours     float64
}

// ErrNotFound reports that the store holds no invoice matching a lookup.
var ErrNotFound = errors.New("invoice not found")

// Store reads and writes invoices in a SQLite database.
type Store struct {
	db *gorm.DB
}

// Open opens the database at path, creating its parent directory and running
// schema migrations when needed.
func Open(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("database path is required")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating database directory %q: %w", dir, err)
	}

	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("opening database %q: %w", path, err)
	}

	if err := db.AutoMigrate(&Invoice{}, &Week{}); err != nil {
		return nil, fmt.Errorf("migrating database: %w", err)
	}
	if err := ensureOneInvoicePerMonth(db); err != nil {
		return nil, fmt.Errorf("enforcing one invoice per month: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	db, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("getting database connection: %w", err)
	}
	return db.Close()
}

const monthIndex = "idx_invoices_year_month"

// ensureOneInvoicePerMonth keeps the most recently created invoice for each
// year and month, removes the other invoices and their weeks, and constrains
// the table so a month holds one invoice.
func ensureOneInvoicePerMonth(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := removeDuplicateInvoices(tx); err != nil {
			return err
		}
		return createMonthIndex(tx)
	})
}

func removeDuplicateInvoices(tx *gorm.DB) error {
	invoices := tx.Config.NamingStrategy.TableName("Invoice")
	weeks := tx.Config.NamingStrategy.TableName("Week")
	duplicates := fmt.Sprintf(
		"SELECT id FROM %s WHERE rowid NOT IN (SELECT MAX(rowid) FROM %s GROUP BY year, month)",
		invoices,
		invoices,
	)

	if err := tx.Exec(fmt.Sprintf("DELETE FROM %s WHERE invoice_id IN (%s)", weeks, duplicates)).Error; err != nil {
		return fmt.Errorf("removing duplicate invoice weeks: %w", err)
	}
	if err := tx.Exec(fmt.Sprintf("DELETE FROM %s WHERE id IN (%s)", invoices, duplicates)).Error; err != nil {
		return fmt.Errorf("removing duplicate invoices: %w", err)
	}
	return nil
}

func createMonthIndex(tx *gorm.DB) error {
	invoices := tx.Config.NamingStrategy.TableName("Invoice")
	statement := fmt.Sprintf("CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s (year, month)", monthIndex, invoices)
	if err := tx.Exec(statement).Error; err != nil {
		return fmt.Errorf("creating monthly invoice index: %w", err)
	}
	return nil
}

// Save stores an invoice and its weekly line items and returns the invoice
// identifier and whether the invoice was newly created. Saving an invoice for
// an existing year and month replaces the stored invoice, including its
// customer and project, and keeps its identifier.
func (s *Store) Save(inv *invoice.Invoice) (string, bool, error) {
	var id string
	created := false

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var existing Invoice
		err := tx.
			Where("year = ? AND month = ?", inv.Year, int(inv.Month)).
			First(&existing).
			Error

		switch {
		case err == nil:
			id = existing.ID
			if err := updateInvoice(tx, id, inv); err != nil {
				return err
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			newInvoiceID, err := newID()
			if err != nil {
				return err
			}
			id = newInvoiceID
			created = true
			if err := createInvoice(tx, id, inv); err != nil {
				return err
			}
		default:
			return fmt.Errorf("looking up invoice: %w", err)
		}

		return replaceWeeks(tx, id, inv.Weeks)
	})
	if err != nil {
		return "", false, err
	}
	return id, created, nil
}

func createInvoice(tx *gorm.DB, id string, inv *invoice.Invoice) error {
	record := &Invoice{
		ID:       id,
		Month:    int(inv.Month),
		Year:     inv.Year,
		Vendor:   inv.Vendor,
		Customer: inv.Customer,
		Project:  inv.Project,
		Rate:     inv.Rate,
	}
	if err := tx.Create(record).Error; err != nil {
		return fmt.Errorf("saving invoice: %w", err)
	}
	return nil
}

func updateInvoice(tx *gorm.DB, id string, inv *invoice.Invoice) error {
	updates := map[string]any{
		"month":    int(inv.Month),
		"year":     inv.Year,
		"vendor":   inv.Vendor,
		"customer": inv.Customer,
		"project":  inv.Project,
		"rate":     inv.Rate,
	}
	if err := tx.Model(&Invoice{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("updating invoice: %w", err)
	}
	return nil
}

func replaceWeeks(tx *gorm.DB, id string, weeks []invoice.Week) error {
	if err := tx.Where("invoice_id = ?", id).Delete(&Week{}).Error; err != nil {
		return fmt.Errorf("replacing invoice weeks: %w", err)
	}
	if len(weeks) == 0 {
		return nil
	}

	records := make([]Week, 0, len(weeks))
	for _, w := range weeks {
		records = append(records, Week{
			InvoiceID: id,
			Start:     w.Start,
			End:       w.End,
			Hours:     w.Hours,
		})
	}
	if err := tx.Create(&records).Error; err != nil {
		return fmt.Errorf("saving invoice weeks: %w", err)
	}
	return nil
}

// Get returns the invoice with the given identifier.
func (s *Store) Get(id string) (*invoice.Invoice, error) {
	var record Invoice
	err := s.db.
		Preload("Weeks", func(db *gorm.DB) *gorm.DB {
			return db.Order("start")
		}).
		First(&record, "id = ?", id).
		Error
	if err != nil {
		return nil, fmt.Errorf("loading invoice %q: %w", id, err)
	}
	return toInvoice(&record), nil
}

// List returns every stored invoice, newest first by year and month, with its
// weekly line items.
func (s *Store) List() ([]*invoice.Invoice, error) {
	var records []Invoice
	err := s.db.
		Preload("Weeks", func(db *gorm.DB) *gorm.DB {
			return db.Order("start")
		}).
		Order("year DESC").
		Order("month DESC").
		Find(&records).
		Error
	if err != nil {
		return nil, fmt.Errorf("listing invoices: %w", err)
	}

	invoices := make([]*invoice.Invoice, 0, len(records))
	for i := range records {
		invoices = append(invoices, toInvoice(&records[i]))
	}
	return invoices, nil
}

// GetByMonth returns the invoice for the given year and month. It reports
// ErrNotFound when that month holds no invoice.
func (s *Store) GetByMonth(year int, month time.Month) (*invoice.Invoice, error) {
	var record Invoice
	err := s.db.
		Preload("Weeks", func(db *gorm.DB) *gorm.DB {
			return db.Order("start")
		}).
		Where("year = ? AND month = ?", year, int(month)).
		First(&record).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("%w for %s %d", ErrNotFound, month, year)
	}
	if err != nil {
		return nil, fmt.Errorf("loading invoice for %s %d: %w", month, year, err)
	}
	return toInvoice(&record), nil
}

func toInvoice(record *Invoice) *invoice.Invoice {
	inv := &invoice.Invoice{
		Month:    time.Month(record.Month),
		Year:     record.Year,
		Vendor:   record.Vendor,
		Customer: record.Customer,
		Project:  record.Project,
		Rate:     record.Rate,
		Weeks:    make([]invoice.Week, 0, len(record.Weeks)),
	}
	for _, w := range record.Weeks {
		inv.Weeks = append(inv.Weeks, invoice.Week{
			Start: w.Start,
			End:   w.End,
			Hours: w.Hours,
		})
	}
	return inv
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generating invoice identifier: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}
