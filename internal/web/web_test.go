package web_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zon/invoicer/internal/invoice"
	"github.com/zon/invoicer/internal/web"
)

type fakeStore struct {
	invoices []*invoice.Invoice
	err      error
}

func (f *fakeStore) List() ([]*invoice.Invoice, error) {
	return f.invoices, f.err
}

func get(t *testing.T, s web.Store, path string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	web.New(s).ServeHTTP(recorder, request)
	return recorder
}

func TestListShowsEveryInvoiceNewestFirst(t *testing.T) {
	older := &invoice.Invoice{
		Month:    time.October,
		Year:     2027,
		Customer: "Older Customer",
		Project:  "Older Project",
		Rate:     100,
		Weeks:    []invoice.Week{{Hours: 10}},
	}
	newer := &invoice.Invoice{
		Month:    time.January,
		Year:     2028,
		Customer: "Newer Customer",
		Project:  "Newer Project",
		Rate:     100,
		Weeks:    []invoice.Week{{Hours: 20}},
	}
	newest := &invoice.Invoice{
		Month:    time.December,
		Year:     2028,
		Customer: "Newest Customer",
		Project:  "Newest Project",
		Rate:     100,
		Weeks:    []invoice.Week{{Hours: 30}},
	}

	recorder := get(t, &fakeStore{invoices: []*invoice.Invoice{older, newer, newest}}, "/")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}

	body := recorder.Body.String()
	for _, want := range []string{
		"Older Customer", "October", "2027", "Older Project", "$1000.00",
		"Newer Customer", "January", "2028", "Newer Project", "$2000.00",
		"Newest Customer", "December", "2028", "Newest Project", "$3000.00",
		`href="/2027/oct"`, `href="/2028/jan"`, `href="/2028/dec"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body does not contain %q", want)
		}
	}

	newestAt := strings.Index(body, "Newest Customer")
	newerAt := strings.Index(body, "Newer Customer")
	olderAt := strings.Index(body, "Older Customer")
	if newestAt > newerAt || newerAt > olderAt {
		t.Errorf("invoices are not newest first: newest %d, newer %d, older %d", newestAt, newerAt, olderAt)
	}
}

func TestListOmitsUnsetProject(t *testing.T) {
	recorder := get(t, &fakeStore{invoices: []*invoice.Invoice{{
		Month:    time.March,
		Year:     2027,
		Customer: "Acme Corp",
		Rate:     150,
	}}}, "/")

	body := recorder.Body.String()
	if !strings.Contains(body, "Acme Corp") {
		t.Fatal("body does not contain the customer")
	}
	row := body[strings.Index(body, "Acme Corp"):]
	if strings.Contains(row, "Website Redesign") {
		t.Error("body contains a project that is not set")
	}
}

func TestListShowsEmptyList(t *testing.T) {
	recorder := get(t, &fakeStore{}, "/")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), "No invoices.") {
		t.Error("body does not show an empty list")
	}
}

func TestListReportsStoreError(t *testing.T) {
	recorder := get(t, &fakeStore{err: errors.New("database unavailable")}, "/")

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}
