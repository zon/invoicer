package access

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

func TestLoginRedirectsWithSignInState(t *testing.T) {
	stub := newProviderStub(t)
	a := newTestAccess(t, stub)
	router := testRouter(a)

	recorder := perform(router, "/login?next=%2F2027%2Foct")

	if recorder.Code != http.StatusFound {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusFound)
	}

	location, err := url.Parse(recorder.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parsing redirect: %v", err)
	}
	if location.Scheme != "http" || location.Host != strings.TrimPrefix(stub.server.URL, "http://") {
		t.Errorf("redirect host: got %q, want %q", location.Host, stub.server.URL)
	}
	if location.Path != "/authorize" {
		t.Errorf("redirect path: got %q, want %q", location.Path, "/authorize")
	}

	query := location.Query()
	for name, want := range map[string]string{
		"client_id":             "client",
		"redirect_uri":          "https://invoices.example.com/callback",
		"response_type":         "code",
		"code_challenge_method": "S256",
	} {
		if got := query.Get(name); got != want {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}
	for _, name := range []string{"state", "nonce", "code_challenge"} {
		if query.Get(name) == "" {
			t.Errorf("%s is empty", name)
		}
	}
	if got := query.Get("scope"); got != "openid email profile groups" {
		t.Errorf("scope: got %q, want %q", got, "openid email profile groups")
	}

	cookie := findCookie(t, recorder, signInCookieName)
	if cookie.MaxAge <= 0 || cookie.MaxAge > int(signInDuration.Seconds()) {
		t.Errorf("sign-in cookie max age: got %d, want between 1 and %d", cookie.MaxAge, int(signInDuration.Seconds()))
	}
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("sign-in cookie is not HttpOnly, Secure, and SameSite=Lax: %+v", cookie)
	}

	var state signInState
	if err := a.signIn.Decode(signInCookieName, cookieValue(t, cookie), &state); err != nil {
		t.Fatalf("decoding sign-in cookie: %v", err)
	}
	if state.State != query.Get("state") {
		t.Errorf("cookie state: got %q, want %q", state.State, query.Get("state"))
	}
	if state.Nonce != query.Get("nonce") {
		t.Errorf("cookie nonce: got %q, want %q", state.Nonce, query.Get("nonce"))
	}
	if state.Verifier == "" {
		t.Error("cookie verifier is empty")
	}
	if state.ReturnTo != "/2027/oct" {
		t.Errorf("cookie return path: got %q, want %q", state.ReturnTo, "/2027/oct")
	}

	challenge := sha256.Sum256([]byte(state.Verifier))
	if got, want := query.Get("code_challenge"), base64.RawURLEncoding.EncodeToString(challenge[:]); got != want {
		t.Errorf("code challenge: got %q, want %q", got, want)
	}

	if err := a.signIn.Decode(signInCookieName, tamper(cookieValue(t, cookie)), &state); err == nil {
		t.Error("tampered sign-in cookie was accepted")
	}
}

func tamper(value string) string {
	if strings.HasPrefix(value, "A") {
		return "B" + value[1:]
	}
	return "A" + value[1:]
}

func TestCallbackStartsSession(t *testing.T) {
	cases := []struct {
		name   string
		groups []string
		want   Role
	}{
		{name: "admin", groups: []string{"admins"}, want: Admin},
		{name: "customer", groups: []string{"customers"}, want: Customer},
		{name: "both groups", groups: []string{"customers", "admins"}, want: Admin},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := newProviderStub(t)
			a := newTestAccess(t, stub)
			router := testRouter(a)

			authURL, signInCookie, _ := startSignIn(t, a, router, "/2027/oct")
			recorder := completeSignIn(t, stub, router, authURL, signInCookie, map[string]any{
				"email":          "jane@example.com",
				"email_verified": true,
				"groups":         tc.groups,
			})

			if recorder.Code != http.StatusFound {
				t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusFound)
			}
			if location := recorder.Header().Get("Location"); location != "/2027/oct" {
				t.Errorf("location: got %q, want %q", location, "/2027/oct")
			}

			cookie := findCookie(t, recorder, sessionCookieName)
			if cookie.MaxAge != int(sessionDuration.Seconds()) {
				t.Errorf("session max age: got %d, want %d", cookie.MaxAge, int(sessionDuration.Seconds()))
			}
			if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
				t.Errorf("session cookie is not HttpOnly, Secure, and SameSite=Lax: %+v", cookie)
			}

			var user User
			if err := a.session.Decode(sessionCookieName, cookieValue(t, cookie), &user); err != nil {
				t.Fatalf("decoding session: %v", err)
			}
			if user.Email != "jane@example.com" {
				t.Errorf("email: got %q, want %q", user.Email, "jane@example.com")
			}
			if user.Role != tc.want {
				t.Errorf("role: got %q, want %q", user.Role, tc.want)
			}
			if until := time.Until(user.Expires); until < 11*time.Hour || until > 13*time.Hour {
				t.Errorf("session expiry is not 12 hours away: %v", until)
			}

			whoami := performWithCookies(router, "/whoami", cookie)
			if whoami.Code != http.StatusOK {
				t.Fatalf("whoami status: got %d, want %d", whoami.Code, http.StatusOK)
			}
			if body := whoami.Body.String(); !strings.Contains(body, "jane@example.com") || !strings.Contains(body, string(tc.want)) {
				t.Errorf("whoami body: got %q", body)
			}
		})
	}
}

