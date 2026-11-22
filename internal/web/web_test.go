package web_test

import (
	"bytes"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/zon/invoicer/internal/access"
	"github.com/zon/invoicer/internal/invoice"
	"github.com/zon/invoicer/internal/store"
	"github.com/zon/invoicer/internal/web"
)

type fakeAccess struct {
	user     *access.User
	login    bool
	callback bool
	logout   bool
}

func (f *fakeAccess) CurrentUser(*gin.Context) (*access.User, bool) {
	return f.user, f.user != nil
}

func (f *fakeAccess) Login(c *gin.Context) {
	f.login = true
	c.Status(http.StatusNoContent)
}

func (f *fakeAccess) Callback(c *gin.Context) {
	f.callback = true
	c.Status(http.StatusNoContent)
}

func (f *fakeAccess) Logout(c *gin.Context) {
	f.logout = true
	c.Status(http.StatusNoContent)
}

type fakeStore struct {
	invoices []*invoice.Invoice
	err      error
	setErr   error
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

func (f *fakeStore) SetProject(year int, month time.Month, project string) error {
	if f.setErr != nil {
		return f.setErr
	}
	for _, inv := range f.invoices {
		if inv.Year == year && inv.Month == month {
			if inv.Paid {
				return store.ErrPaid
			}
			inv.Project = project
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *fakeStore) SetPaid(year int, month time.Month, paid bool) error {
	if f.setErr != nil {
		return f.setErr
	}
	for _, inv := range f.invoices {
		if inv.Year == year && inv.Month == month {
			inv.Paid = paid
			return nil
		}
	}
	return store.ErrNotFound
}

func TestNewDoesNotLogRoutesByDefault(t *testing.T) {
	t.Setenv(gin.EnvGinMode, "")
	gin.SetMode(gin.DebugMode)

	var out bytes.Buffer
	originalWriter := gin.DefaultWriter
	gin.DefaultWriter = &out
	defer func() {
		gin.DefaultWriter = originalWriter
		gin.SetMode(gin.ReleaseMode)
	}()

	web.New(&fakeStore{})

	if out.Len() > 0 {
		t.Errorf("New wrote startup logs: %q", out.String())
	}
}

func TestNewRegistersAccessRoutes(t *testing.T) {
	auth := &fakeAccess{}
	server := web.New(&fakeStore{}, auth)

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/login", nil))
	if !auth.login {
		t.Error("GET /login did not reach the access component")
	}

	recorder = httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/callback", nil))
	if !auth.callback {
		t.Error("GET /callback did not reach the access component")
	}

	recorder = httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/logout", nil))
	if !auth.logout {
		t.Error("GET /logout did not reach the access component")
	}
}

func get(t *testing.T, s web.Store, path string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	web.New(s).ServeHTTP(recorder, req)
	return recorder
}

func getWithAccess(t *testing.T, s web.Store, a web.Access, path string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	web.New(s, a).ServeHTTP(recorder, req)
	return recorder
}

func post(t *testing.T, s web.Store, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	web.New(s).ServeHTTP(recorder, req)
	return recorder
}

func postWithAccess(t *testing.T, s web.Store, a web.Access, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	web.New(s, a).ServeHTTP(recorder, req)
	return recorder
}

func TestRequestsWithoutSessionGoToLogin(t *testing.T) {
	stored := &invoice.Invoice{
		Month:    time.October,
		Year:     2027,
		Customer: "Acme Corp",
		Rate:     100,
	}

	for _, path := range []string{"/", "/2027/oct", "/style.css", "/unknown"} {
		t.Run(path, func(t *testing.T) {
			recorder := getWithAccess(t, &fakeStore{invoices: []*invoice.Invoice{stored}}, &fakeAccess{}, path)

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
			if next := location.Query().Get("next"); next != path {
				t.Errorf("return page: got %q, want %q", next, path)
			}
		})
	}
}

func TestStylesheetRequiresSession(t *testing.T) {
	recorder := getWithAccess(t, &fakeStore{}, &fakeAccess{}, "/style.css")

	if recorder.Code != http.StatusFound {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusFound)
	}
	if location := recorder.Header().Get("Location"); location != "/login?next=%2Fstyle.css" {
		t.Errorf("location: got %q, want %q", location, "/login?next=%2Fstyle.css")
	}
}

func TestSignedInUserLoadsStylesheet(t *testing.T) {
	user := &access.User{
		Email:   "jane@example.com",
		Role:    access.Customer,
		Expires: time.Now().Add(time.Hour),
	}

	recorder := getWithAccess(t, &fakeStore{}, &fakeAccess{user: user}, "/style.css")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "text/css") {
		t.Errorf("content type: got %q, want text/css", contentType)
	}
	if !strings.Contains(recorder.Body.String(), ".invoice-detail") {
		t.Error("body does not contain the invoice detail style")
	}
}

func TestSignedInUserReadsInvoices(t *testing.T) {
	user := &access.User{
		Email:   "jane@example.com",
		Role:    access.Customer,
		Expires: time.Now().Add(time.Hour),
	}
	stored := &invoice.Invoice{
		Month:    time.October,
		Year:     2027,
		Customer: "Acme Corp",
		Rate:     100,
	}

	for _, path := range []string{"/", "/2027/oct"} {
		t.Run(path, func(t *testing.T) {
			recorder := getWithAccess(t, &fakeStore{invoices: []*invoice.Invoice{stored}}, &fakeAccess{user: user}, path)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
			}
			if !strings.Contains(recorder.Body.String(), "Acme Corp") {
				t.Error("body does not show the invoice")
			}
		})
	}
}

