// Package access signs users in through an OpenID Connect provider and keeps
// their sessions in encrypted cookies.
package access

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/securecookie"
	"golang.org/x/oauth2"
)

// Role is a signed-in user's access level.
type Role string

const (
	// Admin identifies a user in the configured admin group.
	Admin Role = "admin"
	// Customer identifies a user in the configured customer group.
	Customer Role = "customer"
)

// User is the current user of the web app.
type User struct {
	Email   string
	Role    Role
	Expires time.Time
}

// Options holds the settings the access component needs.
type Options struct {
	BaseURL       string
	Issuer        string
	ClientID      string
	ClientSecret  string
	CookieKey     []byte
	AdminGroup    string
	CustomerGroup string
	GroupsClaim   string
}

const (
	signInCookieName  = "invoicer_signin"
	sessionCookieName = "invoicer_session"

	signInDuration  = 10 * time.Minute
	sessionDuration = 12 * time.Hour

	defaultAdminGroup    = "invoicer-admins"
	defaultCustomerGroup = "invoicer-customers"
	defaultGroupsClaim   = "groups"
)

// Access discovers the provider and handles sign-in, sign-out, and sessions.
type Access struct {
	oauth         *oauth2.Config
	verifier      *oidc.IDTokenVerifier
	adminGroup    string
	customerGroup string
	groupsClaim   string
	signIn        *securecookie.SecureCookie
	session       *securecookie.SecureCookie
	now           func() time.Time
}

// New discovers the provider at opts.Issuer and returns access control that
// uses it.
func New(ctx context.Context, opts Options) (*Access, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	provider, err := discover(ctx, opts.Issuer)
	if err != nil {
		return nil, err
	}

	redirectURL := strings.TrimRight(opts.BaseURL, "/") + "/callback"
	return &Access{
		oauth: &oauth2.Config{
			ClientID:     opts.ClientID,
			ClientSecret: opts.ClientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  redirectURL,
			Scopes:       []string{oidc.ScopeOpenID, "email", "profile", opts.GroupsClaim},
		},
		verifier:      provider.Verifier(&oidc.Config{ClientID: opts.ClientID}),
		adminGroup:    opts.AdminGroup,
		customerGroup: opts.CustomerGroup,
		groupsClaim:   opts.GroupsClaim,
		signIn:        newCookieCodec(opts.CookieKey, signInCookieName, signInDuration),
		session:       newCookieCodec(opts.CookieKey, sessionCookieName, sessionDuration),
		now:           time.Now,
	}, nil
}

// Validate reports a missing or invalid setting and applies the group and
// claim defaults.
func (o *Options) Validate() error {
	if err := validateAddress("baseUrl", o.BaseURL); err != nil {
		return err
	}
	if err := validateAddress("oidc.issuer", o.Issuer); err != nil {
		return err
	}
	if o.ClientID == "" {
		return errors.New("oidc.clientId is required")
	}
	if o.ClientSecret == "" {
		return errors.New("client secret is required")
	}
	if len(o.CookieKey) < 32 {
		return errors.New("cookie key must hold at least 32 bytes")
	}
	if o.AdminGroup == "" {
		o.AdminGroup = defaultAdminGroup
	}
	if o.CustomerGroup == "" {
		o.CustomerGroup = defaultCustomerGroup
	}
	if o.GroupsClaim == "" {
		o.GroupsClaim = defaultGroupsClaim
	}
	return nil
}

func validateAddress(name, address string) error {
	if address == "" {
		return fmt.Errorf("%s is required", name)
	}

	parsed, err := url.Parse(address)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return fmt.Errorf("%s must be an absolute HTTPS address", name)
	}
	if parsed.Scheme == "https" {
		return nil
	}
	if parsed.Scheme == "http" && isLoopback(parsed.Hostname()) {
		return nil
	}
	return fmt.Errorf("%s must use HTTPS unless the host is loopback", name)
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func discover(ctx context.Context, issuer string) (*oidc.Provider, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("discovering OIDC provider at %q: %w", issuer, err)
	}

	var metadata struct {
		AuthorizationEndpoint string `json:"authorization_endpoint"`
		TokenEndpoint         string `json:"token_endpoint"`
		JWKSURI               string `json:"jwks_uri"`
	}
	if err := provider.Claims(&metadata); err != nil {
		return nil, fmt.Errorf("reading OIDC provider metadata: %w", err)
	}
	if metadata.AuthorizationEndpoint == "" {
		return nil, fmt.Errorf("OIDC provider at %q has no authorization endpoint", issuer)
	}
	if metadata.TokenEndpoint == "" {
		return nil, fmt.Errorf("OIDC provider at %q has no token endpoint", issuer)
	}
	if metadata.JWKSURI == "" {
		return nil, fmt.Errorf("OIDC provider at %q has no JWKS endpoint", issuer)
	}
	return provider, nil
}

func newCookieCodec(key []byte, name string, duration time.Duration) *securecookie.SecureCookie {
	codec := securecookie.New(deriveKey(key, name+" sign"), deriveKey(key, name+" encrypt")[:32])
	codec.MaxAge(int(duration.Seconds()))
	return codec
}

func deriveKey(key []byte, label string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(label))
	return mac.Sum(nil)
}

type signInState struct {
	State    string
	Nonce    string
	Verifier string
	ReturnTo string
}

