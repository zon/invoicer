package cli

import (
	"fmt"
	"io"

	"github.com/zon/invoicer/internal/email"
	"github.com/zon/invoicer/internal/invoice"
	"github.com/zon/invoicer/internal/store"
)

// emailSender sends a rendered invoice email and returns the Resend email
// identifier.
type emailSender interface {
	Send(msg email.Message, id string) (string, error)
}

// newEmailSender builds a sender that reads its API key from apiKeyFile. Tests
// replace it to record sends without contacting Resend.
var newEmailSender = func(apiKeyFile string) (emailSender, error) {
	return email.NewSender(apiKeyFile)
}

// sendInvoice renders and sends the invoice email, records the send time, and
// prints the recipient and the Resend email identifier.
func sendInvoice(out io.Writer, s *store.Store, inv *invoice.Invoice, opts *ResolvedOptions) error {
	sender, err := newEmailSender(opts.Resend.APIKeyFile)
	if err != nil {
		return fmt.Errorf("sending invoice email: %w", err)
	}

	msg := email.Render(inv, opts.BaseURL, opts.Resend.From)
	emailID, err := sender.Send(msg, inv.ID)
	if err != nil {
		return err
	}

	if err := s.SetEmailSent(inv.Year, inv.Month, invoice.Now()); err != nil {
		return fmt.Errorf("recording email send time: %w", err)
	}

	fmt.Fprintf(out, "Sent invoice email to %s: %s\n", msg.To, emailID)
	return nil
}
