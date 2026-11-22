package cli

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/securecookie"

	"github.com/zon/invoicer/internal/access"
	"github.com/zon/invoicer/internal/invoice"
	"github.com/zon/invoicer/internal/store"
)

// TestWebSignInAgainstLocalProvider runs the whole sign-in flow through the web
// command: a page without a session, the provider authorization redirect, the
// callback, the signed-in pages, and sign-out.
func TestWebSignInAgainstLocalProvider(t *testing.T) {
	stub := newDiscoveryStub(t)
	secretPath, keyPath := writeAccessFiles(t)
	dbPath := seedAccessStore(t)
	configPath := writeTestConfig(t, "database: "+dbPath+"\n"+accessYAMLWithIssuer(stub.server.URL, secretPath, keyPath))

	handler, stop := startWebApp(t, configPath)
	defer stop()

	page := accessRequest(handler, "/2027/oct")
	if page.Code != http.StatusFound {
		t.Fatalf("page status: got %d, want %d", page.Code, http.StatusFound)
	}
	if location := page.Header().Get("Location"); location != "/login?next=%2F2027%2Foct" {
		t.Errorf("page location: got %q, want %q", location, "/login?next=%2F2027%2Foct")
	}

	signIn := signInThroughProvider(t, handler, stub, "/2027/oct", map[string]any{
		"email":          "jane@example.com",
		"email_verified": true,
		"groups":         []string{"invoicer-admins"},
	})
	if signIn.response.Code != http.StatusFound {
		t.Fatalf("callback status: got %d, want %d", signIn.response.Code, http.StatusFound)
	}
	if location := signIn.response.Header().Get("Location"); location != "/2027/oct" {
		t.Errorf("callback location: got %q, want %q", location, "/2027/oct")
	}
	if signIn.session == nil {
		t.Fatal("callback set no session")
	}
	if signIn.session.MaxAge != 12*60*60 {
		t.Errorf("session max age: got %d, want %d", signIn.session.MaxAge, 12*60*60)
	}

	detail := accessRequest(handler, "/2027/oct", signIn.session)
	if detail.Code != http.StatusOK {
		t.Fatalf("detail status: got %d, want %d", detail.Code, http.StatusOK)
	}
	for _, want := range []string{"Acme Corp", "jane@example.com", `href="/logout"`} {
		if !strings.Contains(detail.Body.String(), want) {
			t.Errorf("detail does not contain %q", want)
		}
	}

	list := accessRequest(handler, "/", signIn.session)
	if list.Code != http.StatusOK {
		t.Fatalf("list status: got %d, want %d", list.Code, http.StatusOK)
	}
	if !strings.Contains(list.Body.String(), "Acme Corp") {
		t.Error("list does not show the stored invoice")
	}

	logout := accessRequest(handler, "/logout", signIn.session)
	if logout.Code != http.StatusFound {
		t.Fatalf("logout status: got %d, want %d", logout.Code, http.StatusFound)
	}
	if location := logout.Header().Get("Location"); location != "/login" {
		t.Errorf("logout location: got %q, want %q", location, "/login")
	}
	cleared := responseCookieNamed(t, logout, signIn.session.Name)
	if cleared.Value != "" || cleared.MaxAge >= 0 {
		t.Errorf("logout did not clear the session: %+v", cleared)
	}

	signedOut := accessRequest(handler, "/")
	if signedOut.Code != http.StatusFound {
		t.Fatalf("signed-out status: got %d, want %d", signedOut.Code, http.StatusFound)
	}
	if location := signedOut.Header().Get("Location"); location != "/login?next=%2F" {
		t.Errorf("signed-out location: got %q, want %q", location, "/login?next=%2F")
	}
}

