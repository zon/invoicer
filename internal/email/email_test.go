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

	wantText := "Invoice ABC123 for October 2026 from Jane Smith.\n\n" +
		"Total: $4800.00\n" +
		"View the invoice: https://invoices.example.com/2026/oct"
	if msg.Text != wantText {
		t.Errorf("Text:\n got %q\nwant %q", msg.Text, wantText)
	}

	wantHTML := "<p>Invoice ABC123 for October 2026 from Jane Smith.</p>\n" +
		"<p>Total: $4800.00</p>\n" +
		"<p><a href=\"https://invoices.example.com/2026/oct\">View the invoice</a></p>\n"
	if msg.HTML != wantHTML {
		t.Errorf("HTML:\n got %q\nwant %q", msg.HTML, wantHTML)
	}
}

func TestRenderTrimsTrailingSlashFromBaseURL(t *testing.T) {
	msg := email.Render(testInvoice(), "https://invoices.example.com/", "invoicer@example.com")

	wantLink := "https://invoices.example.com/2026/oct"
	if !strings.Contains(msg.Text, "View the invoice: "+wantLink) {
		t.Errorf("Text: got %q, want it to contain %q", msg.Text, wantLink)
	}
	if !strings.Contains(msg.HTML, "href=\""+wantLink+"\"") {
		t.Errorf("HTML: got %q, want the detail link", msg.HTML)
	}
	if strings.Contains(msg.Text+msg.HTML, "example.com//2026/oct") {
		t.Errorf("message: got a double slash before the detail path")
	}
}

func TestRenderUppercasesIdentifier(t *testing.T) {
	inv := testInvoice()
	inv.ID = "a1b2c3"

	msg := email.Render(inv, "https://invoices.example.com", "invoicer@example.com")

	if !strings.Contains(msg.Text, "Invoice A1B2C3 ") {
		t.Errorf("Text: got %q, want the uppercase identifier", msg.Text)
	}
	if !strings.Contains(msg.HTML, "Invoice A1B2C3 ") {
		t.Errorf("HTML: got %q, want the uppercase identifier", msg.HTML)
	}
}

func TestRenderEscapesStoredMarkupInHTML(t *testing.T) {
	inv := testInvoice()
	inv.Vendor = "<b>Jane</b>"

	msg := email.Render(inv, "https://invoices.example.com", "invoicer@example.com")

	if !strings.Contains(msg.Text, "from <b>Jane</b>.") {
		t.Errorf("Text: got %q, want the vendor as written", msg.Text)
	}
	if !strings.Contains(msg.HTML, "from &lt;b&gt;Jane&lt;/b&gt;.") {
		t.Errorf("HTML: got %q, want the vendor escaped", msg.HTML)
	}
	if strings.Contains(msg.HTML, "<b>Jane</b>") {
		t.Errorf("HTML: got %q, want no stored markup", msg.HTML)
	}
}
