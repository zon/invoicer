// Package email renders and sends invoice email messages.
package email

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/resend/resend-go/v4"
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

// sendTimeout bounds one send to Resend.
const sendTimeout = 10 * time.Second

// Sender sends invoice email through Resend.
type Sender struct {
	client  *resend.Client
	timeout time.Duration
}

// NewSender reads the Resend API key from apiKeyFile and returns a sender that
// uses it.
func NewSender(apiKeyFile string) (*Sender, error) {
	key, err := os.ReadFile(apiKeyFile)
	if err != nil {
		return nil, fmt.Errorf("reading resend.apiKeyFile %q: %w", apiKeyFile, err)
	}
	return &Sender{
		client:  resend.NewClient(strings.TrimSpace(string(key))),
		timeout: sendTimeout,
	}, nil
}

// Send sends msg as the email for the invoice with stored identifier id and
// returns the Resend email identifier.
func (s *Sender) Send(msg Message, id string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	sent, err := s.client.Emails.SendWithOptions(ctx, &resend.SendEmailRequest{
		From:    msg.From,
		To:      []string{msg.To},
		Subject: msg.Subject,
		Text:    msg.Body,
	}, &resend.SendEmailOptions{IdempotencyKey: "invoice/" + id})
	if err != nil {
		return "", fmt.Errorf("sending invoice email: %w", err)
	}
	return sent.Id, nil
}