func TestAdminAndCustomerSeeEveryInvoice(t *testing.T) {
	entries := []struct {
		path    string
		invoice *invoice.Invoice
	}{
		{
			path: "/2027/oct",
			invoice: &invoice.Invoice{
				Month:    time.October,
				Year:     2027,
				Customer: "October Customer",
				Project:  "October Project",
				Rate:     100,
				Weeks:    []invoice.Week{{Hours: 10}},
			},
		},
		{
			path: "/2028/jan",
			invoice: &invoice.Invoice{
				Month:    time.January,
				Year:     2028,
				Customer: "January Customer",
				Project:  "January Project",
				Rate:     100,
				Weeks:    []invoice.Week{{Hours: 20}},
			},
		},
		{
			path: "/2028/dec",
			invoice: &invoice.Invoice{
				Month:    time.December,
				Year:     2028,
				Customer: "December Customer",
				Project:  "December Project",
				Rate:     100,
				Weeks:    []invoice.Week{{Hours: 30}},
			},
		},
	}

	invoices := make([]*invoice.Invoice, 0, len(entries))
	for _, entry := range entries {
		invoices = append(invoices, entry.invoice)
	}

	for _, role := range []struct {
		name string
		role access.Role
	}{
		{"admin", access.Admin},
		{"customer", access.Customer},
	} {
		t.Run(role.name, func(t *testing.T) {
			user := &access.User{
				Email:   role.name + "@example.com",
				Role:    role.role,
				Expires: time.Now().Add(time.Hour),
			}

			list := getWithAccess(t, &fakeStore{invoices: invoices}, &fakeAccess{user: user}, "/")
			if list.Code != http.StatusOK {
				t.Fatalf("list status: got %d, want %d", list.Code, http.StatusOK)
			}
			for _, entry := range entries {
				if !strings.Contains(list.Body.String(), entry.invoice.Customer) {
					t.Errorf("list does not show %q", entry.invoice.Customer)
				}
			}

			for _, entry := range entries {
				detail := getWithAccess(t, &fakeStore{invoices: invoices}, &fakeAccess{user: user}, entry.path)
				if detail.Code != http.StatusOK {
					t.Fatalf("%s status: got %d, want %d", entry.path, detail.Code, http.StatusOK)
				}
				body := detail.Body.String()
				if !strings.Contains(body, entry.invoice.Customer) {
					t.Errorf("%s does not show %q", entry.path, entry.invoice.Customer)
				}
				for _, other := range entries {
					if other.path != entry.path && strings.Contains(body, other.invoice.Customer) {
						t.Errorf("%s shows another invoice %q", entry.path, other.invoice.Customer)
					}
				}
			}
		})
	}
}

func TestSignedInUnknownInvoiceReturnsNotFound(t *testing.T) {
	stored := &invoice.Invoice{
		Month:    time.October,
		Year:     2027,
		Customer: "Acme Corp",
		Rate:     100,
	}

	for _, role := range []struct {
		name string
		role access.Role
	}{
		{"admin", access.Admin},
		{"customer", access.Customer},
	} {
		t.Run(role.name, func(t *testing.T) {
			user := &access.User{
				Email:   role.name + "@example.com",
				Role:    role.role,
				Expires: time.Now().Add(time.Hour),
			}

			for _, path := range []string{
				"/2026/oct",
				"/2027/sep",
				"/2027/foo",
				"/2027/13",
				"/not-a-year/oct",
			} {
				t.Run(path, func(t *testing.T) {
					recorder := getWithAccess(t, &fakeStore{invoices: []*invoice.Invoice{stored}}, &fakeAccess{user: user}, path)

					if recorder.Code != http.StatusNotFound {
						t.Errorf("status: got %d, want %d", recorder.Code, http.StatusNotFound)
					}
				})
			}
		})
	}
}