func TestCallbackRedirectsToRequestedPage(t *testing.T) {
	cases := []struct {
		name     string
		next     string
		location string
	}{
		{name: "no requested page", next: "", location: "/"},
		{name: "requested invoice", next: "/2027/oct", location: "/2027/oct"},
		{name: "another site", next: "//example.com/invoices", location: "/"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := newProviderStub(t)
			a := newTestAccess(t, stub)
			router := testRouter(a)

			authURL, signInCookie, _ := startSignIn(t, a, router, tc.next)
			recorder := completeSignIn(t, stub, router, authURL, signInCookie, map[string]any{
				"email":          "jane@example.com",
				"email_verified": true,
				"groups":         []string{"admins"},
			})

			if recorder.Code != http.StatusFound {
				t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusFound)
			}
			if location := recorder.Header().Get("Location"); location != tc.location {
				t.Errorf("location: got %q, want %q", location, tc.location)
			}
		})
	}
}

func TestCallbackStartsTwelveHourSession(t *testing.T) {
	stub := newProviderStub(t)
	a := newTestAccess(t, stub)
	router := testRouter(a)

	signedInAt := time.Date(2026, time.November, 22, 9, 0, 0, 0, time.UTC)
	a.now = func() time.Time { return signedInAt }

	authURL, signInCookie, _ := startSignIn(t, a, router, "/")
	recorder := completeSignIn(t, stub, router, authURL, signInCookie, map[string]any{
		"email":          "jane@example.com",
		"email_verified": true,
		"groups":         []string{"customers"},
	})

	cookie := findCookie(t, recorder, sessionCookieName)
	if cookie.MaxAge != 12*60*60 {
		t.Errorf("session max age: got %d, want %d", cookie.MaxAge, 12*60*60)
	}

	var user User
	if err := a.session.Decode(sessionCookieName, cookieValue(t, cookie), &user); err != nil {
		t.Fatalf("decoding session: %v", err)
	}
	if want := signedInAt.Add(12 * time.Hour); !user.Expires.Equal(want) {
		t.Errorf("session expiry: got %s, want %s", user.Expires, want)
	}
}

func TestCallbackRejectsMissingSignInState(t *testing.T) {
	stub := newProviderStub(t)
	a := newTestAccess(t, stub)
	router := testRouter(a)

	recorder := perform(router, "/callback?code=test&state=test")

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	assertNoSessionCookie(t, recorder)
}

func TestCallbackRejectsMissingCode(t *testing.T) {
	stub := newProviderStub(t)
	a := newTestAccess(t, stub)
	router := testRouter(a)

	authURL, signInCookie, _ := startSignIn(t, a, router, "/")
	recorder := performWithCookies(
		router,
		"/callback?state="+url.QueryEscape(authURL.Query().Get("state")),
		signInCookie,
	)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	assertNoSessionCookie(t, recorder)
}

func TestCallbackRejectsTokenEndpointError(t *testing.T) {
	stub := newProviderStub(t)
	stub.mu.Lock()
	stub.tokenStatus = http.StatusBadRequest
	stub.mu.Unlock()
	a := newTestAccess(t, stub)
	router := testRouter(a)

	authURL, signInCookie, _ := startSignIn(t, a, router, "/")
	recorder := completeSignIn(t, stub, router, authURL, signInCookie, map[string]any{
		"email":          "jane@example.com",
		"email_verified": true,
		"groups":         []string{"admins"},
	})

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	assertNoSessionCookie(t, recorder)
}

