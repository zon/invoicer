package web_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/zon/invoicer/internal/access"
	"github.com/zon/invoicer/internal/invoice"
)

func pdfAccessInvoice(paid bool) *invoice.Invoice {
	return &invoice.Invoice{
		Month:    time.October,
		Year:     2027,
		Vendor:   "Jane Contractor",
		Customer: "Acme Corp",
		Project:  "Website Redesign",
		Rate:     100,
		Paid:     paid,
		Weeks:    []invoice.Week{{Hours: 10}, {Hours: 20}},
	}
}

func assertPDFDownload(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/pdf" {
		t.Errorf("content type: got %q, want application/pdf", contentType)
	}
	if !bytes.HasPrefix(recorder.Body.Bytes(), []byte("%PDF-")) {
		t.Error("response is not a PDF")
	}
}

func TestSignedInAdminAndCustomerDownloadPDF(t *testing.T) {
	for _, role := range []struct {
		name string
		role access.Role
	}{
		{"admin", access.Admin},
		{"customer", access.Customer},
	} {
		t.Run(role.name, func(t *testing.T) {
			s := &fakeStore{invoices: []*invoice.Invoice{pdfAccessInvoice(false)}}

			recorder := getWithAccess(t, s, signedInAs(role.name, role.role), "/2027/oct/pdf")

			assertPDFDownload(t, recorder)
		})
	}
}

func TestPaidInvoiceDownloadsPDF(t *testing.T) {
	for _, role := range []struct {
		name string
		role access.Role
	}{
		{"admin", access.Admin},
		{"customer", access.Customer},
	} {
		t.Run(role.name, func(t *testing.T) {
			s := &fakeStore{invoices: []*invoice.Invoice{pdfAccessInvoice(true)}}

			recorder := getWithAccess(t, s, signedInAs(role.name, role.role), "/2027/oct/pdf")

			assertPDFDownload(t, recorder)
		})
	}
}

func TestPDFDownloadWithoutSessionGoesToLogin(t *testing.T) {
	s := &fakeStore{invoices: []*invoice.Invoice{pdfAccessInvoice(false)}}

	recorder := getWithAccess(t, s, &fakeAccess{}, "/2027/oct/pdf")

	if recorder.Code != http.StatusFound {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusFound)
	}
	location, err := url.Parse(recorder.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parsing redirect: %v", err)
	}
	if location.Path != "/login" {
		t.Errorf("redirect path: got %q, want %q", location.Path, "/login")
	}
	if next := location.Query().Get("next"); next != "/2027/oct/pdf" {
		t.Errorf("return address: got %q, want %q", next, "/2027/oct/pdf")
	}
	if bytes.HasPrefix(recorder.Body.Bytes(), []byte("%PDF-")) {
		t.Error("response holds a PDF")
	}
}

func TestPDFDownloadWithoutAccessSettings(t *testing.T) {
	s := &fakeStore{invoices: []*invoice.Invoice{pdfAccessInvoice(false)}}

	recorder := get(t, s, "/2027/oct/pdf")

	assertPDFDownload(t, recorder)
}
