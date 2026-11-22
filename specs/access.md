# Access Specification

## Purpose
Sign-in and read access for the invoice web app. The app signs users in through an OpenID Connect provider and gives every signed-in user read access to every stored invoice.

## Requirements

### Requirement: Sign-In Required
The app SHALL require a valid session for every page. A request without a session SHALL go to `/login` and SHALL return the user to the requested page after sign-in.

The stylesheet SHALL NOT require a session.

#### Scenario: Page without a session
- GIVEN a request without a session
- WHEN `/` is requested
- THEN the app sends the user to `/login`

#### Scenario: Return to the requested page
- GIVEN a request without a session for an invoice detail
- WHEN the user signs in
- THEN the app shows that invoice detail

#### Scenario: Stylesheet without a session
- GIVEN a request without a session
- WHEN `/style.css` is requested
- THEN the app returns the stylesheet

### Requirement: Groups and Roles
The app SHALL read group membership from the claim named by `oidc.groupsClaim`. Config SHALL name the admin group and the customer group.

A user in either group SHALL sign in. A user in neither group SHALL NOT sign in.

A user in the admin group SHALL be an admin user. A user in the customer group SHALL be a customer user. A user in both groups SHALL be an admin user. The app SHALL NOT store group membership.

#### Scenario: Admin user
- GIVEN a user in the admin group and not the customer group
- WHEN the user signs in
- THEN the user is an admin user

#### Scenario: Customer user
- GIVEN a user in the customer group and not the admin group
- WHEN the user signs in
- THEN the user is a customer user

#### Scenario: Both groups
- GIVEN a user in the admin group and the customer group
- WHEN the user signs in
- THEN the user is an admin user

#### Scenario: Neither group
- GIVEN a user in neither group
- WHEN the user tries to sign in
- THEN the app sets no session
- AND the app returns 401

### Requirement: Sign-In Start
`/login` SHALL send the browser to the provider's authorization endpoint with the client ID, the callback address, the scopes `openid`, `email`, `profile`, and `groups`, a state value, a nonce, and a PKCE challenge.

The app SHALL keep the state, nonce, and PKCE verifier in a short-lived signed cookie.

#### Scenario: Authorization request
- GIVEN a sign-in attempt
- WHEN `/login` is requested
- THEN the browser goes to the provider's authorization endpoint
- AND the request carries the client ID, callback address, scopes, state, nonce, and PKCE challenge

#### Scenario: Sign-in state is kept
- GIVEN a sign-in attempt
- WHEN `/login` returns its redirect
- THEN the response sets a short-lived signed cookie
- AND the cookie holds the state, nonce, and PKCE verifier

### Requirement: Sign-In Completion
`/callback` SHALL check the state, exchange the code, and validate the ID token's issuer, audience, expiry, and nonce.

The app SHALL require the email claim and SHALL reject an email the provider marks unverified.

On success the app SHALL set the session and SHALL redirect to the address the user first asked for, or to `/`.

#### Scenario: Successful sign-in
- GIVEN a valid callback for a user in the admin group
- WHEN `/callback` is requested
- THEN the app sets a session
- AND the app redirects to the requested page or `/`

#### Scenario: State does not match
- GIVEN a callback whose state does not match the sign-in cookie
- WHEN `/callback` is requested
- THEN the app sets no session
- AND the app returns 401

#### Scenario: Invalid ID token
- GIVEN an ID token with the wrong issuer, audience, expiry, or nonce
- WHEN `/callback` validates it
- THEN the app sets no session
- AND the app returns 401

#### Scenario: Missing email
- GIVEN an ID token without an email claim
- WHEN `/callback` validates it
- THEN the app sets no session
- AND the app returns 401

#### Scenario: Unverified email
- GIVEN an ID token whose email the provider marks unverified
- WHEN `/callback` validates it
- THEN the app sets no session
- AND the app returns 401

### Requirement: Failed Sign-In
A failed sign-in SHALL set no session and SHALL return 401 with a complete HTML page and a link to try again.

#### Scenario: Failed sign-in page
- GIVEN a sign-in that fails
- WHEN the app responds
- THEN the response is a complete HTML page
- AND the page links to try again
- AND the response sets no session

### Requirement: Session
The session cookie SHALL hold the user's email, role, and expiry. The app SHALL encrypt and sign the cookie with the key from `oidc.cookieKeyFile`. The cookie SHALL be HttpOnly, Secure, and SameSite=Lax.

A session SHALL last 12 hours and SHALL NOT renew. A missing, expired, or invalid cookie SHALL count as no session.

#### Scenario: Session lasts 12 hours
- GIVEN a session that is less than 12 hours old
- WHEN a page is requested
- THEN the app serves the page

