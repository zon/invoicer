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
| `year`   | Year of the invoice month. Defaults to the year closest to the given month. |

### Options

| Option | Short | Description |
|--------|-------|-------------|
| `--vendor` | `-v` | Name of the contractor sending the invoice. Required if not set in config. |
| `--customer` | `-c` | Name of the customer receiving the invoice. Required if not set in config. |
| `--project` | | Project name to store with the invoice. Optional. |
| `--database` | | Path to the invoice database. Defaults to `~/.invoicer/invoices.db`. |
| `--rate` | `-r` | Hourly rate in dollars. Required if not set in config. |
| `--hours` | `-H` | Hours worked per week. Required if not set in config. |

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
rate: 150
hours: 40
project: Website Redesign
database: ~/.invoicer/invoices.db
```

### `set config` Subcommand

Use the `set config` subcommand to write options to the config file without editing it by hand. The command updates only the options you specify and leaves the others unchanged.

```
invoicer set config [options]
```

| Option | Description |
|--------|-------------|
| `--vendor` | Name of the contractor sending the invoice. |
| `--customer` | Name of the customer receiving the invoice. |
| `--project` | Project name to store with invoices. |
| `--database` | Path to the invoice database. |
| `--rate` | Hourly rate in dollars. |
| `--hours` | Hours worked per week. |

Invoicer creates the config file and its directory (`~/.invoicer/`) when they do not exist.

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
Created invoice for October 2026: 2f8c1a9b5d3e4f6a7c8b9d0e1f2a3b4c
```

Running generate again for the same year and month edits the stored invoice and prints the same identifier:

```
$ invoicer --vendor "Jane Smith" --customer "Acme Corp" --rate 175 --hours 40
Edited invoice for October 2026: 2f8c1a9b5d3e4f6a7c8b9d0e1f2a3b4c
```

Changing the customer or project replaces the stored values and keeps the identifier.

## Web App

`invoicer web` starts a read-only web app for the stored invoices. It lists invoices at `/` and shows each invoice as a full page with a link back to the list.

```
invoicer web [options]
```

| Option | Description |
|--------|-------------|
| `--address` | Address the server listens on. Defaults to `:8080`. |
| `--database` | Path to the invoice database. Defaults to the config value, then `~/.invoicer/invoices.db`. |

The server serves pages and the stylesheet from the binary, so it needs no support files. An interrupt stops the server gracefully. Without access settings in the config, the app starts without sign-in for local development.

```bash
# Serve the default database at http://localhost:8080/
invoicer web

# Serve a different database on another address
invoicer web --address 127.0.0.1:9000 --database ./work.db
```

## Database

The database path comes from `--database`, then the config file, then the default of `~/.invoicer/invoices.db`. Invoicer creates the database file and its parent directory on first use, then migrates the schema.

## Development

```bash
just build   # Build the binary to bin/invoicer
just dev     # Rebuild and restart the web app on changes
just install # Build and install to $GOPATH/bin
just test    # Run all tests
just         # List available recipes
```

`just dev` runs the web app through [air](https://github.com/air-verse/air), installed with `go install github.com/air-verse/air@latest`. It uses the address and database `invoicer web` would use. Because the templates and stylesheet are embedded, air watches `html` and `css` files as well as Go files, so an edit to `internal/web/templates` reloads too. Extra arguments are passed to the app:

```bash
# Reload the web app on another port with a scratch database
just dev -- --address 127.0.0.1:9000 --database ./tmp/scratch.db
```
