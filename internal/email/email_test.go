package email_test

import (
	"strings"
	"testing"
	"time"

	"github.com/zon/invoicer/internal/email"
	"github.com/zon/invoicer/internal/invoice"
)

func testInvoice() *invoice.Invoice {
	return &invoice.Invoice{
		ID:           "abc123",
		Month:        time.October,
		Year:         2026,
		Vendor:       "Jane Smith",
		Customer:     "Acme Corp",
		ContactEmail: "billing@acme.com",
		Rate:         100,
		Weeks: []invoice.Week{
			{Hours: 40},
			{Hours: 8},
		},
	}
}

func TestRenderInvoiceMessage(t *testing.T) {
	msg := email.Render(testInvoice(), "https://invoices.example.com", "invoicer@example.com")

	if msg.To != "billing@acme.com" {
		t.Errorf("To: got %q, want the contact email", msg.To)
	}
	if msg.From != "invoicer@example.com" {
		t.Errorf("From: got %q, want the configured sender", msg.From)
	}
	if msg.Subject != "Invoice for October 2026" {
		t.Errorf("Subject: got %q, want %q", msg.Subject, "Invoice for October 2026")
	}

	want := "Invoice ABC123 for October 2026 from Jane Smith.\n\n" +
		"Total: $4800.00\n" +
		"View the invoice: https://invoices.example.com/2026/oct"
	if msg.Body != want {
		t.Errorf("Body:\n got %q\nwant %q", msg.Body, want)
	}
}

func TestRenderTrimsTrailingSlashFromBaseURL(t *testing.T) {
	msg := email.Render(testInvoice(), "https://invoices.example.com/", "invoicer@example.com")

	want := "View the invoice: https://invoices.example.com/2026/oct"
	if !strings.Contains(msg.Body, want) {
		t.Errorf("Body: got %q, want it to contain %q", msg.Body, want)
	}
	if strings.Contains(msg.Body, "example.com//2026/oct") {
		t.Errorf("Body: got %q, want one slash before the detail path", msg.Body)
	}
}

func TestRenderUppercasesIdentifier(t *testing.T) {
	inv := testInvoice()
	inv.ID = "a1b2c3"

	msg := email.Render(inv, "https://invoices.example.com", "invoicer@example.com")

	if !strings.Contains(msg.Body, "Invoice A1B2C3 ") {
		t.Errorf("Body: got %q, want the uppercase identifier", msg.Body)
	}
}

func TestRenderKeepsStoredMarkupAsText(t *testing.T) {
	inv := testInvoice()
	inv.Vendor = "<b>Jane</b>"

	msg := email.Render(inv, "https://invoices.example.com", "invoicer@example.com")

	if !strings.Contains(msg.Body, "from <b>Jane</b>.") {
		t.Errorf("Body: got %q, want the vendor as plain text", msg.Body)
	}
}
