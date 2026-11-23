package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zon/invoicer/internal/config"
	"github.com/zon/invoicer/internal/email"
	"github.com/zon/invoicer/internal/invoice"
	"github.com/zon/invoicer/internal/store"
)

// stubSender records the messages generate sends and returns a fixed email
// identifier.
type stubSender struct {
	messages []email.Message
	ids      []string
	emailID  string
	err      error
}

func (s *stubSender) Send(msg email.Message, id string) (string, error) {
	s.messages = append(s.messages, msg)
	s.ids = append(s.ids, id)
	if s.err != nil {
		return "", s.err
	}
	return s.emailID, nil
}

// useStubSender replaces the email sender that generate builds.
func useStubSender(t *testing.T, sender *stubSender) {
	t.Helper()
	original := newEmailSender
	newEmailSender = func(string) (emailSender, error) {
		return sender, nil
	}
	t.Cleanup(func() { newEmailSender = original })
}

// fixNow pins the clock that records the email send time.
func fixNow(t *testing.T, now time.Time) {
	t.Helper()
	original := invoice.Now
	invoice.Now = func() time.Time { return now }
	t.Cleanup(func() { invoice.Now = original })
}

// writeAPIKeyFile creates a file that holds a Resend API key.
func writeAPIKeyFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "resend-api-key")
	if err := os.WriteFile(path, []byte("re_test_key\n"), 0o600); err != nil {
		t.Fatalf("writing API key file: %v", err)
	}
	return path
}

// emailConfig returns a config that turns on email sending.
func emailConfig(t *testing.T) string {
	t.Helper()
	return emailConfigWithKeyFile(t, writeAPIKeyFile(t))
}

// emailConfigWithKeyFile returns a config that turns on email sending and names
// the given API key file.
func emailConfigWithKeyFile(t *testing.T, keyFile string) string {
	t.Helper()
	return fmt.Sprintf(`customerEmail: billing@acme.com
baseUrl: https://invoices.example.com
resend:
  apiKeyFile: %s
  from: invoicer@example.com
`, keyFile)
}

func TestGenerateSendsCreatedInvoice(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "invoices.db")
	configPath := writeTestConfig(t, emailConfig(t))

	sentAt := time.Date(2026, time.November, 23, 9, 30, 0, 0, time.UTC)
	fixNow(t, sentAt)

	sender := &stubSender{emailID: "email_123"}
	useStubSender(t, sender)

	cmd := &GenerateCmd{
		Month:    "january",
		Year:     2025,
		Vendor:   "Jane Contractor",
		Customer: "Acme Corp",
		Rate:     150,
		Hours:    40,
		Database: dbPath,
	}

	var out bytes.Buffer
	if err := cmd.run(configPath, &out); err != nil {
		t.Fatalf("run: %v", err)
	}

	id := outputID(t, out.String())
	want := "Created invoice for January 2025: " + id + "\n" +
		"Sent invoice email to billing@acme.com: email_123\n"
	if out.String() != want {
		t.Errorf("output: got %q, want %q", out.String(), want)
	}

	if len(sender.messages) != 1 {
		t.Fatalf("messages: got %d, want 1", len(sender.messages))
	}
	msg := sender.messages[0]
	if msg.To != "billing@acme.com" {
		t.Errorf("To: got %q, want the stored contact email", msg.To)
	}
	if msg.From != "invoicer@example.com" {
		t.Errorf("From: got %q, want the configured sender", msg.From)
	}
	if msg.Subject != "Invoice for January 2025" {
		t.Errorf("Subject: got %q, want the invoice subject", msg.Subject)
	}
	if !strings.Contains(msg.Body, "Invoice "+id+" ") {
		t.Errorf("Body: got %q, want the uppercase identifier", msg.Body)
	}
	if !strings.Contains(msg.Body, "View the invoice: https://invoices.example.com/2025/jan") {
		t.Errorf("Body: got %q, want the detail link", msg.Body)
	}
	if sender.ids[0] != strings.ToLower(id) {
		t.Errorf("send identifier: got %q, want the stored identifier", sender.ids[0])
	}

	got := loadInvoice(t, dbPath, id)
	if !got.EmailSentAt.Equal(sentAt) {
		t.Errorf("EmailSentAt: got %v, want %v", got.EmailSentAt, sentAt)
	}
}