func TestSignedInPagesShowEmailAndSignOut(t *testing.T) {
	user := &access.User{
		Email:   "jane@example.com",
		Role:    access.Customer,
		Expires: time.Now().Add(time.Hour),
	}
	stored := &invoice.Invoice{
		Month:    time.October,
		Year:     2027,
		Customer: "Acme Corp",
		Rate:     100,
	}

	for _, path := range []string{"/", "/2027/oct"} {
		t.Run(path, func(t *testing.T) {
			recorder := getWithAccess(t, &fakeStore{invoices: []*invoice.Invoice{stored}}, &fakeAccess{user: user}, path)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
			}

			body := recorder.Body.String()
			for _, want := range []string{"jane@example.com", `href="/logout"`, "Sign out"} {
				if !strings.Contains(body, want) {
					t.Errorf("body does not contain %q", want)
				}
			}
		})
	}
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

func TestListShowsPaidStatus(t *testing.T) {
	for _, tc := range []struct {
		name string
		paid bool
		want string
	}{
		{"paid", true, "Paid"},
		{"unpaid", false, "Unpaid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stored := &invoice.Invoice{
				Month:    time.October,
				Year:     2027,
				Customer: "Acme Corp",
				Rate:     100,
				Paid:     tc.paid,
			}
			s := &fakeStore{invoices: []*invoice.Invoice{stored}}

			list := get(t, s, "/").Body.String()
			if !strings.Contains(list, "<th>Status</th>") {
				t.Fatal("list does not have a Status column")
			}
			row := invoiceRowFor(t, list, "Acme Corp")
			if !strings.Contains(row, "<td>"+tc.want+"</td>") {
				t.Errorf("list row does not show %q", tc.want)
			}

			detail := get(t, s, "/2027/oct").Body.String()
			if !strings.Contains(detail, "<dd>"+tc.want+"</dd>") {
				t.Errorf("detail does not show %q", tc.want)
			}
		})
	}
}

func invoiceRowFor(t *testing.T, body, customer string) string {
	t.Helper()

	at := strings.Index(body, customer)
	if at < 0 {
		t.Fatalf("body does not show %q", customer)
	}
	start := strings.LastIndex(body[:at], "<tr>")
	end := strings.Index(body[at:], "</tr>")
	if start < 0 || end < 0 {
		t.Fatalf("body does not contain a row for %q", customer)
	}
	return body[start : at+end]
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

func TestDetailShowsFullPageWithBackLink(t *testing.T) {
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
	if !strings.Contains(body, `href="/"`) || !strings.Contains(body, "Back to invoices") {
		t.Error("page does not link back to the invoice list")
	}
	if strings.Contains(body, "Other Customer") {
		t.Error("detail page lists another invoice")
	}
	for _, want := range []string{
		"Jane Smith", "Acme Corp", "Website Redesign", "October", "2027", "$100.00",
		"Oct 4-10", "40", "Oct 11-17", "32", "$7200.00",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body does not contain %q", want)
		}
	}
}

func TestDetailLinksToInvoicePDF(t *testing.T) {
	for _, tc := range []struct {
		name  string
		month time.Month
		year  int
		path  string
	}{
		{"October", time.October, 2027, "/2027/oct"},
		{"January", time.January, 2028, "/2028/jan"},
	} {
		for _, paid := range []bool{false, true} {
			status := "unpaid"
			if paid {
				status = "paid"
			}
			t.Run(tc.name+" "+status, func(t *testing.T) {
				stored := &invoice.Invoice{
					Month:    tc.month,
					Year:     tc.year,
					Customer: "Acme Corp",
					Rate:     100,
					Paid:     paid,
				}

				recorder := get(t, &fakeStore{invoices: []*invoice.Invoice{stored}}, tc.path)
				if recorder.Code != http.StatusOK {
					t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
				}

				body := recorder.Body.String()
				if !strings.Contains(body, `href="`+tc.path+`/pdf"`) {
					t.Errorf("detail does not link to %q", tc.path+"/pdf")
				}
				if !strings.Contains(body, "Download PDF") {
					t.Error("detail does not label the PDF link")
				}
			})
		}
	}
}

var (
	formPattern  = regexp.MustCompile(`(?s)<form\b.*?</form>`)
	inputPattern = regexp.MustCompile(`<input\b[^>]*>`)
)

func projectForm(t *testing.T, body string) string {
	t.Helper()

	for _, form := range formPattern.FindAllString(body, -1) {
		if strings.Contains(form, `name="project"`) {
			return form
		}
	}
	t.Fatal("detail does not contain a project form")
	return ""
}