func TestCallbackRejectsTokenWithoutIDToken(t *testing.T) {
	stub := newProviderStub(t)
	stub.mu.Lock()
	stub.tokenIDToken = false
	stub.mu.Unlock()
	a := newTestAccess(t, stub)
	router := testRouter(a)

	authURL, signInCookie, _ := startSignIn(t, a, router, "/")
	recorder := completeSignIn(t, stub, router, authURL, signInCookie, map[string]any{
		"email":          "jane@example.com",
		"email_verified": true,
		"groups":         []string{"admins"},
	})

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	assertNoSessionCookie(t, recorder)
}

func assertNoSessionCookie(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == sessionCookieName && cookie.Value != "" && cookie.MaxAge >= 0 {
			t.Errorf("failed sign-in set a session cookie: %+v", cookie)
		}
	}
}

func TestCallbackRejectsFailedSignIns(t *testing.T) {
	cases := []struct {
		name     string
		claims   map[string]any
		badState bool
		nonce    string
	}{
		{
			name:   "user in neither group",
			claims: map[string]any{"email": "jane@example.com", "email_verified": true, "groups": []string{"other"}},
		},
		{
			name:   "missing email",
			claims: map[string]any{"email_verified": true, "groups": []string{"admins"}},
		},
		{
			name:   "unverified email",
			claims: map[string]any{"email": "jane@example.com", "email_verified": false, "groups": []string{"admins"}},
		},
		{
			name:   "nonce does not match",
			claims: map[string]any{"email": "jane@example.com", "email_verified": true, "groups": []string{"admins"}},
			nonce:  "another-nonce",
		},
		{
			name:     "state does not match",
			claims:   map[string]any{"email": "jane@example.com", "email_verified": true, "groups": []string{"admins"}},
			badState: true,
		},
		{
			name:   "wrong issuer",
			claims: map[string]any{"iss": "https://another.example.com", "email": "jane@example.com", "email_verified": true, "groups": []string{"admins"}},
		},
		{
			name:   "wrong audience",
			claims: map[string]any{"aud": "another-client", "email": "jane@example.com", "email_verified": true, "groups": []string{"admins"}},
		},
		{
			name:   "expired token",
			claims: map[string]any{"exp": time.Now().Add(-time.Hour).Unix(), "email": "jane@example.com", "email_verified": true, "groups": []string{"admins"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := newProviderStub(t)
			a := newTestAccess(t, stub)
			router := testRouter(a)

			authURL, signInCookie, _ := startSignIn(t, a, router, "/")
			state := authURL.Query().Get("state")
			if tc.badState {
				state = "another-state"
			}
			recorder := completeSignInWithState(t, stub, router, state, authURL, signInCookie, tc.nonce, tc.claims)

			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusUnauthorized)
			}
			body := recorder.Body.String()
			if !strings.HasPrefix(body, "<!doctype html>") || !strings.Contains(body, `href="/login"`) {
				t.Errorf("failed sign-in is not a complete page linking to /login: %q", body)
			}
			assertNoSessionCookie(t, recorder)
		})
	}
}

func TestExpiredSessionCountsAsNoSession(t *testing.T) {
	stub := newProviderStub(t)
	a := newTestAccess(t, stub)
	router := testRouter(a)

	authURL, signInCookie, _ := startSignIn(t, a, router, "/")
	recorder := completeSignIn(t, stub, router, authURL, signInCookie, map[string]any{
		"email":          "jane@example.com",
		"email_verified": true,
		"groups":         []string{"admins"},
	})
	cookie := findCookie(t, recorder, sessionCookieName)

	now := a.now()
	a.now = func() time.Time { return now.Add(sessionDuration + time.Minute) }

	whoami := performWithCookies(router, "/whoami", cookie)
	if whoami.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", whoami.Code, http.StatusUnauthorized)
	}
}

func TestMissingSessionCountsAsNoSession(t *testing.T) {
	stub := newProviderStub(t)
	router := testRouter(newTestAccess(t, stub))

	whoami := perform(router, "/whoami")

	if whoami.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", whoami.Code, http.StatusUnauthorized)
	}
}