func TestGenerateSecondRunSendsNothingAndKeepsSendTime(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "invoices.db")
	configPath := writeTestConfig(t, emailConfig(t))

	sentAt := time.Date(2026, time.November, 23, 9, 30, 0, 0, time.UTC)
	fixNow(t, sentAt)

	sender := &stubSender{emailID: "email_123"}
	useStubSender(t, sender)

	cmd := &GenerateCmd{
		Month:    "january",
		Year:     2025,
		Vendor:   "Jane Contractor",
		Customer: "Acme Corp",
		Rate:     150,
		Hours:    40,
		Database: dbPath,
	}

	var firstOut bytes.Buffer
	if err := cmd.run(configPath, &firstOut); err != nil {
		t.Fatalf("first run: %v", err)
	}

	second := *cmd
	second.Rate = 200
	var secondOut bytes.Buffer
	if err := second.run(configPath, &secondOut); err != nil {
		t.Fatalf("second run: %v", err)
	}

	id := outputID(t, firstOut.String())
	want := "Edited invoice for January 2025: " + id + "\n"
	if secondOut.String() != want {
		t.Errorf("second output: got %q, want %q", secondOut.String(), want)
	}
	if len(sender.messages) != 1 {
		t.Errorf("messages: got %d, want no second send", len(sender.messages))
	}

	got := loadInvoice(t, dbPath, id)
	if !got.EmailSentAt.Equal(sentAt) {
		t.Errorf("EmailSentAt: got %v, want the first send time %v", got.EmailSentAt, sentAt)
	}
}

func TestGenerateSendsWhenEditingUnsentInvoice(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "invoices.db")

	cmd := &GenerateCmd{
		Month:         "january",
		Year:          2025,
		Vendor:        "Jane Contractor",
		Customer:      "Acme Corp",
		CustomerEmail: "billing@acme.com",
		Rate:          150,
		Hours:         40,
		Database:      dbPath,
	}

	var firstOut bytes.Buffer
	if err := cmd.run(writeTestConfig(t, ""), &firstOut); err != nil {
		t.Fatalf("first run: %v", err)
	}

	sentAt := time.Date(2026, time.November, 23, 9, 30, 0, 0, time.UTC)
	fixNow(t, sentAt)

	sender := &stubSender{emailID: "email_456"}
	useStubSender(t, sender)

	second := *cmd
	second.Rate = 200
	var secondOut bytes.Buffer
	if err := second.run(writeTestConfig(t, emailConfig(t)), &secondOut); err != nil {
		t.Fatalf("second run: %v", err)
	}

	id := outputID(t, firstOut.String())
	want := "Edited invoice for January 2025: " + id + "\n" +
		"Sent invoice email to billing@acme.com: email_456\n"
	if secondOut.String() != want {
		t.Errorf("second output: got %q, want %q", secondOut.String(), want)
	}
	if len(sender.messages) != 1 {
		t.Fatalf("messages: got %d, want 1", len(sender.messages))
	}

	got := loadInvoice(t, dbPath, id)
	if !got.EmailSentAt.Equal(sentAt) {
		t.Errorf("EmailSentAt: got %v, want %v", got.EmailSentAt, sentAt)
	}
}