func projectInput(t *testing.T, form string) string {
	t.Helper()

	input := inputPattern.FindString(form)
	if input == "" {
		t.Fatal("project form does not contain an input")
	}
	return input
}

func paidForm(t *testing.T, body string) string {
	t.Helper()

	for _, form := range formPattern.FindAllString(body, -1) {
		if strings.Contains(form, `name="paid"`) {
			return form
		}
	}
	t.Fatal("detail does not contain a paid status form")
	return ""
}

func TestDetailShowsPaidStatus(t *testing.T) {
	for _, tc := range []struct {
		name string
		paid bool
		want string
	}{
		{"paid", true, "Paid"},
		{"unpaid", false, "Unpaid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stored := &invoice.Invoice{
				Month:    time.October,
				Year:     2027,
				Customer: "Acme Corp",
				Rate:     100,
				Paid:     tc.paid,
			}

			recorder := get(t, &fakeStore{invoices: []*invoice.Invoice{stored}}, "/2027/oct")
			if recorder.Code != http.StatusOK {
				t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
			}

			body := recorder.Body.String()
			if !strings.Contains(body, "<dt>Status</dt>") {
				t.Error("detail does not have a status entry")
			}
			if !strings.Contains(body, "<dd>"+tc.want+"</dd>") {
				t.Errorf("detail does not show %q", tc.want)
			}
		})
	}
}

func TestAdminSeesPaidStatusForm(t *testing.T) {
	for _, tc := range []struct {
		name   string
		paid   bool
		value  string
		button string
	}{
		{"unpaid", false, "true", "Mark paid"},
		{"paid", true, "false", "Mark unpaid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stored := &invoice.Invoice{
				Month:    time.October,
				Year:     2027,
				Customer: "Acme Corp",
				Rate:     100,
				Paid:     tc.paid,
			}

			recorder := getWithAccess(
				t,
				&fakeStore{invoices: []*invoice.Invoice{stored}},
				signedInAs("admin", access.Admin),
				"/2027/oct",
			)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
			}

			form := paidForm(t, recorder.Body.String())
			opening := form[:strings.Index(form, ">")+1]
			for _, want := range []string{`method="post"`, `action="/2027/oct/paid"`} {
				if !strings.Contains(opening, want) {
					t.Errorf("form does not contain %q", want)
				}
			}

			if count := strings.Count(form, `name="paid"`); count != 1 {
				t.Errorf("form posts %d paid fields, want 1", count)
			}
			if !strings.Contains(form, `value="`+tc.value+`"`) {
				t.Errorf("form does not post the value %q", tc.value)
			}
			if !strings.Contains(form, tc.button) {
				t.Errorf("form does not have a %q button", tc.button)
			}
		})
	}
}

func TestCustomerDoesNotSeePaidStatusForm(t *testing.T) {
	for _, tc := range []struct {
		name string
		paid bool
	}{
		{"paid", true},
		{"unpaid", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stored := &invoice.Invoice{
				Month:    time.October,
				Year:     2027,
				Customer: "Acme Corp",
				Rate:     100,
				Paid:     tc.paid,
			}

			recorder := getWithAccess(
				t,
				&fakeStore{invoices: []*invoice.Invoice{stored}},
				signedInAs("customer", access.Customer),
				"/2027/oct",
			)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
			}

			body := recorder.Body.String()
			if !strings.Contains(body, "<dt>Status</dt>") {
				t.Error("detail does not show the status")
			}
			if strings.Contains(body, `name="paid"`) {
				t.Error("customer detail shows the paid status form")
			}
		})
	}
}

func TestPaidStatusFormShowsWithoutAccessSettings(t *testing.T) {
	stored := &invoice.Invoice{
		Month:    time.October,
		Year:     2027,
		Customer: "Acme Corp",
		Rate:     100,
	}

	recorder := get(t, &fakeStore{invoices: []*invoice.Invoice{stored}}, "/2027/oct")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}

	form := paidForm(t, recorder.Body.String())
	if !strings.Contains(form, "Mark paid") {
		t.Error("form does not have a Mark paid button")
	}
}

