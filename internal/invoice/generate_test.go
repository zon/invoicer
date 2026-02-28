package invoice_test

import (
	"strings"
	"testing"
	"time"

	"github.com/zon/invoicer/internal/invoice"
)

func TestOutputFilename(t *testing.T) {
	inv := &invoice.Invoice{
		Customer: "Acme Corp",
		Year:     2025,
		Month:    time.January,
	}
	got := invoice.OutputFilename(inv)
	want := "invoice-acme-corp-2025-01"
	if got != want {
		t.Errorf("OutputFilename() = %q, want %q", got, want)
	}
}

func TestOutputFilename_SimpleCustomer(t *testing.T) {
	inv := &invoice.Invoice{
		Customer: "Google",
		Year:     2024,
		Month:    time.December,
	}
	got := invoice.OutputFilename(inv)
	want := "invoice-google-2024-12"
	if got != want {
		t.Errorf("OutputFilename() = %q, want %q", got, want)
	}
}

func TestInvoiceFilePath(t *testing.T) {
	inv := &invoice.Invoice{
		Customer: "Stripe",
		Year:     2025,
		Month:    time.March,
	}
	got := invoice.InvoiceFilePath(inv, "/tmp")
	if !strings.HasSuffix(got, ".html") {
		t.Errorf("InvoiceFilePath() = %q, want .html suffix", got)
	}
	if !strings.Contains(got, "stripe") {
		t.Errorf("InvoiceFilePath() = %q, want customer name in path", got)
	}
}

func TestPDFFilePath(t *testing.T) {
	inv := &invoice.Invoice{
		Customer: "Stripe",
		Year:     2025,
		Month:    time.March,
	}
	got := invoice.PDFFilePath(inv, "/tmp")
	if !strings.HasSuffix(got, ".pdf") {
		t.Errorf("PDFFilePath() = %q, want .pdf suffix", got)
	}
}

func TestFormatWeekLabel_SameMonth(t *testing.T) {
	w := invoice.Week{
		Start: time.Date(2025, time.January, 6, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2025, time.January, 12, 0, 0, 0, 0, time.UTC),
	}
	got := invoice.FormatWeekLabel(w)
	// Should contain "Jan" and "6" and "12".
	if !strings.Contains(got, "Jan") {
		t.Errorf("FormatWeekLabel() = %q, expected to contain 'Jan'", got)
	}
	if !strings.Contains(got, "6") {
		t.Errorf("FormatWeekLabel() = %q, expected to contain '6'", got)
	}
	if !strings.Contains(got, "12") {
		t.Errorf("FormatWeekLabel() = %q, expected to contain '12'", got)
	}
}

func TestFormatWeekLabel_CrossMonth(t *testing.T) {
	w := invoice.Week{
		Start: time.Date(2025, time.January, 30, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2025, time.February, 2, 0, 0, 0, 0, time.UTC),
	}
	got := invoice.FormatWeekLabel(w)
	if !strings.Contains(got, "Jan") {
		t.Errorf("FormatWeekLabel() = %q, expected to contain 'Jan'", got)
	}
	if !strings.Contains(got, "Feb") {
		t.Errorf("FormatWeekLabel() = %q, expected to contain 'Feb'", got)
	}
}