func TestGenerateReportsFailedSendAndLeavesInvoiceUnsent(t *testing.T) {
	cases := []struct {
		name   string
		config func(t *testing.T) string
		sender *stubSender
		want   string
	}{
		{
			name:   "timeout",
			config: emailConfig,
			sender: &stubSender{err: context.DeadlineExceeded},
			want:   "context deadline exceeded",
		},
		{
			name:   "rejected message",
			config: emailConfig,
			sender: &stubSender{err: errors.New("sending invoice email: invalid from address")},
			want:   "invalid from address",
		},
		{
			name: "missing API key file",
			config: func(t *testing.T) string {
				return emailConfigWithKeyFile(t, filepath.Join(t.TempDir(), "missing"))
			},
			want: "reading resend.apiKeyFile",
		},
		{
			name: "unreadable API key file",
			config: func(t *testing.T) string {
				return emailConfigWithKeyFile(t, t.TempDir())
			},
			want: "reading resend.apiKeyFile",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			dbPath := filepath.Join(dir, "invoices.db")
			configPath := writeTestConfig(t, tc.config(t))

			if tc.sender != nil {
				useStubSender(t, tc.sender)
			}

			cmd := &GenerateCmd{
				Month:    "january",
				Year:     2025,
				Vendor:   "Jane Contractor",
				Customer: "Acme Corp",
				Rate:     150,
				Hours:    40,
				Database: dbPath,
			}

			var out bytes.Buffer
			err := cmd.run(configPath, &out)
			if err == nil {
				t.Fatal("run did not report the failed send")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error: got %q, want it to contain %q", err, tc.want)
			}

			id := outputID(t, out.String())
			wantOut := "Created invoice for January 2025: " + id + "\n"
			if out.String() != wantOut {
				t.Errorf("output: got %q, want only the identifier %q", out.String(), wantOut)
			}
			if tc.sender != nil && len(tc.sender.messages) != 1 {
				t.Errorf("messages: got %d, want one attempted send", len(tc.sender.messages))
			}

			got := loadInvoice(t, dbPath, id)
			if !got.EmailSentAt.IsZero() {
				t.Errorf("EmailSentAt: got %v, want no send time", got.EmailSentAt)
			}

			sentAt := time.Date(2026, time.November, 23, 9, 30, 0, 0, time.UTC)
			fixNow(t, sentAt)

			sender := &stubSender{emailID: "email_123"}
			useStubSender(t, sender)

			second := *cmd
			second.Rate = 200
			var secondOut bytes.Buffer
			if err := second.run(writeTestConfig(t, emailConfig(t)), &secondOut); err != nil {
				t.Fatalf("second run: %v", err)
			}

			wantSecond := "Edited invoice for January 2025: " + id + "\n" +
				"Sent invoice email to billing@acme.com: email_123\n"
			if secondOut.String() != wantSecond {
				t.Errorf("second output: got %q, want %q", secondOut.String(), wantSecond)
			}
			if len(sender.messages) != 1 {
				t.Errorf("second messages: got %d, want one send", len(sender.messages))
			}

			got = loadInvoice(t, dbPath, id)
			if !got.EmailSentAt.Equal(sentAt) {
				t.Errorf("EmailSentAt: got %v, want the second send time %v", got.EmailSentAt, sentAt)
			}
		})
	}
}

func TestSendInvoiceReportsFailureToRecordSendTime(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "invoices.db")

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	id, _, err := s.Save(&invoice.Invoice{
		Month:        time.January,
		Year:         2025,
		Vendor:       "Jane Contractor",
		Customer:     "Acme Corp",
		ContactEmail: "billing@acme.com",
		Rate:         150,
		Weeks:        invoice.WeeksForMonth(2025, time.January, 40),
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	stored, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("closing store: %v", err)
	}

	sender := &stubSender{emailID: "email_123"}
	useStubSender(t, sender)

	opts := &ResolvedOptions{
		BaseURL: "https://invoices.example.com",
		Resend: &config.Resend{
			APIKeyFile: writeAPIKeyFile(t),
			From:       "invoicer@example.com",
		},
	}

	var out bytes.Buffer
	err = sendInvoice(&out, s, stored, opts)
	if err == nil {
		t.Fatal("sendInvoice did not report the failed send time")
	}
	if !strings.Contains(err.Error(), "recording email send time") {
		t.Errorf("error: got %q, want the send time failure", err)
	}
	if len(sender.messages) != 1 {
		t.Errorf("messages: got %d, want the accepted send", len(sender.messages))
	}
	if out.Len() != 0 {
		t.Errorf("output: got %q, want nothing before the send time is recorded", out.String())
	}

	got := loadInvoice(t, dbPath, strings.ToLower(id))
	if !got.EmailSentAt.IsZero() {
		t.Errorf("EmailSentAt: got %v, want no send time", got.EmailSentAt)
	}
}

func TestGeneratePaidInvoiceSendsNothing(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "invoices.db")

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	_, _, err = s.Save(&invoice.Invoice{
		Month:        time.January,
		Year:         2025,
		Vendor:       "Jane Contractor",
		Customer:     "Acme Corp",
		ContactEmail: "billing@acme.com",
		Rate:         150,
		Weeks:        invoice.WeeksForMonth(2025, time.January, 40),
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := s.SetPaid(2025, time.January, true); err != nil {
		t.Fatalf("SetPaid: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("closing store: %v", err)
	}

	sender := &stubSender{emailID: "email_789"}
	useStubSender(t, sender)

	cmd := &GenerateCmd{
		Month:    "january",
		Year:     2025,
		Vendor:   "Jane Contractor",
		Customer: "Acme Corp",
		Rate:     200,
		Hours:    10,
		Database: dbPath,
	}

	var out bytes.Buffer
	err = cmd.run(writeTestConfig(t, emailConfig(t)), &out)
	if !errors.Is(err, store.ErrPaid) {
		t.Fatalf("run error: got %v, want %v", err, store.ErrPaid)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output, got %q", out.String())
	}
	if len(sender.messages) != 0 {
		t.Errorf("messages: got %d, want no send for a paid invoice", len(sender.messages))
	}
}