#### Scenario: Expired session
- GIVEN a session older than 12 hours
- WHEN a page is requested
- THEN the app sends the user to `/login`

#### Scenario: Invalid session
- GIVEN a session cookie that fails its signature or cannot be decrypted
- WHEN a page is requested
- THEN the app sends the user to `/login`

### Requirement: Sign-Out
`/logout` SHALL clear the session and SHALL send the user to `/login`.

#### Scenario: Sign out
- GIVEN a signed-in user
- WHEN `/logout` is requested
- THEN the app clears the session
- AND the app sends the user to `/login`

### Requirement: Read Access
Every signed-in user SHALL read every stored invoice. The list and detail SHALL show the same invoices to admin and customer users. A signed-in user SHALL NOT change stored invoices.

An unknown invoice SHALL return 404, as [Web](web.md#requirement-missing-invoice) describes.

#### Scenario: Admin reads every invoice
- GIVEN a signed-in admin user and stored invoices for several months
- WHEN `/` is requested
- THEN the list shows every stored invoice

#### Scenario: Customer reads every invoice
- GIVEN a signed-in customer user and stored invoices for several months
- WHEN `/` is requested
- THEN the list shows every stored invoice

#### Scenario: Detail
- GIVEN a signed-in customer user and a stored invoice
- WHEN its detail is requested
- THEN the detail shows that invoice

### Requirement: Signed-In Header
Every page shown to a signed-in user SHALL show the user's email and a sign-out link.

#### Scenario: Header on a page
- GIVEN a signed-in user
- WHEN an invoice page is requested
- THEN the page shows the user's email
- AND the page links to `/logout`

### Requirement: Access Configuration
The web app SHALL read these settings once at startup from the config file:

```yaml
baseUrl: https://invoices.example.com
oidc:
  issuer: https://auth.example.com
  clientId: invoicer
  clientSecretFile: /run/secrets/oidc-client-secret
  cookieKeyFile: /run/secrets/cookie-key
  adminGroup: invoicer-admins
  customerGroup: invoicer-customers
  groupsClaim: groups
```

`baseUrl` and `oidc.issuer` SHALL be absolute HTTPS addresses. A loopback address MAY use HTTP.

`oidc.clientId` SHALL be set. `oidc.clientSecretFile` SHALL name a readable, non-empty file. `oidc.cookieKeyFile` SHALL name a readable file with at least 32 bytes. The app SHALL read the client secret and the cookie key from those files once at startup.

The callback address SHALL be `baseUrl` plus `/callback`.

`oidc.adminGroup` SHALL default to `invoicer-admins`, `oidc.customerGroup` SHALL default to `invoicer-customers`, and `oidc.groupsClaim` SHALL default to `groups`.

Startup SHALL fail with an error when a required setting is missing or invalid.

#### Scenario: Missing client ID
- GIVEN a config without `oidc.clientId`
- WHEN `invoicer web` starts
- THEN the command reports an error
- AND the command exits non-zero

#### Scenario: Secret file is missing
- GIVEN a config whose `oidc.clientSecretFile` does not exist
- WHEN `invoicer web` starts
- THEN the command reports an error
- AND the command exits non-zero

#### Scenario: Cookie key is too short
- GIVEN a config whose `oidc.cookieKeyFile` holds fewer than 32 bytes
- WHEN `invoicer web` starts
- THEN the command reports an error
- AND the command exits non-zero

#### Scenario: Plain HTTP outside loopback
- GIVEN a `baseUrl` that is plain HTTP and not a loopback address
- WHEN `invoicer web` starts
- THEN the command reports an error
- AND the command exits non-zero

#### Scenario: Defaults
- GIVEN a config that sets the required settings but no groups or claim names
- WHEN the app starts
- THEN the admin group is `invoicer-admins`
- AND the customer group is `invoicer-customers`
- AND the groups claim is `groups`

### Requirement: Provider Discovery
At startup the app SHALL read the provider's discovery document from the issuer. Startup SHALL fail with an error when the issuer is unreachable or the document lacks the authorization, token, or JWKS endpoints.

The app SHALL validate tokens against the provider's published keys and SHALL use the authorization code flow with PKCE. It SHALL ask for no refresh token.

#### Scenario: Issuer is unreachable
- GIVEN an issuer that does not respond
- WHEN `invoicer web` starts
- THEN the command reports an error
- AND the command exits non-zero

#### Scenario: Discovery document lacks an endpoint
- GIVEN a discovery document without a JWKS endpoint
- WHEN `invoicer web` starts
- THEN the command reports an error
- AND the command exits non-zero

#### Scenario: Tokens use published keys
- GIVEN a provider that publishes its signing keys
- WHEN the app validates an ID token
- THEN the app checks the signature against the published keys
