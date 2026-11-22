package web_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zon/invoicer/internal/invoice"
	"github.com/zon/invoicer/internal/store"
	"github.com/zon/invoicer/internal/web"
)

type fakeStore struct {
	invoices []*invoice.Invoice
	err      error
}

func (f *fakeStore) List() ([]*invoice.Invoice, error) {
	return f.invoices, f.err
}

func (f *fakeStore) GetByMonth(year int, month time.Month) (*invoice.Invoice, error) {
	for _, inv := range f.invoices {
		if inv.Year == year && inv.Month == month {
			return inv, nil
		}
	}
	return nil, store.ErrNotFound
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

func TestSelectingInvoiceShowsDetailBesideTheList(t *testing.T) {
	selected := &invoice.Invoice{
		Month:    time.October,
		Year:     2027,
		Vendor:   "Jane Smith",
		Customer: "Acme Corp",
		Project:  "Website Redesign",
		Rate:     100,
		Weeks: []invoice.Week{
			{
				Start: time.Date(2027, time.October, 4, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2027, time.October, 10, 0, 0, 0, 0, time.UTC),
				Hours: 40,
			},
			{
				Start: time.Date(2027, time.October, 11, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2027, time.October, 17, 0, 0, 0, 0, time.UTC),
				Hours: 32,
			},
		},
	}
	other := &invoice.Invoice{
		Month:    time.November,
		Year:     2027,
		Customer: "Other Customer",
		Project:  "Other Project",
		Rate:     50,
		Weeks:    []invoice.Week{{Hours: 10}},
	}

	recorder := get(t, &fakeStore{invoices: []*invoice.Invoice{selected, other}}, "/2027/oct")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}

	body := recorder.Body.String()
	if !strings.HasPrefix(body, "<!doctype html>") {
		t.Error("body is not a complete page")
	}
	if !strings.Contains(body, "</html>") {
		t.Error("body does not close the page document")
	}
	for _, want := range []string{
		`id="detail"`,
		"Jane Smith", "Acme Corp", "Website Redesign", "October", "2027", "$100.00",
		"Oct 4-10", "40", "Oct 11-17", "32", "$7200.00",
		"Other Customer", "Other Project", `href="/2027/nov"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body does not contain %q", want)
		}
	}
}

func TestDetailReturnsNotFound(t *testing.T) {
	stored := &invoice.Invoice{
		Month:    time.October,
		Year:     2027,
		Customer: "Acme Corp",
		Rate:     100,
	}

	for _, path := range []string{
		"/2026/oct",
		"/2027/sep",
		"/2027/foo",
		"/2027/13",
		"/not-a-year/oct",
	} {
		t.Run(path, func(t *testing.T) {
			recorder := get(t, &fakeStore{invoices: []*invoice.Invoice{stored}}, path)

			if recorder.Code != http.StatusNotFound {
				t.Errorf("status: got %d, want %d", recorder.Code, http.StatusNotFound)
			}
		})
	}
}

func TestDetailOmitsUnsetProject(t *testing.T) {
	recorder := get(t, &fakeStore{invoices: []*invoice.Invoice{{
		Month:    time.March,
		Year:     2027,
		Vendor:   "Jane Smith",
		Customer: "Acme Corp",
		Rate:     150,
		Weeks: []invoice.Week{{
			Start: time.Date(2027, time.March, 1, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2027, time.March, 7, 0, 0, 0, 0, time.UTC),
			Hours: 40,
		}},
	}}}, "/2027/mar")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}

	detail := detailPanel(t, recorder.Body.String())
	if !strings.Contains(detail, "Acme Corp") {
		t.Error("detail does not contain the customer")
	}
	if strings.Contains(detail, "Project") {
		t.Error("detail contains a project label that is not set")
	}
}

func detailPanel(t *testing.T, body string) string {
	t.Helper()
	start := strings.Index(body, `id="detail"`)
	if start < 0 {
		t.Fatal("body does not contain a detail panel")
	}
	end := strings.Index(body[start:], "</aside>")
	if end < 0 {
		t.Fatal("detail panel is not closed")
	}
	return body[start : start+end]
}