// TestWebRejectsUserOutsideGroupsAgainstLocalProvider checks that a user the
// local provider puts in neither group gets the retry page and no session.
func TestWebRejectsUserOutsideGroupsAgainstLocalProvider(t *testing.T) {
	stub := newDiscoveryStub(t)
	secretPath, keyPath := writeAccessFiles(t)
	dbPath := seedAccessStore(t)
	configPath := writeTestConfig(t, "database: "+dbPath+"\n"+accessYAMLWithIssuer(stub.server.URL, secretPath, keyPath))

	handler, stop := startWebApp(t, configPath)
	defer stop()

	signIn := signInThroughProvider(t, handler, stub, "/", map[string]any{
		"email":          "jane@example.com",
		"email_verified": true,
		"groups":         []string{"other"},
	})
	if signIn.response.Code != http.StatusUnauthorized {
		t.Fatalf("callback status: got %d, want %d", signIn.response.Code, http.StatusUnauthorized)
	}
	if signIn.session != nil {
		t.Errorf("rejected sign-in set a session: %+v", signIn.session)
	}

	body := signIn.response.Body.String()
	if !strings.HasPrefix(body, "<!doctype html>") || !strings.HasSuffix(strings.TrimSpace(body), "</html>") {
		t.Errorf("rejected sign-in is not a complete HTML page: %q", body)
	}
	if !strings.Contains(body, "Try again") || !strings.Contains(body, `href="/login"`) {
		t.Errorf("rejected sign-in does not link to try again: %q", body)
	}
	if contentType := signIn.response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/html") {
		t.Errorf("rejected sign-in content type: got %q, want HTML", contentType)
	}

	page := accessRequest(handler, "/")
	if page.Code != http.StatusFound {
		t.Fatalf("page status after a rejected sign-in: got %d, want %d", page.Code, http.StatusFound)
	}
	if location := page.Header().Get("Location"); location != "/login?next=%2F" {
		t.Errorf("page location after a rejected sign-in: got %q, want %q", location, "/login?next=%2F")
	}
}

// TestWebExpiredSessionAgainstLocalProvider signs in through the local
// provider and then presents a validly signed session whose expiry has passed.
// A future session from the same path must still work, so the test knows the
// expiry alone caused the rejection.
func TestWebExpiredSessionAgainstLocalProvider(t *testing.T) {
	stub := newDiscoveryStub(t)
	secretPath, keyPath := writeAccessFiles(t)
	dbPath := seedAccessStore(t)
	configPath := writeTestConfig(t, "database: "+dbPath+"\n"+accessYAMLWithIssuer(stub.server.URL, secretPath, keyPath))

	handler, stop := startWebApp(t, configPath)
	defer stop()

	signIn := signInThroughProvider(t, handler, stub, "/", map[string]any{
		"email":          "jane@example.com",
		"email_verified": true,
		"groups":         []string{"invoicer-admins"},
	})
	if signIn.session == nil {
		t.Fatal("callback set no session")
	}

	key := bytes.Repeat([]byte("k"), 32)
	future := forgeSession(t, signIn.session.Name, key, access.User{
		Email:   "jane@example.com",
		Role:    access.Admin,
		Expires: time.Now().Add(time.Hour),
	})
	if page := accessRequest(handler, "/", future); page.Code != http.StatusOK {
		t.Fatalf("future session status: got %d, want %d", page.Code, http.StatusOK)
	}

	expired := forgeSession(t, signIn.session.Name, key, access.User{
		Email:   "jane@example.com",
		Role:    access.Admin,
		Expires: time.Now().Add(-time.Minute),
	})
	page := accessRequest(handler, "/", expired)
	if page.Code != http.StatusFound {
		t.Fatalf("expired session status: got %d, want %d", page.Code, http.StatusFound)
	}
	if location := page.Header().Get("Location"); location != "/login?next=%2F" {
		t.Errorf("expired session location: got %q, want %q", location, "/login?next=%2F")
	}
}

