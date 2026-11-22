# Access Design

The web app requires sign-in through an OpenID Connect provider. Every page needs a valid session. The provider reports group membership. A user in the admin group or the customer group may sign in. Every signed-in user reads every invoice and changes invoice projects, as [Project Edits](project-edits.md) describes. The role decides other writes, such as marking an invoice paid.

## Groups

The provider reports group membership in a groups claim. Config names the admin group and the customer group. Membership in either group allows sign-in. A user in neither group cannot sign in.

A user in the admin group is an admin user. A user in the customer group is a customer user. A user in both groups is an admin user.

The app does not store group membership. The provider owns the groups.

## Read Access

Every invoice belongs to the customer. Every signed-in user reads every stored invoice. The list and detail show the same invoices to both groups.

A request without a session goes to `/login` and returns to the requested page after sign-in.

A request for an unknown invoice returns 404.

## Sign-In

`/login` starts sign-in. The app redirects the browser to the provider's authorization endpoint with:

* the client ID
* the callback address
* the scopes `openid`, `email`, `profile`, and `groups`
* a state value
* a nonce
* a PKCE challenge

The app keeps state, nonce, and the PKCE verifier in a short-lived signed cookie.

`/callback` completes sign-in. It checks the state, exchanges the code, and validates the ID token: issuer, audience, expiry, and nonce. It reads the user's email and groups. It then sets the session cookie and redirects to the address the user first asked for, or to `/`.

The app requires the email claim and rejects an email the provider marks unverified. A user in neither group fails sign-in.

A failed sign-in sets no session and returns 401 with a complete HTML page and a link to try again. When the email is unverified, the page says so.

## Session

The session cookie holds the user's email, role, and expiry. The app encrypts and signs the cookie with a key from a file. The cookie is HttpOnly, Secure, and SameSite=Lax. A session lasts 12 hours and does not renew.

A missing, expired, or invalid cookie counts as no session. The app sends the user to `/login` and returns them to the requested page after sign-in.

`/logout` clears the session and sends the user to `/login`.

The signed-in email and a sign-out link show on every page. Every route except `/login`, `/callback`, and `/logout` requires a session, including the stylesheet. The failed sign-in page carries its own inline styles.

## Configuration

The web app reads its settings from the config file. The app reads the file once at startup:

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

Secrets live in files, so a deployment mounts them from its secret store and the config names only their paths. The app reads the client secret and cookie key once at startup.

A config with no access settings starts the app without sign-in. This mode supports local development. When the config sets any access setting, the full configuration is required, and startup fails when one is missing:

* `baseUrl` and `oidc.issuer` are absolute HTTPS addresses. Loopback addresses may use HTTP.
* `oidc.clientId` is set.
* `oidc.clientSecretFile` names a readable, non-empty file.
* `oidc.cookieKeyFile` names a readable file with at least 32 bytes.

The callback address is `baseUrl` plus `/callback`.

`oidc.adminGroup` defaults to `invoicer-admins`. `oidc.customerGroup` defaults to `invoicer-customers`. `oidc.groupsClaim` defaults to `groups`.

## Provider

The app is an OIDC client. At startup it reads the provider's discovery document from the issuer and fails when the issuer is unreachable or the document lacks the authorization, token, or JWKS endpoints.

Tokens are validated against the provider's published keys. The app uses the authorization code flow with PKCE and asks for no refresh token.

## Out of Scope

* Serving several customers
* Mapping groups to customers
* Roles other than admin user and customer user
* Refresh tokens and offline access
* Provider-initiated sign-out
* API tokens
* Other writes from the web app