func TestPostPaidChangesStatus(t *testing.T) {
	for _, tc := range []struct {
		name  string
		paid  bool
		value string
		want  bool
	}{
		{"mark paid", false, "true", true},
		{"mark unpaid", true, "false", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stored := &invoice.Invoice{
				Month:    time.October,
				Year:     2027,
				Customer: "Acme Corp",
				Project:  "Website Redesign",
				Rate:     100,
				Paid:     tc.paid,
				Weeks:    []invoice.Week{{Hours: 10}},
			}
			s := &fakeStore{invoices: []*invoice.Invoice{stored}}
			auth := signedInAs("admin", access.Admin)

			recorder := postWithAccess(t, s, auth, "/2027/oct/paid", url.Values{"paid": {tc.value}})
			if recorder.Code != http.StatusSeeOther {
				t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusSeeOther)
			}
			if location := recorder.Header().Get("Location"); location != "/2027/oct" {
				t.Errorf("location: got %q, want %q", location, "/2027/oct")
			}
			if stored.Paid != tc.want {
				t.Errorf("Paid: got %t, want %t", stored.Paid, tc.want)
			}
			if stored.Project != "Website Redesign" {
				t.Errorf("Project: got %q, want %q", stored.Project, "Website Redesign")
			}
			if len(stored.Weeks) != 1 || stored.Weeks[0].Hours != 10 {
				t.Errorf("weeks changed: %+v", stored.Weeks)
			}

			body := getWithAccess(t, s, auth, "/2027/oct").Body.String()
			want := "Unpaid"
			if tc.want {
				want = "Paid"
			}
			if !strings.Contains(body, "<dd>"+want+"</dd>") {
				t.Errorf("detail does not show %q", want)
			}
		})
	}
}

func TestPostPaidReturnsNotFound(t *testing.T) {
	for _, path := range []string{
		"/2026/oct/paid",
		"/2027/sep/paid",
		"/2027/foo/paid",
		"/2027/13/paid",
		"/not-a-year/oct/paid",
	} {
		t.Run(path, func(t *testing.T) {
			stored := &invoice.Invoice{
				Month:    time.October,
				Year:     2027,
				Customer: "Acme Corp",
				Rate:     100,
			}
			s := &fakeStore{invoices: []*invoice.Invoice{stored}}

			recorder := post(t, s, path, url.Values{"paid": {"true"}})
			if recorder.Code != http.StatusNotFound {
				t.Errorf("status: got %d, want %d", recorder.Code, http.StatusNotFound)
			}
			if stored.Paid {
				t.Error("invoice is paid after a not-found response")
			}
		})
	}
}

func TestPostPaidRejectsInvalidValue(t *testing.T) {
	for _, tc := range []struct {
		name string
		form url.Values
	}{
		{"missing", url.Values{}},
		{"empty", url.Values{"paid": {""}}},
		{"other", url.Values{"paid": {"yes"}}},
		{"number", url.Values{"paid": {"1"}}},
		{"uppercase", url.Values{"paid": {"TRUE"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stored := &invoice.Invoice{
				Month:    time.October,
				Year:     2027,
				Customer: "Acme Corp",
				Rate:     100,
			}
			s := &fakeStore{invoices: []*invoice.Invoice{stored}}

			recorder := post(t, s, "/2027/oct/paid", tc.form)
			if recorder.Code != http.StatusBadRequest {
				t.Errorf("status: got %d, want %d", recorder.Code, http.StatusBadRequest)
			}
			if stored.Paid {
				t.Error("invoice is paid after an invalid value")
			}
		})
	}
}

func TestPostPaidReportsSaveFailure(t *testing.T) {
	for _, tc := range []struct {
		name  string
		paid  bool
		value string
	}{
		{"mark paid", false, "true"},
		{"mark unpaid", true, "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stored := &invoice.Invoice{
				Month:    time.October,
				Year:     2027,
				Customer: "Acme Corp",
				Rate:     100,
				Paid:     tc.paid,
			}
			s := &fakeStore{
				invoices: []*invoice.Invoice{stored},
				setErr:   errors.New("database unavailable"),
			}

			recorder := post(t, s, "/2027/oct/paid", url.Values{"paid": {tc.value}})

			if recorder.Code != http.StatusInternalServerError {
				t.Errorf("status: got %d, want %d", recorder.Code, http.StatusInternalServerError)
			}
			if stored.Paid != tc.paid {
				t.Errorf("Paid: got %t, want %t", stored.Paid, tc.paid)
			}
		})
	}
}

func TestCustomerPostPaidReturnsForbidden(t *testing.T) {
	for _, tc := range []struct {
		name  string
		paid  bool
		value string
	}{
		{"mark paid", false, "true"},
		{"mark unpaid", true, "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stored := &invoice.Invoice{
				Month:    time.October,
				Year:     2027,
				Customer: "Acme Corp",
				Project:  "Website Redesign",
				Rate:     100,
				Paid:     tc.paid,
				Weeks:    []invoice.Week{{Hours: 10}},
			}
			s := &fakeStore{invoices: []*invoice.Invoice{stored}}
			auth := signedInAs("customer", access.Customer)

			recorder := postWithAccess(t, s, auth, "/2027/oct/paid", url.Values{"paid": {tc.value}})
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusForbidden)
			}
			if stored.Paid != tc.paid {
				t.Errorf("Paid: got %t, want %t", stored.Paid, tc.paid)
			}
			if stored.Project != "Website Redesign" {
				t.Errorf("Project: got %q, want %q", stored.Project, "Website Redesign")
			}
			if len(stored.Weeks) != 1 || stored.Weeks[0].Hours != 10 {
				t.Errorf("weeks changed: %+v", stored.Weeks)
			}
		})
	}
}

