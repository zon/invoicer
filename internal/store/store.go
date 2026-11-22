// Package store persists invoices in a SQLite database.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

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

	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("opening database %q: %w", path, err)
	}

	if err := db.AutoMigrate(&Invoice{}, &Week{}); err != nil {
		return nil, fmt.Errorf("migrating database: %w", err)
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

// Save stores an invoice and its weekly line items and returns the invoice identifier.
func (s *Store) Save(inv *invoice.Invoice) (string, error) {
	id, err := newID()
	if err != nil {
		return "", err
	}

	record := &Invoice{
		ID:       id,
		Month:    int(inv.Month),
		Year:     inv.Year,
		Vendor:   inv.Vendor,
		Customer: inv.Customer,
		Project:  inv.Project,
		Rate:     inv.Rate,
		Weeks:    make([]Week, 0, len(inv.Weeks)),
	}
	for _, w := range inv.Weeks {
		record.Weeks = append(record.Weeks, Week{
			Start: w.Start,
			End:   w.End,
			Hours: w.Hours,
		})
	}

	if err := s.db.Create(record).Error; err != nil {
		return "", fmt.Errorf("saving invoice: %w", err)
	}
	return id, nil
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
	return inv, nil
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generating invoice identifier: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}