// Login sends the browser to the provider to start sign-in.
func (a *Access) Login(c *gin.Context) {
	state, err := randomValue()
	if err != nil {
		a.fail(c)
		return
	}
	nonce, err := randomValue()
	if err != nil {
		a.fail(c)
		return
	}

	verifier := oauth2.GenerateVerifier()
	value, err := a.signIn.Encode(signInCookieName, signInState{
		State:    state,
		Nonce:    nonce,
		Verifier: verifier,
		ReturnTo: returnTo(c),
	})
	if err != nil {
		a.fail(c)
		return
	}
	a.setCookie(c, signInCookieName, value, int(signInDuration.Seconds()))

	c.Redirect(http.StatusFound, a.oauth.AuthCodeURL(
		state,
		oidc.Nonce(nonce),
		oauth2.S256ChallengeOption(verifier),
	))
}

// Callback completes sign-in and starts a session for an allowed user.
func (a *Access) Callback(c *gin.Context) {
	state, ok := a.readSignIn(c)
	if !ok {
		a.fail(c)
		return
	}
	if subtle.ConstantTimeCompare([]byte(c.Query("state")), []byte(state.State)) != 1 {
		a.fail(c)
		return
	}

	token, err := a.oauth.Exchange(
		c.Request.Context(),
		c.Query("code"),
		oauth2.VerifierOption(state.Verifier),
	)
	if err != nil {
		a.fail(c)
		return
	}

	user, ok := a.userFromToken(c.Request.Context(), token, state.Nonce)
	if !ok {
		a.fail(c)
		return
	}

	value, err := a.session.Encode(sessionCookieName, user)
	if err != nil {
		a.fail(c)
		return
	}
	a.setCookie(c, sessionCookieName, value, int(sessionDuration.Seconds()))
	a.clearCookie(c, signInCookieName)

	c.Redirect(http.StatusFound, safeReturnTo(state.ReturnTo))
}

func (a *Access) readSignIn(c *gin.Context) (signInState, bool) {
	value, err := c.Cookie(signInCookieName)
	if err != nil || value == "" {
		return signInState{}, false
	}

	var state signInState
	if err := a.signIn.Decode(signInCookieName, value, &state); err != nil {
		return signInState{}, false
	}
	if state.State == "" || state.Nonce == "" || state.Verifier == "" {
		return signInState{}, false
	}
	return state, true
}

func (a *Access) userFromToken(ctx context.Context, token *oauth2.Token, nonce string) (User, bool) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return User{}, false
	}

	idToken, err := a.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return User{}, false
	}
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(nonce)) != 1 {
		return User{}, false
	}

	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		return User{}, false
	}
	email, _ := claims["email"].(string)
	if email == "" {
		return User{}, false
	}
	if verified, marked := claims["email_verified"].(bool); marked && !verified {
		return User{}, false
	}

	role, ok := a.role(groupsFrom(claims[a.groupsClaim]))
	if !ok {
		return User{}, false
	}
	return User{
		Email:   email,
		Role:    role,
		Expires: a.now().Add(sessionDuration),
	}, true
}

func (a *Access) role(groups []string) (Role, bool) {
	if contains(groups, a.adminGroup) {
		return Admin, true
	}
	if contains(groups, a.customerGroup) {
		return Customer, true
	}
	return "", false
}

func groupsFrom(value any) []string {
	switch groups := value.(type) {
	case []string:
		return groups
	case []any:
		result := make([]string, 0, len(groups))
		for _, group := range groups {
			if name, ok := group.(string); ok {
				result = append(result, name)
			}
		}
		return result
	default:
		return nil
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// CurrentUser returns the signed-in user for a request. A missing, expired, or
// invalid session counts as no user.
func (a *Access) CurrentUser(c *gin.Context) (*User, bool) {
	value, err := c.Cookie(sessionCookieName)
	if err != nil || value == "" {
		return nil, false
	}

	var user User
	if err := a.session.Decode(sessionCookieName, value, &user); err != nil {
		return nil, false
	}
	if user.Email == "" || user.Role != Admin && user.Role != Customer {
		return nil, false
	}
	if !user.Expires.After(a.now()) {
		return nil, false
	}
	return &user, true
}

// Logout clears the session and sends the user back to sign-in.
func (a *Access) Logout(c *gin.Context) {
	a.clearCookie(c, sessionCookieName)
	a.clearCookie(c, signInCookieName)
	c.Redirect(http.StatusFound, "/login")
}

func (a *Access) setCookie(c *gin.Context, name, value string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(name, value, maxAge, "/", "", true, true)
}

func (a *Access) clearCookie(c *gin.Context, name string) {
	a.setCookie(c, name, "", -1)
}

func (a *Access) fail(c *gin.Context) {
	a.clearCookie(c, signInCookieName)
	c.Data(http.StatusUnauthorized, "text/html; charset=utf-8", []byte(failedSignInPage))
}

const failedSignInPage = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Sign-in failed</title>
  <link rel="stylesheet" href="/style.css">
</head>
<body>
  <header>
    <h1>Sign-in failed</h1>
  </header>
  <p>You could not be signed in.</p>
  <p><a href="/login">Try again</a></p>
</body>
</html>`

func randomValue() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generating sign-in value: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value[:]), nil
}

func returnTo(c *gin.Context) string {
	if next := c.Query("next"); next != "" {
		return safeReturnTo(next)
	}
	return safeReturnTo(c.Query("returnTo"))
}

func safeReturnTo(value string) string {
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return "/"
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return "/"
	}
	return value
}
