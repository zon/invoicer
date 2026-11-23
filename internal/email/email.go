// Package email renders invoice email messages.
package email

import (
	"fmt"
	"strings"
	"time"

	"github.com/zon/invoicer/internal/invoice"
)

// Message is the plain-text email for one invoice.
type Message struct {
	To      string
	From    string
	Subject string
	Body    string
}

// Render builds the email message for inv, linking to the invoice detail under
// baseURL.
func Render(inv *invoice.Invoice, baseURL, from string) Message {
	return Message{
		To:      inv.ContactEmail,
		From:    from,
		Subject: fmt.Sprintf("Invoice for %s %d", inv.Month, inv.Year),
		Body: fmt.Sprintf(
			"Invoice %s for %s %d from %s.\n\nTotal: $%.2f\nView the invoice: %s",
			strings.ToUpper(inv.ID),
			inv.Month,
			inv.Year,
			inv.Vendor,
			inv.Total(),
			invoiceURL(baseURL, inv.Year, inv.Month),
		),
	}
}

func invoiceURL(baseURL string, year int, month time.Month) string {
	return strings.TrimRight(baseURL, "/") + detailPath(year, month)
}

func detailPath(year int, month time.Month) string {
	return fmt.Sprintf("/%d/%s", year, strings.ToLower(month.String()[:3]))
}