func TestPostPaidWithoutSessionGoesToLogin(t *testing.T) {
	stored := &invoice.Invoice{
		Month:    time.October,
		Year:     2027,
		Customer: "Acme Corp",
		Rate:     100,
	}
	s := &fakeStore{invoices: []*invoice.Invoice{stored}}

	recorder := postWithAccess(t, s, &fakeAccess{}, "/2027/oct/paid", url.Values{"paid": {"true"}})

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
	if next := location.Query().Get("next"); next != "/2027/oct" {
		t.Errorf("return page: got %q, want %q", next, "/2027/oct")
	}
	if stored.Paid {
		t.Error("invoice is paid after a request without a session")
	}
}

func TestDetailShowsProjectForm(t *testing.T) {
	stored := &invoice.Invoice{
		Month:    time.October,
		Year:     2027,
		Customer: "Acme Corp",
		Project:  "Website Redesign",
		Rate:     100,
	}

	recorder := get(t, &fakeStore{invoices: []*invoice.Invoice{stored}}, "/2027/oct")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}

	form := projectForm(t, recorder.Body.String())
	opening := form[:strings.Index(form, ">")+1]
	for _, want := range []string{`method="post"`, `action="/2027/oct"`} {
		if !strings.Contains(opening, want) {
			t.Errorf("form does not contain %q", want)
		}
	}

	if count := strings.Count(form, `name="project"`); count != 1 {
		t.Errorf("form posts %d project fields, want 1", count)
	}

	input := projectInput(t, form)
	for _, want := range []string{`type="text"`, `name="project"`, `value="Website Redesign"`} {
		if !strings.Contains(input, want) {
			t.Errorf("project input does not contain %q", want)
		}
	}

	if !strings.Contains(form, "Save") {
		t.Error("form does not have a Save button")
	}
}

func TestDetailHidesProjectFormWhenPaid(t *testing.T) {
	stored := &invoice.Invoice{
		Month:    time.October,
		Year:     2027,
		Customer: "Acme Corp",
		Project:  "Website Redesign",
		Rate:     100,
		Paid:     true,
	}

	recorder := get(t, &fakeStore{invoices: []*invoice.Invoice{stored}}, "/2027/oct")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "Website Redesign") {
		t.Error("detail does not show the project")
	}
	if strings.Contains(body, `name="project"`) {
		t.Error("paid detail shows the project form")
	}
}

func TestPostProjectOnPaidInvoiceReturnsConflict(t *testing.T) {
	for _, role := range editRoles {
		t.Run(role.name, func(t *testing.T) {
			stored := &invoice.Invoice{
				Month:    time.October,
				Year:     2027,
				Customer: "Acme Corp",
				Project:  "Website Redesign",
				Rate:     100,
				Paid:     true,
				Weeks:    []invoice.Week{{Hours: 10}},
			}
			s := &fakeStore{invoices: []*invoice.Invoice{stored}}
			auth := signedInAs(role.name, role.role)

			recorder := postWithAccess(t, s, auth, "/2027/oct", url.Values{"project": {"Mobile App"}})
			if recorder.Code != http.StatusConflict {
				t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusConflict)
			}
			if stored.Project != "Website Redesign" {
				t.Errorf("Project: got %q, want %q", stored.Project, "Website Redesign")
			}
			if !stored.Paid {
				t.Error("invoice is no longer paid")
			}
			if len(stored.Weeks) != 1 || stored.Weeks[0].Hours != 10 {
				t.Errorf("weeks changed: %+v", stored.Weeks)
			}
		})
	}
}

var editRoles = []struct {
	name string
	role access.Role
}{
	{"admin", access.Admin},
	{"customer", access.Customer},
}

func signedInAs(name string, role access.Role) *fakeAccess {
	return &fakeAccess{user: &access.User{
		Email:   name + "@example.com",
		Role:    role,
		Expires: time.Now().Add(time.Hour),
	}}
}