func TestInvalidSessionCountsAsNoSession(t *testing.T) {
	stub := newProviderStub(t)
	a := newTestAccess(t, stub)
	router := testRouter(a)

	whoami := performWithCookies(router, "/whoami", &http.Cookie{
		Name:  sessionCookieName,
		Value: "not-a-session",
	})

	if whoami.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", whoami.Code, http.StatusUnauthorized)
	}
}

func TestSessionCookieIsEncryptedAndSigned(t *testing.T) {
	stub := newProviderStub(t)
	a := newTestAccess(t, stub)
	router := testRouter(a)

	authURL, signInCookie, _ := startSignIn(t, a, router, "/")
	recorder := completeSignIn(t, stub, router, authURL, signInCookie, map[string]any{
		"email":          "jane@example.com",
		"email_verified": true,
		"groups":         []string{"admins"},
	})
	cookie := findCookie(t, recorder, sessionCookieName)

	decoded, err := base64.URLEncoding.DecodeString(cookieValue(t, cookie))
	if err != nil {
		t.Fatalf("decoding session cookie: %v", err)
	}
	for _, hidden := range []string{"jane@example.com", "admin", "customer"} {
		if strings.Contains(string(decoded), hidden) {
			t.Errorf("session cookie exposes %q", hidden)
		}
	}

	tampered := *cookie
	tampered.Value = tamper(cookieValue(t, cookie))
	if whoami := performWithCookies(router, "/whoami", &tampered); whoami.Code != http.StatusUnauthorized {
		t.Errorf("tampered session status: got %d, want %d", whoami.Code, http.StatusUnauthorized)
	}

	other := newTestAccessWith(t, stub, Options{
		BaseURL:       "https://invoices.example.com",
		ClientID:      "client",
		ClientSecret:  "secret",
		CookieKey:     bytes.Repeat([]byte("z"), 32),
		AdminGroup:    "admins",
		CustomerGroup: "customers",
	})
	if whoami := performWithCookies(testRouter(other), "/whoami", cookie); whoami.Code != http.StatusUnauthorized {
		t.Errorf("session under another key status: got %d, want %d", whoami.Code, http.StatusUnauthorized)
	}
}

func TestLogoutClearsSessionAndRedirects(t *testing.T) {
	stub := newProviderStub(t)
	a := newTestAccess(t, stub)
	router := testRouter(a)

	authURL, signInCookie, _ := startSignIn(t, a, router, "/")
	recorder := completeSignIn(t, stub, router, authURL, signInCookie, map[string]any{
		"email":          "jane@example.com",
		"email_verified": true,
		"groups":         []string{"admins"},
	})
	session := findCookie(t, recorder, sessionCookieName)

	logout := performWithCookies(router, "/logout", session)
	if logout.Code != http.StatusFound {
		t.Fatalf("status: got %d, want %d", logout.Code, http.StatusFound)
	}
	if location := logout.Header().Get("Location"); location != "/login" {
		t.Errorf("location: got %q, want %q", location, "/login")
	}

	cleared := findCookie(t, logout, sessionCookieName)
	if cleared.Value != "" || cleared.MaxAge >= 0 {
		t.Errorf("session cookie was not cleared: %+v", cleared)
	}
}

func TestNewRejectsInvalidOptions(t *testing.T) {
	stub := newProviderStub(t)
	valid := Options{
		BaseURL:       "https://invoices.example.com",
		Issuer:        stub.server.URL,
		ClientID:      "client",
		ClientSecret:  "secret",
		CookieKey:     bytes.Repeat([]byte("k"), 32),
		AdminGroup:    "admins",
		CustomerGroup: "customers",
	}

	cases := []struct {
		name   string
		change func(*Options)
	}{
		{name: "missing issuer", change: func(opts *Options) { opts.Issuer = "" }},
		{name: "missing client ID", change: func(opts *Options) { opts.ClientID = "" }},
		{name: "missing client secret", change: func(opts *Options) { opts.ClientSecret = "" }},
		{name: "missing base URL", change: func(opts *Options) { opts.BaseURL = "" }},
		{name: "short cookie key", change: func(opts *Options) { opts.CookieKey = []byte("short") }},
		{name: "relative base URL", change: func(opts *Options) { opts.BaseURL = "/invoices" }},
		{name: "plain HTTP base URL", change: func(opts *Options) { opts.BaseURL = "http://invoices.example.com" }},
		{name: "relative issuer", change: func(opts *Options) { opts.Issuer = "auth.example.com" }},
		{name: "plain HTTP issuer", change: func(opts *Options) { opts.Issuer = "http://auth.example.com" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := valid
			tc.change(&opts)
			if _, err := New(context.Background(), opts); err == nil {
				t.Fatal("New did not report the invalid option")
			}
		})
	}
}

