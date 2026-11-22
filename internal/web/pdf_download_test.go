package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/zon/invoicer/internal/invoice"
	"github.com/zon/invoicer/internal/store"
)

type downloadStore struct {
	invoices []*invoice.Invoice
}

func (s *downloadStore) List() ([]*invoice.Invoice, error) {
	return s.invoices, nil
}

func (s *downloadStore) GetByMonth(year int, month time.Month) (*invoice.Invoice, error) {
	for _, inv := range s.invoices {
		if inv.Year == year && inv.Month == month {
			return inv, nil
		}
	}
	return nil, store.ErrNotFound
}

func (s *downloadStore) SetProject(year int, month time.Month, project string) error {
	for _, inv := range s.invoices {
		if inv.Year == year && inv.Month == month {
			inv.Project = project
			return nil
		}
	}
	return store.ErrNotFound
}

func (s *downloadStore) SetPaid(year int, month time.Month, paid bool) error {
	for _, inv := range s.invoices {
		if inv.Year == year && inv.Month == month {
			inv.Paid = paid
			return nil
		}
	}
	return store.ErrNotFound
}

func download(t *testing.T, s Store, path string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	New(s).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder
}

func TestPDFDownloadServesStoredInvoice(t *testing.T) {
	s := &downloadStore{invoices: []*invoice.Invoice{testInvoice()}}

	recorder := download(t, s, "/2027/oct/pdf")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/pdf" {
		t.Errorf("content type: got %q, want application/pdf", contentType)
	}
	if !bytes.HasPrefix(recorder.Body.Bytes(), []byte("%PDF-")) {
		t.Fatal("response is not a PDF")
	}

	text, _ := readInvoicePDF(t, recorder.Body.Bytes())
	for _, want := range []string{"Jane Doe", "Acme Corp", "Website", "October", "2027", "Total: $10000.00"} {
		if !strings.Contains(text, want) {
			t.Errorf("PDF does not show %q", want)
		}
	}
}

func TestPDFDownloadNamesFileWithTwoDigitMonth(t *testing.T) {
	for _, tc := range []struct {
		name  string
		month time.Month
		year  int
		want  string
	}{
		{"October", time.October, 2027, `attachment; filename="invoice-2027-10.pdf"`},
		{"January", time.January, 2028, `attachment; filename="invoice-2028-01.pdf"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inv := testInvoice()
			inv.Month = tc.month
			inv.Year = tc.year
			s := &downloadStore{invoices: []*invoice.Invoice{inv}}

			recorder := download(t, s, detailPath(tc.year, tc.month)+"/pdf")

			if recorder.Code != http.StatusOK {
				t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
			}
			if disposition := recorder.Header().Get("Content-Disposition"); disposition != tc.want {
				t.Errorf("content disposition: got %q, want %q", disposition, tc.want)
			}
		})
	}
}

func TestPDFDownloadRendersLatestStoredValues(t *testing.T) {
	s := &downloadStore{invoices: []*invoice.Invoice{testInvoice()}}

	before, _ := readInvoicePDF(t, download(t, s, "/2027/oct/pdf").Body.Bytes())
	if !strings.Contains(before, "Website") {
		t.Fatal("first download does not show the stored project")
	}

	if err := s.SetProject(2027, time.October, "Mobile App"); err != nil {
		t.Fatalf("changing the project: %v", err)
	}

	after, _ := readInvoicePDF(t, download(t, s, "/2027/oct/pdf").Body.Bytes())
	if !strings.Contains(after, "Mobile App") {
		t.Error("download does not show the latest project")
	}
	if strings.Contains(after, "Website") {
		t.Error("download still shows the old project")
	}
}

func TestPDFDownloadChangesNothing(t *testing.T) {
	inv := testInvoice()
	want := *inv
	want.Weeks = append([]invoice.Week(nil), inv.Weeks...)
	s := &downloadStore{invoices: []*invoice.Invoice{inv}}

	if recorder := download(t, s, "/2027/oct/pdf"); recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}

	if !reflect.DeepEqual(inv, &want) {
		t.Errorf("invoice changed: got %+v, want %+v", inv, &want)
	}
}