func TestPostProjectChangesProject(t *testing.T) {
	for _, role := range editRoles {
		t.Run(role.name, func(t *testing.T) {
			stored := &invoice.Invoice{
				Month:    time.October,
				Year:     2027,
				Customer: "Acme Corp",
				Project:  "Website Redesign",
				Rate:     100,
				Weeks:    []invoice.Week{{Hours: 10}},
			}
			s := &fakeStore{invoices: []*invoice.Invoice{stored}}
			auth := signedInAs(role.name, role.role)

			recorder := postWithAccess(t, s, auth, "/2027/oct", url.Values{"project": {"Mobile App"}})
			if recorder.Code != http.StatusSeeOther {
				t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusSeeOther)
			}
			if location := recorder.Header().Get("Location"); location != "/2027/oct" {
				t.Errorf("location: got %q, want %q", location, "/2027/oct")
			}
			if stored.Project != "Mobile App" {
				t.Errorf("Project: got %q, want %q", stored.Project, "Mobile App")
			}

			for _, path := range []string{"/", "/2027/oct"} {
				body := getWithAccess(t, s, auth, path).Body.String()
				if !strings.Contains(body, "Mobile App") {
					t.Errorf("%s does not show the new project", path)
				}
				if strings.Contains(body, "Website Redesign") {
					t.Errorf("%s still shows the old project", path)
				}
			}
		})
	}
}

func TestPostProjectClearsProject(t *testing.T) {
	for _, role := range editRoles {
		t.Run(role.name, func(t *testing.T) {
			stored := &invoice.Invoice{
				Month:    time.October,
				Year:     2027,
				Customer: "Acme Corp",
				Project:  "Website Redesign",
				Rate:     100,
			}
			s := &fakeStore{invoices: []*invoice.Invoice{stored}}
			auth := signedInAs(role.name, role.role)

			recorder := postWithAccess(t, s, auth, "/2027/oct", url.Values{"project": {""}})
			if recorder.Code != http.StatusSeeOther {
				t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusSeeOther)
			}
			if stored.Project != "" {
				t.Errorf("Project: got %q, want empty", stored.Project)
			}

			body := getWithAccess(t, s, auth, "/2027/oct").Body.String()
			if strings.Contains(body, "<dt>Project</dt>") {
				t.Error("detail contains a project entry after the project was cleared")
			}
			if input := projectInput(t, projectForm(t, body)); !strings.Contains(input, `value=""`) {
				t.Errorf("project input is not empty: %s", input)
			}
		})
	}
}

func TestPostProjectTrimsWhitespace(t *testing.T) {
	for _, role := range editRoles {
		t.Run(role.name, func(t *testing.T) {
			stored := &invoice.Invoice{
				Month:    time.October,
				Year:     2027,
				Customer: "Acme Corp",
				Rate:     100,
			}
			s := &fakeStore{invoices: []*invoice.Invoice{stored}}
			auth := signedInAs(role.name, role.role)

			recorder := postWithAccess(t, s, auth, "/2027/oct", url.Values{"project": {"  Mobile App  "}})
			if recorder.Code != http.StatusSeeOther {
				t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusSeeOther)
			}
			if stored.Project != "Mobile App" {
				t.Errorf("Project: got %q, want %q", stored.Project, "Mobile App")
			}
		})
	}
}

func TestPostProjectReturnsNotFound(t *testing.T) {
	for _, role := range editRoles {
		t.Run(role.name, func(t *testing.T) {
			auth := signedInAs(role.name, role.role)

			for _, path := range []string{
				"/2026/oct",
				"/2027/sep",
				"/2027/foo",
				"/2027/13",
				"/not-a-year/oct",
			} {
				t.Run(path, func(t *testing.T) {
					stored := &invoice.Invoice{
						Month:    time.October,
						Year:     2027,
						Customer: "Acme Corp",
						Project:  "Website Redesign",
						Rate:     100,
					}
					s := &fakeStore{invoices: []*invoice.Invoice{stored}}

					recorder := postWithAccess(t, s, auth, path, url.Values{"project": {"Mobile App"}})

					if recorder.Code != http.StatusNotFound {
						t.Errorf("status: got %d, want %d", recorder.Code, http.StatusNotFound)
					}
					if stored.Project != "Website Redesign" {
						t.Errorf("Project: got %q, want %q", stored.Project, "Website Redesign")
					}
				})
			}
		})
	}
}

func TestPostProjectReportsSaveFailure(t *testing.T) {
	stored := &invoice.Invoice{
		Month:    time.October,
		Year:     2027,
		Customer: "Acme Corp",
		Project:  "Website Redesign",
		Rate:     100,
	}
	s := &fakeStore{
		invoices: []*invoice.Invoice{stored},
		setErr:   errors.New("database unavailable"),
	}

	recorder := post(t, s, "/2027/oct", url.Values{"project": {"Mobile App"}})

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if stored.Project != "Website Redesign" {
		t.Errorf("Project: got %q, want %q", stored.Project, "Website Redesign")
	}
}

