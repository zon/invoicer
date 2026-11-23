# invoicer

Generate monthly invoices for hourly contractors. The CLI stores invoices and their weekly line items in a SQLite database.

## Installation

```
just install
```

Requires `$GOPATH/bin` to be in your `PATH`.

## Usage

```
invoicer [<month> [<year>]] [options]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `month`  | Month to invoice for. Accepts full name (`january`), abbreviation (`jan`), or numeric (`1`–`12`). Defaults to the previous calendar month. |
| `year`   | Year of the invoice month. Defaults to the current year, except when the month is omitted and the previous month is December, which uses the prior year. |

### Options

| Option | Short | Description |
|--------|-------|-------------|
| `--vendor` | `-v` | Name of the contractor sending the invoice. Required if not set in config. |
| `--customer` | `-c` | Name of the customer receiving the invoice. Required if not set in config. |
| `--customer-email` | | Email address that receives the invoice email. Defaults to the `customerEmail` config value. |
| `--project` | | Project name to store with the invoice. Optional. |
| `--database` | | Path to the invoice database. Defaults to `~/.invoicer/invoices.db`. |
| `--rate` | `-r` | Hourly rate in dollars. Required if not set in config. |
| `--hours` | `-H` | Hours worked per week. Required if not set in config. |
| `--version` | | Print the app version and exit. |

### Examples

```bash
# Generate invoice for the previous month (requires config or flags)
invoicer

# Generate invoice for a specific month
invoicer january

# Generate invoice for a specific month and year
invoicer 3 2025

# Specify all options inline
invoicer --vendor "Jane Smith" --customer "Acme Corp" --rate 150 --hours 40

# Add a project name to the invoice
invoicer --vendor "Jane Smith" --customer "Acme Corp" --rate 150 --hours 40 --project "Website Redesign"

# Store the invoice in a different database
invoicer --vendor "Jane Smith" --customer "Acme Corp" --rate 150 --hours 40 --database ./work.db
```

## Config File

Store common options in `~/.invoicer/config.yaml` to avoid repeating them on every run. CLI options always override config file values.

### Config File Format

```yaml
vendor: Jane Smith
customer: Acme Corp
customerEmail: billing@acme.com
rate: 150
hours: 40
project: Website Redesign
database: ~/.invoicer/invoices.db
```

The web app also reads `baseUrl` and `oidc` settings from the same file, as [Access Settings](#access-settings) describes. Generate reads `customerEmail` and `resend` settings from the same file, as [Email Settings](#email-settings) describes.

### Email Settings

Generate can email the invoice to the customer. Set `customerEmail` and a `resend` block:

```yaml
customerEmail: billing@acme.com
baseUrl: https://invoices.example.com
resend:
  apiKeyFile: /run/secrets/resend-api-key
  from: invoicer@example.com
```

`customerEmail` receives the invoice email. `--customer-email` overrides the config value. `resend.apiKeyFile` names a readable file with the Resend API key. `resend.from` is the sender address on a verified Resend domain.

When the config holds any `resend` value, generate requires `customerEmail`, `baseUrl`, `resend.apiKeyFile`, and `resend.from`. It exits with an error before it saves when one is missing. Without `resend` settings, generate saves the invoice and sends nothing.

### `set config` Subcommand

Use the `set config` subcommand to write options to the config file without editing it by hand. The command updates only the options you specify and leaves the others unchanged.

```
invoicer set config [options]
```

| Option | Description |
|--------|-------------|
| `--vendor` | Name of the contractor sending the invoice. |
| `--customer` | Name of the customer receiving the invoice. |
| `--customer-email` | Email address that receives the invoice email. |
| `--project` | Project name to store with invoices. |
| `--database` | Path to the invoice database. |
| `--rate` | Hourly rate in dollars. |
| `--hours` | Hours worked per week. |

Invoicer creates the config file and its directory (`~/.invoicer/`) when they do not exist. The command writes `customerEmail` and leaves the `resend` block unchanged. Edit the file to configure email sending.

#### Examples

```bash
# Set vendor and customer
invoicer set config --vendor "Jane Smith" --customer "Acme Corp"

# Set rate and hours
invoicer set config --rate 150 --hours 40

