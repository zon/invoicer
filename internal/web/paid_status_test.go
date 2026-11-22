package web_test

import (
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zon/invoicer/internal/invoice"
	"github.com/zon/invoicer/internal/store"
)

// TestPaidStatusFlowWithoutAccessSettings changes the paid status through the
// web app and the real store, then checks the paid lock. Without access
// settings the app treats the request as an admin user.
func TestPaidStatusFlowWithoutAccessSettings(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "invoices.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	id, _, err := s.Save(&invoice.Invoice{
		Month:    time.October,
		Year:     2027,
		Vendor:   "Jane Contractor",
		Customer: "Acme Corp",
		Project:  "Website Redesign",
		Rate:     100,
		Weeks:    []invoice.Week{{Hours: 10}, {Hours: 20}},
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	assertStatus(t, s, "Unpaid")

	recorder := post(t, s, "/2027/oct/paid", url.Values{"paid": {"true"}})
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("mark paid status: got %d, want %d", recorder.Code, http.StatusSeeOther)
	}
	if location := recorder.Header().Get("Location"); location != "/2027/oct" {
		t.Errorf("mark paid location: got %q, want %q", location, "/2027/oct")
	}
	assertStatus(t, s, "Paid")

	got, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Project != "Website Redesign" {
		t.Errorf("Project: got %q, want %q", got.Project, "Website Redesign")
	}
	if len(got.Weeks) != 2 || got.Weeks[0].Hours != 10 || got.Weeks[1].Hours != 20 {
		t.Errorf("weeks changed: %+v", got.Weeks)
	}
	if got.Total() != 3000 {
		t.Errorf("Total: got %v, want 3000", got.Total())
	}

	recorder = post(t, s, "/2027/oct", url.Values{"project": {"Mobile App"}})
	if recorder.Code != http.StatusConflict {
		t.Fatalf("paid project edit status: got %d, want %d", recorder.Code, http.StatusConflict)
	}
	got, err = s.Get(id)
	if err != nil {
		t.Fatalf("Get after refused project edit: %v", err)
	}
	if got.Project != "Website Redesign" {
		t.Errorf("Project after refused edit: got %q, want %q", got.Project, "Website Redesign")
	}

	recorder = post(t, s, "/2027/oct/paid", url.Values{"paid": {"false"}})
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("mark unpaid status: got %d, want %d", recorder.Code, http.StatusSeeOther)
	}
	assertStatus(t, s, "Unpaid")

	recorder = post(t, s, "/2027/oct", url.Values{"project": {"Mobile App"}})
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("unpaid project edit status: got %d, want %d", recorder.Code, http.StatusSeeOther)
	}
	got, err = s.Get(id)
	if err != nil {
		t.Fatalf("Get after project edit: %v", err)
	}
	if got.Project != "Mobile App" {
		t.Errorf("Project after edit: got %q, want %q", got.Project, "Mobile App")
	}
}

func assertStatus(t *testing.T, s *store.Store, want string) {
	t.Helper()

	list := get(t, s, "/").Body.String()
	row := invoiceRowFor(t, list, "Acme Corp")
	if !strings.Contains(row, "<td>"+want+"</td>") {
		t.Errorf("list row does not show %q", want)
	}

	detail := get(t, s, "/2027/oct").Body.String()
	if !strings.Contains(detail, "<dd>"+want+"</dd>") {
		t.Errorf("detail does not show %q", want)
	}
}