func TestNewDefaultsGroupsAndClaim(t *testing.T) {
	stub := newProviderStub(t)
	a := newTestAccessWith(t, stub, Options{
		BaseURL:      "https://invoices.example.com",
		Issuer:       stub.server.URL,
		ClientID:     "client",
		ClientSecret: "secret",
		CookieKey:    bytes.Repeat([]byte("k"), 32),
	})

	if a.adminGroup != defaultAdminGroup {
		t.Errorf("admin group: got %q, want %q", a.adminGroup, defaultAdminGroup)
	}
	if a.customerGroup != defaultCustomerGroup {
		t.Errorf("customer group: got %q, want %q", a.customerGroup, defaultCustomerGroup)
	}
	if a.groupsClaim != defaultGroupsClaim {
		t.Errorf("groups claim: got %q, want %q", a.groupsClaim, defaultGroupsClaim)
	}
}

func TestValidateAllowsLoopbackHTTP(t *testing.T) {
	for _, baseURL := range []string{
		"http://localhost:8080",
		"http://127.0.0.1:8080",
		"http://[::1]:8080",
	} {
		t.Run(baseURL, func(t *testing.T) {
			opts := Options{
				BaseURL:      baseURL,
				Issuer:       baseURL,
				ClientID:     "client",
				ClientSecret: "secret",
				CookieKey:    bytes.Repeat([]byte("k"), 32),
			}
			if err := opts.Validate(); err != nil {
				t.Errorf("Validate: %v", err)
			}
		})
	}
}

func TestNewReportsDiscoveryErrors(t *testing.T) {
	t.Run("issuer is unreachable", func(t *testing.T) {
		server := httptest.NewServer(http.NotFoundHandler())
		issuer := server.URL
		server.Close()

		_, err := New(context.Background(), Options{
			BaseURL:      "https://invoices.example.com",
			Issuer:       issuer,
			ClientID:     "client",
			ClientSecret: "secret",
			CookieKey:    bytes.Repeat([]byte("k"), 32),
		})
		if err == nil {
			t.Fatal("New did not report the unreachable issuer")
		}
	})

	for _, endpoint := range []string{"authorization_endpoint", "token_endpoint", "jwks_uri"} {
		t.Run("missing "+endpoint, func(t *testing.T) {
			stub := newProviderStub(t)
			stub.metadata = map[string]any{endpoint: ""}

			_, err := New(context.Background(), Options{
				BaseURL:      "https://invoices.example.com",
				Issuer:       stub.server.URL,
				ClientID:     "client",
				ClientSecret: "secret",
				CookieKey:    bytes.Repeat([]byte("k"), 32),
			})
			if err == nil {
				t.Fatalf("New did not report the missing %s", endpoint)
			}
		})
	}
}

func newTestAccess(t *testing.T, stub *providerStub) *Access {
	t.Helper()
	return newTestAccessWith(t, stub, Options{
		BaseURL:       "https://invoices.example.com",
		Issuer:        stub.server.URL,
		ClientID:      "client",
		ClientSecret:  "secret",
		CookieKey:     bytes.Repeat([]byte("k"), 32),
		AdminGroup:    "admins",
		CustomerGroup: "customers",
	})
}