func seedAccessStore(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "invoices.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	if _, _, err := s.Save(&invoice.Invoice{
		Month:    time.October,
		Year:     2027,
		Vendor:   "Jane Smith",
		Customer: "Acme Corp",
		Rate:     100,
	}); err != nil {
		t.Fatalf("saving invoice: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("closing store: %v", err)
	}
	return path
}

// startWebApp runs the web command and hands back the handler while the
// database is open. The returned stop function shuts the command down.
func startWebApp(t *testing.T, configPath string) (http.Handler, func()) {
	t.Helper()

	original := listenAndServe
	t.Cleanup(func() { listenAndServe = original })

	started := make(chan http.Handler, 1)
	release := make(chan struct{})
	finished := make(chan error, 1)
	listenAndServe = func(server *http.Server) error {
		started <- server.Handler
		<-release
		return nil
	}

	go func() {
		finished <- (&WebCmd{}).run(configPath, io.Discard)
	}()

	select {
	case handler := <-started:
		return handler, func() {
			close(release)
			if err := <-finished; err != nil {
				t.Errorf("web command: %v", err)
			}
		}
	case err := <-finished:
		t.Fatalf("web command stopped before serving: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("web server did not start")
	}
	return nil, nil
}

type providerSignIn struct {
	response *httptest.ResponseRecorder
	session  *http.Cookie
}

// signInThroughProvider follows the sign-in redirects and plays the provider's
// part: it reads the state and nonce from the authorization request and
// returns a signed ID token from the local stub.
func signInThroughProvider(t *testing.T, handler http.Handler, stub *discoveryStub, next string, claims map[string]any) providerSignIn {
	t.Helper()

	login := accessRequest(handler, "/login?next="+url.QueryEscape(next))
	if login.Code != http.StatusFound {
		t.Fatalf("login status: got %d, want %d", login.Code, http.StatusFound)
	}
	authURL, err := url.Parse(login.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parsing authorization URL: %v", err)
	}
	if !strings.HasPrefix(authURL.String(), stub.server.URL+"/") {
		t.Fatalf("authorization URL does not use the local provider: %q", authURL)
	}

	signInCookie := firstResponseCookie(t, login)
	stub.prepareNonce(authURL.Query().Get("nonce"), claims)

	callback := accessRequest(
		handler,
		"/callback?code=test&state="+url.QueryEscape(authURL.Query().Get("state")),
		signInCookie,
	)
	return providerSignIn{
		response: callback,
		session:  newSessionCookie(t, callback, signInCookie.Name),
	}
}

// newSessionCookie returns the session set by a callback, or nil when the
// sign-in failed.
func newSessionCookie(t *testing.T, recorder *httptest.ResponseRecorder, signInName string) *http.Cookie {
	t.Helper()
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name != signInName && cookie.Value != "" && cookie.MaxAge > 0 {
			return cookie
		}
	}
	return nil
}

func accessRequest(handler http.Handler, target string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	handler.ServeHTTP(recorder, request)
	return recorder
}

func firstResponseCookie(t *testing.T, recorder *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	cookies := recorder.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("response sets no cookie")
	}
	return cookies[0]
}

func responseCookieNamed(t *testing.T, recorder *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("response has no %q cookie", name)
	return nil
}

// forgeSession builds a session cookie the way the access component does, so
// a test can present one whose expiry has passed without waiting 12 hours.
func forgeSession(t *testing.T, name string, key []byte, user access.User) *http.Cookie {
	t.Helper()

	codec := securecookie.New(deriveTestKey(key, name+" sign"), deriveTestKey(key, name+" encrypt")[:32])
	codec.MaxAge(int((12 * time.Hour).Seconds()))
	value, err := codec.Encode(name, user)
	if err != nil {
		t.Fatalf("encoding session: %v", err)
	}
	return &http.Cookie{Name: name, Value: url.QueryEscape(value)}
}

func deriveTestKey(key []byte, label string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(label))
	return mac.Sum(nil)
}