# Store the database in a custom location
invoicer set config --database ./work.db
```

## Invoice Generation

An invoice covers one calendar month and holds weekly line items. The database stores one invoice per month. A week belongs to a month if its **Wednesday** falls in that month. Each week bills the full weekly hours.

Generate saves the invoice and its weeks, then reports whether it created or edited the invoice, with the month, year, and identifier:

```
$ invoicer --vendor "Jane Smith" --customer "Acme Corp" --rate 150 --hours 40
Created invoice for October 2026: 3F9K2M8Q1X
```

Running generate again for the same year and month edits the stored invoice and prints the same identifier:

```
$ invoicer --vendor "Jane Smith" --customer "Acme Corp" --rate 175 --hours 40
Edited invoice for October 2026: 3F9K2M8Q1X
```

Changing the customer or project replaces the stored values and keeps the identifier. A paid invoice blocks generation until an admin user marks it unpaid in the web app.

### Invoice Email

With email settings, generate sends the invoice email after it saves the invoice. The email goes to the stored contact email and links to the invoice detail page. Generate prints the recipient and the Resend email identifier after Resend accepts the message.

The email sends once. A later generate for an invoice with a send time sends nothing, so an edit does not send a second email. Generate sends nothing for a paid invoice.

A failed send reports an error and exits non-zero. The invoice stays stored without a send time, so a later generate sends it again. A timeout, a rejected message, and a missing or unreadable API key file count as failed sends.

## Removing an Invoice

`invoicer remove` deletes a stored invoice and its weekly line items. Pass the invoice identifier, which accepts uppercase or lowercase:

```
invoicer remove <id> [options]
```

| Option | Description |
|--------|-------------|
| `--database` | Path to the invoice database. Defaults to the config value, then `~/.invoicer/invoices.db`. |

```bash
$ invoicer remove 3F9K2M8Q1X
Removed invoice for October 2026: 3F9K2M8Q1X
```

The command removes paid invoices too. An unknown identifier reports an error and removes nothing.

## Web App

`invoicer web` starts a web app for the stored invoices. It lists invoices at `/` and shows each invoice as a full page with a link back to the list. A user downloads an invoice as a PDF from its detail page. A signed-in user can change an invoice's project from its detail page. An admin user can mark the invoice paid or unpaid from that page. A paid invoice blocks project edits.

```
invoicer web [options]
```

| Option | Description |
|--------|-------------|
| `--address` | Address the server listens on. Defaults to `:8080`. |
| `--database` | Path to the invoice database. Defaults to the config value, then `~/.invoicer/invoices.db`. |

The server serves pages and the stylesheet from the binary, so it needs no support files. An interrupt stops the server gracefully. Without access settings in the config, the app starts without sign-in for local development. With access settings, the app needs a configured OpenID Connect provider and requires sign-in.

```bash
# Serve the default database at http://localhost:8080/
invoicer web

# Serve a different database on another address
invoicer web --address 127.0.0.1:9000 --database ./work.db
```

### Access Settings

Set `baseUrl` and the `oidc` section in the config file:

```yaml
baseUrl: https://invoices.haralovich.com
oidc:
  issuer: https://auth.example.com
  clientId: invoicer
  clientSecretFile: /run/secrets/oidc-client-secret
  cookieKeyFile: /run/secrets/cookie-key
  adminGroup: invoicer-admins
  customerGroup: invoicer-customers
  groupsClaim: groups
