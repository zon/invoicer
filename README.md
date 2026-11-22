# invoicer

Generate monthly invoices for hourly contractors. Invoices and their weekly line items are stored in a SQLite database.

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
| `--customer` | `-c` | Name of the client receiving the invoice. Required if not set in config. |
| `--project` | | Name of the project to annotate the invoice with. Optional. |
| `--database` | | Path to the invoice database. Defaults to `~/.invoicer/invoices.db`. |
| `--rate` | `-r` | Hourly rate in dollars. Required if not set in config. |
| `--hours` | `-H` | Hours per week worked. Required if not set in config. |

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

# Annotate the invoice with a project name
invoicer --vendor "Jane Smith" --customer "Acme Corp" --rate 150 --hours 40 --project "Website Redesign"

# Store the invoice in a different database
invoicer --vendor "Jane Smith" --customer "Acme Corp" --rate 150 --hours 40 --database ./work.db
```

## Config File

Frequently-used options can be stored in `~/.invoicer/config.yaml` to avoid repeating them on every invocation. CLI options always take precedence over config file values.

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

Use the `set config` subcommand to write options to the config file without editing it manually. Only the options you specify are updated; others remain unchanged.

```
invoicer set config [options]
```

| Option | Description |
|--------|-------------|
| `--vendor` | Name of the contractor sending the invoice. |
| `--customer` | Name of the client receiving the invoice. |
| `--project` | Name of the project to annotate invoices with. |
| `--database` | Path to the invoice database. |
| `--rate` | Hourly rate in dollars. |
| `--hours` | Hours per week worked. |

The config file and its directory (`~/.invoicer/`) are created automatically if they do not exist.

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

Invoices cover one calendar month and are broken into weekly line items. A week belongs to a month if its **Wednesday** falls in that month. Each week bills the full hours per week.

Generate saves the invoice and its weeks, then reports whether it created or edited the invoice, with the month, year, and identifier:

```
$ invoicer --vendor "Jane Smith" --customer "Acme Corp" --rate 150 --hours 40
Created invoice for October 2026: 2f8c1a9b5d3e4f6a7c8b9d0e1f2a3b4c
```

Running generate again for the same customer, year, month, and project edits the stored invoice and prints the same identifier:

```
$ invoicer --vendor "Jane Smith" --customer "Acme Corp" --rate 175 --hours 40
Edited invoice for October 2026: 2f8c1a9b5d3e4f6a7c8b9d0e1f2a3b4c
```

A different project creates a separate invoice.

## Database

The database path comes from `--database`, then the config file, then the default of `~/.invoicer/invoices.db`. Invoicer creates the database file and its parent directory on first use, then migrates the schema automatically.

## Development

```bash
just build   # Build the binary to bin/invoicer
just install # Build and install to $GOPATH/bin
just test    # Run all tests
just         # List available recipes
```