func newTestAccessWith(t *testing.T, stub *providerStub, opts Options) *Access {
	t.Helper()
	if opts.Issuer == "" {
		opts.Issuer = stub.server.URL
	}
	a, err := New(context.Background(), opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return a
}

func testRouter(a *Access) *gin.Engine {
	router := gin.New()
	router.GET("/login", a.Login)
	router.GET("/callback", a.Callback)
	router.GET("/logout", a.Logout)
	router.GET("/whoami", func(c *gin.Context) {
		user, ok := a.CurrentUser(c)
		if !ok {
			c.Status(http.StatusUnauthorized)
			return
		}
		c.String(http.StatusOK, "%s %s", user.Email, user.Role)
	})
	return router
}

func startSignIn(t *testing.T, a *Access, router http.Handler, next string) (*url.URL, *http.Cookie, signInState) {
	t.Helper()
	recorder := perform(router, "/login?next="+url.QueryEscape(next))
	if recorder.Code != http.StatusFound {
		t.Fatalf("login status: got %d, want %d", recorder.Code, http.StatusFound)
	}

	authURL, err := url.Parse(recorder.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parsing authorization URL: %v", err)
	}
	cookie := findCookie(t, recorder, signInCookieName)

	var state signInState
	if err := a.signIn.Decode(signInCookieName, cookieValue(t, cookie), &state); err != nil {
		t.Fatalf("decoding sign-in cookie: %v", err)
	}
	return authURL, cookie, state
}

func completeSignIn(t *testing.T, stub *providerStub, router http.Handler, authURL *url.URL, cookie *http.Cookie, claims map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return completeSignInWithState(t, stub, router, authURL.Query().Get("state"), authURL, cookie, authURL.Query().Get("nonce"), claims)
}

func completeSignInWithState(t *testing.T, stub *providerStub, router http.Handler, state string, authURL *url.URL, cookie *http.Cookie, nonce string, claims map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	if nonce == "" {
		nonce = authURL.Query().Get("nonce")
	}
	stub.mu.Lock()
	stub.nonce = nonce
	stub.claims = claims
	stub.mu.Unlock()

	return performWithCookies(router, "/callback?code=test&state="+url.QueryEscape(state), cookie)
}

func perform(handler http.Handler, target string) *httptest.ResponseRecorder {
	return performWithCookies(handler, target)
}

func performWithCookies(handler http.Handler, target string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	handler.ServeHTTP(recorder, request)
	return recorder
}

func cookieValue(t *testing.T, cookie *http.Cookie) string {
	t.Helper()
	value, err := url.QueryUnescape(cookie.Value)
	if err != nil {
		t.Fatalf("unescaping cookie %q: %v", cookie.Name, err)
	}
	return value
}

func findCookie(t *testing.T, recorder *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("response has no %q cookie", name)
	return nil
}

type providerStub struct {
	server   *httptest.Server
	key      *rsa.PrivateKey
	metadata map[string]any

	mu           sync.Mutex
	nonce        string
	claims       map[string]any
	tokenStatus  int
	tokenIDToken bool
}

func newProviderStub(t *testing.T) *providerStub {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating provider key: %v", err)
	}

	stub := &providerStub{key: key, tokenIDToken: true}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", stub.discovery)
	mux.HandleFunc("/keys", stub.keys)
	mux.HandleFunc("/token", stub.token)
	stub.server = httptest.NewServer(mux)
	t.Cleanup(stub.server.Close)
	return stub
}

func (p *providerStub) discovery(w http.ResponseWriter, r *http.Request) {
	metadata := map[string]any{
		"issuer":                 p.server.URL,
		"authorization_endpoint": p.server.URL + "/authorize",
		"token_endpoint":         p.server.URL + "/token",
		"jwks_uri":               p.server.URL + "/keys",
	}
	for key, value := range p.metadata {
		if value == "" {
			delete(metadata, key)
			continue
		}
		metadata[key] = value
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metadata); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (p *providerStub) keys(w http.ResponseWriter, r *http.Request) {
	exponent := big.NewInt(int64(p.key.PublicKey.E))
	response := map[string]any{
		"keys": []map[string]any{{
			"kty": "RSA",
			"kid": "test",
			"use": "sig",
			"alg": "RS256",
			"n":   base64.RawURLEncoding.EncodeToString(p.key.PublicKey.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(exponent.Bytes()),
		}},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (p *providerStub) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	p.mu.Lock()
	status := p.tokenStatus
	includeIDToken := p.tokenIDToken
	claims := map[string]any{
		"iss":   p.server.URL,
		"aud":   "client",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
		"nonce": p.nonce,
	}
	for name, value := range p.claims {
		claims[name] = value
	}
	p.mu.Unlock()

	if status != 0 {
		http.Error(w, "token endpoint failed", status)
		return
	}

	response := map[string]any{
		"access_token": "access",
		"token_type":   "Bearer",
	}
	if includeIDToken {
		response["id_token"] = p.sign(claims)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (p *providerStub) sign(claims map[string]any) string {
	header := encodeJSON(map[string]any{"alg": "RS256", "kid": "test", "typ": "JWT"})
	payload := encodeJSON(claims)
	signingInput := header + "." + payload

	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, p.key, crypto.SHA256, digest[:])
	if err != nil {
		panic(err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func encodeJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}