```

`baseUrl` is the public address of the app. `oidc.issuer` is the provider's issuer address. Both must be absolute HTTPS addresses. A loopback address may use HTTP. The callback address is `baseUrl` plus `/callback`.

`oidc.clientId` is required. `oidc.clientSecretFile` names a readable, non-empty file, and `oidc.cookieKeyFile` names a readable file with at least 32 bytes. The app reads both once at startup.

`oidc.adminGroup` defaults to `invoicer-admins`, `oidc.customerGroup` defaults to `invoicer-customers`, and `oidc.groupsClaim` defaults to `groups`.

The app reads the provider's discovery document from `oidc.issuer` at startup. It stops with an error when the issuer is unreachable or the document lacks the authorization, token, or JWKS endpoints.

### Sign-In

Every page requires a session. A request without one goes to `/login`, which sends the browser to the provider's authorization endpoint. The app asks for the `openid`, `email`, `profile`, and `groups` scopes and uses the authorization code flow with PKCE.

The provider returns the browser to `/callback`. The app checks the sign-in state and validates the ID token's issuer, audience, expiry, and nonce. It requires a verified email, then starts a 12 hour session and redirects to the page the user first asked for, or to `/`.

Only a user in the admin group or the customer group may sign in. A user in both groups is an admin user. A failed sign-in sets no session and returns a page that links to try again.

The session does not renew. A missing, expired, or invalid session goes to `/login`. Every page shows the signed-in email and a link to `/logout`, which clears the session and returns the user to `/login`.

## Database

The database path comes from `--database`, then the config file, then the default of `~/.invoicer/invoices.db`. Invoicer creates the database file and its parent directory on first use, then migrates the schema.

## Deployment

The repository holds a container image and a Helm chart under `deploy`.

### Container Image

The image runs `invoicer` as its entrypoint and passes arguments through. `web` serves the web app and no subcommand generates an invoice. The image holds no config, database, or secret.

Build the image with Podman from the repository root:

```bash
podman build -t ghcr.io/zon/invoicer:0.1.0 --build-arg VERSION=0.1.0 -f deploy/container/Containerfile .
```

The `VERSION` build argument stamps the app version into the binary. Without it the image reports `dev`.

Push the image to a registry the cluster can reach:

```bash
podman push ghcr.io/zon/invoicer:0.1.0
```

### Helm Chart

The chart uses `ghcr.io/zon/invoicer` with the chart app version as the default tag. Override `image.repository` and `image.tag` to deploy another image.

The chart installs into an existing `invoicer` namespace and does not create it. Create the namespace and the OIDC Secret first:

```bash
kubectl create namespace invoicer
kubectl -n invoicer create secret generic invoicer-oidc \
  --from-file=client-secret=./client-secret \
  --from-file=cookie-key=./cookie-key
```

For email, create the Resend Secret with the API key:

```bash
kubectl -n invoicer create secret generic invoicer-resend \
  --from-file=api-key=./resend-api-key
```

The default keys are `client-secret` and `cookie-key`. `access.clientSecretKey` and `access.cookieKeyKey` name other keys. The Resend key defaults to `api-key`. `resend.apiKeyKey` names another key, and `resend.secretName` names another Secret.

The chart renders `config.customerEmail` and `resend.from` into the invoicer config. When `resend.from` is empty, the chart renders no `resend` block and mounts no Resend Secret, so generate sends no email. Otherwise, the schedule pod mounts the API key at `/run/secrets/resend-api-key`. The web app does not send email and mounts no Resend Secret.

Then install the chart:

```bash
helm install invoicer deploy/chart --namespace invoicer \
  --set hostname=invoices.example.com \
  --set publicURL=https://invoices.example.com \
  --set access.issuer=https://id.example.com
```

Set `hostname` and `publicURL` to the public address. Set `access` to the OIDC provider, `resend.from` to the sender, and `config.customerEmail` to the recipient. The defaults live in `deploy/chart/values.yaml`. The chart creates the web Deployment, Service, claim, Ingress, and Certificate.

## Development

```bash
just build   # Build the binary to bin/invoicer
just dev     # Rebuild and restart the web app on changes
just install # Build and install to $GOPATH/bin
just test    # Run all tests
just release # Tag and push the remote main head as the VERSION release
just         # List available recipes
```

`just build` stamps the version from git. `invoicer --version` reports the release tag without its `v`, or a version naming the commit after the last tag. Uncommitted changes mark the version dirty.

`just dev` runs the web app through [air](https://github.com/air-verse/air), installed with `go install github.com/air-verse/air@latest`. It uses the address and database `invoicer web` would use. Because the templates and stylesheet are embedded, air watches `html` and `css` files as well as Go files, so an edit to `internal/web/templates` reloads too. Extra arguments are passed to the app:

```bash
# Reload the web app on another port with a scratch database
just dev -- --address 127.0.0.1:9000 --database ./tmp/scratch.db
```
