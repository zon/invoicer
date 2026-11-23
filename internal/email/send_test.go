package email

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type sentEmail struct {
	From        string   `json:"from"`
	To          []string `json:"to"`
	Subject     string   `json:"subject"`
	Text        string   `json:"text"`
	Html        string   `json:"html"`
	Attachments []any    `json:"attachments"`
}

type capturedSend struct {
	method        string
	path          string
	authorization string
	contentType   string
	idempotency   string
	email         sentEmail
}

func testMessage() Message {
	return Message{
		To:      "billing@acme.com",
		From:    "invoicer@example.com",
		Subject: "Invoice for October 2026",
		Text:    "Invoice A1B2C3 for October 2026 from Jane Smith.\n\nTotal: $4800.00",
		HTML:    "<p>Invoice A1B2C3 for October 2026 from Jane Smith.</p>\n<p>Total: $4800.00</p>",
	}
}

func TestSendSendsMessageAndReturnsIdentifier(t *testing.T) {
	requests := make(chan capturedSend, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var email sentEmail
		if err := json.NewDecoder(r.Body).Decode(&email); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		requests <- capturedSend{
			method:        r.Method,
			path:          r.URL.Path,
			authorization: r.Header.Get("Authorization"),
			contentType:   r.Header.Get("Content-Type"),
			idempotency:   r.Header.Get("Idempotency-Key"),
			email:         email,
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"email_123"}`)
	}))
	defer server.Close()

	sender := newFileSender(t, server.URL, "re_test_key")

	id, err := sender.Send(testMessage(), "a1b2c3")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if id != "email_123" {
		t.Errorf("identifier: got %q, want %q", id, "email_123")
	}

	got := <-requests
	if got.method != http.MethodPost {
		t.Errorf("method: got %q, want POST", got.method)
	}
	if got.path != "/emails" {
		t.Errorf("path: got %q, want %q", got.path, "/emails")
	}
	if got.authorization != "Bearer re_test_key" {
		t.Errorf("Authorization: got %q, want the API key from the file", got.authorization)
	}
	if !strings.HasPrefix(got.contentType, "application/json") {
		t.Errorf("Content-Type: got %q, want JSON", got.contentType)
	}
	if got.idempotency != "invoice/a1b2c3" {
		t.Errorf("Idempotency-Key: got %q, want %q", got.idempotency, "invoice/a1b2c3")
	}

	if got.email.From != "invoicer@example.com" {
		t.Errorf("From: got %q, want the sender", got.email.From)
	}
	if len(got.email.To) != 1 || got.email.To[0] != "billing@acme.com" {
		t.Errorf("To: got %v, want the one contact address", got.email.To)
	}
	if got.email.Subject != "Invoice for October 2026" {
		t.Errorf("Subject: got %q, want the invoice subject", got.email.Subject)
	}
	if got.email.Text != testMessage().Text {
		t.Errorf("Text: got %q, want the rendered text", got.email.Text)
	}
	if got.email.Html != testMessage().HTML {
		t.Errorf("Html: got %q, want the rendered HTML", got.email.Html)
	}
	if got.email.Attachments != nil {
		t.Errorf("Attachments: got %v, want none", got.email.Attachments)
	}
}

func TestSendReportsRejectedMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprint(w, `{"statusCode":422,"name":"validation_error","message":"invalid from address"}`)
	}))
	defer server.Close()

	sender := newFileSender(t, server.URL, "re_test_key")

	id, err := sender.Send(testMessage(), "a1b2c3")
	if err == nil {
		t.Fatal("expected an error for the rejected message")
	}
	if id != "" {
		t.Errorf("identifier: got %q, want none", id)
	}
	if !strings.Contains(err.Error(), "invalid from address") {
		t.Errorf("error: got %q, want the Resend rejection", err)
	}
}

func TestSendGivesUpAfterTimeout(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer server.Close()
	defer close(release)

	sender := newFileSender(t, server.URL, "re_test_key")
	sender.timeout = 50 * time.Millisecond

	id, err := sender.Send(testMessage(), "a1b2c3")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Send error: got %v, want a timeout", err)
	}
	if id != "" {
		t.Errorf("identifier: got %q, want none", id)
	}
}

func TestNewSenderReportsUnreadableAPIKeyFile(t *testing.T) {
	cases := []struct {
		name string
		path func(t *testing.T) string
	}{
		{
			name: "missing file",
			path: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "missing")
			},
		},
		{
			name: "directory",
			path: func(t *testing.T) string {
				return t.TempDir()
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := tc.path(t)
			if _, err := NewSender(path); err == nil {
				t.Fatal("expected an error for an unreadable API key file")
			} else if !strings.Contains(err.Error(), path) {
				t.Errorf("error: got %q, want it to name %q", err, path)
			}
		})
	}
}

func newFileSender(t *testing.T, serverURL, apiKey string) *Sender {
	t.Helper()

	keyFile := filepath.Join(t.TempDir(), "resend-api-key")
	if err := os.WriteFile(keyFile, []byte(apiKey+"\n"), 0o600); err != nil {
		t.Fatalf("writing API key file: %v", err)
	}

	sender, err := NewSender(keyFile)
	if err != nil {
		t.Fatalf("NewSender: %v", err)
	}

	base, err := url.Parse(serverURL + "/")
	if err != nil {
		t.Fatalf("parsing stub URL: %v", err)
	}
	sender.client.BaseURL = base
	return sender
}