func TestPostProjectWithoutSessionGoesToLogin(t *testing.T) {
	stored := &invoice.Invoice{
		Month:    time.October,
		Year:     2027,
		Customer: "Acme Corp",
		Project:  "Website Redesign",
		Rate:     100,
	}
	s := &fakeStore{invoices: []*invoice.Invoice{stored}}

	recorder := postWithAccess(t, s, &fakeAccess{}, "/2027/oct", url.Values{"project": {"Mobile App"}})

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
	if next := location.Query().Get("next"); next != "/2027/oct" {
		t.Errorf("return page: got %q, want %q", next, "/2027/oct")
	}
	if stored.Project != "Website Redesign" {
		t.Errorf("Project: got %q, want %q", stored.Project, "Website Redesign")
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

func TestListLinksToDetailPages(t *testing.T) {
	recorder := get(t, &fakeStore{invoices: []*invoice.Invoice{{
		Month:    time.October,
		Year:     2027,
		Customer: "Acme Corp",
		Rate:     100,
	}}}, "/")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, `href="/2027/oct"`) {
		t.Error("body does not link to the invoice detail page")
	}
	for _, unwanted := range []string{"hx-get", "hx-target", "hx-swap", "hx-push-url", "htmx.min.js"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("body contains %q", unwanted)
		}
	}
}

func TestServesStylesheet(t *testing.T) {
	recorder := get(t, &fakeStore{}, "/style.css")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "text/css") {
		t.Errorf("content type: got %q, want text/css", contentType)
	}
	if !strings.Contains(recorder.Body.String(), ".invoice-detail") {
		t.Error("body does not contain the invoice detail style")
	}
}

func TestPagesLinkStylesheet(t *testing.T) {
	stored := &invoice.Invoice{
		Month:    time.October,
		Year:     2027,
		Customer: "Acme Corp",
		Rate:     100,
	}

	for _, path := range []string{"/", "/2027/oct"} {
		t.Run(path, func(t *testing.T) {
			body := get(t, &fakeStore{invoices: []*invoice.Invoice{stored}}, path).Body.String()

			if !strings.Contains(body, `rel="stylesheet"`) {
				t.Error("page does not link a stylesheet")
			}
			if !strings.Contains(body, `href="/style.css"`) {
				t.Error("page does not link the served stylesheet")
			}
		})
	}
}

func TestServesAssetsWithoutSupportFiles(t *testing.T) {
	t.Chdir(t.TempDir())

	s := &fakeStore{invoices: []*invoice.Invoice{{
		Month:    time.October,
		Year:     2027,
		Vendor:   "Jane Smith",
		Customer: "Acme Corp",
		Rate:     100,
	}}}

	for _, path := range []string{"/", "/style.css", "/2027/oct/pdf"} {
		t.Run(path, func(t *testing.T) {
			recorder := get(t, s, path)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
			}
			if recorder.Body.Len() == 0 {
				t.Error("body is empty")
			}
			if path == "/2027/oct/pdf" && !bytes.HasPrefix(recorder.Body.Bytes(), []byte("%PDF-")) {
				t.Error("response is not a PDF")
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

	body := recorder.Body.String()
	if !strings.Contains(body, "Acme Corp") {
		t.Error("detail does not contain the customer")
	}
	if strings.Contains(body, "<dt>Project</dt>") {
		t.Error("detail contains a project entry that is not set")
	}
	input := projectInput(t, projectForm(t, body))
	if !strings.Contains(input, `value=""`) {
		t.Errorf("project input is not empty: %s", input)
	}
}

func TestEscapesStoredValues(t *testing.T) {
	stored := &invoice.Invoice{
		Month:    time.October,
		Year:     2027,
		Vendor:   `<script>alert("vendor")</script>`,
		Customer: `<img src=x onerror=alert("customer")>`,
		Project:  `<b>Project & Co</b>`,
		Rate:     100,
		Weeks: []invoice.Week{{
			Start: time.Date(2027, time.October, 4, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2027, time.October, 10, 0, 0, 0, 0, time.UTC),
			Hours: 40,
		}},
	}

	cases := []struct {
		name   string
		path   string
		fields []string
	}{
		{"list", "/", []string{stored.Customer, stored.Project}},
		{"detail", "/2027/oct", []string{stored.Vendor, stored.Customer, stored.Project}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := get(t, &fakeStore{invoices: []*invoice.Invoice{stored}}, tc.path)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
			}

			body := recorder.Body.String()
			for _, field := range tc.fields {
				escaped := template.HTMLEscapeString(field)
				if !strings.Contains(body, escaped) {
					t.Errorf("body does not contain escaped value %q", escaped)
				}
				if strings.Contains(body, field) {
					t.Errorf("body renders raw value %q", field)
				}
			}
		})
	}
}
