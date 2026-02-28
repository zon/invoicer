# invoicer

Generate monthly HTML (and optionally PDF) invoices for hourly contractors. Invoice HTML is written by an AI model via [opencode](https://opencode.ai/) with random creative styling.

## Installation

```
make install
```

Requires `$GOPATH/bin` to be in your `PATH`. Also requires `opencode` to be installed for invoice generation.

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
| `--rate` | `-r` | Hourly rate in dollars. Required if not set in config. |
| `--hours` | `-H` | Hours per week worked. Required if not set in config. |
| `--pdf` | `-p` | Convert the HTML invoice to a PDF file. Defaults to `false`. |
| `--model` | `-m` | opencode-formatted model stub for invoice generation. Defaults to `anthropic/claude-haiku-4-5`. |

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

# Generate and convert to PDF
invoicer -v "Jane Smith" -c "Acme Corp" -r 150 -H 40 --pdf
```

## Config File

Frequently-used options can be stored in `~/.invoicer/config.yaml` to avoid repeating them on every invocation. CLI options always take precedence over config file values.

### Config File Format

```yaml
vendor: Jane Smith
customer: Acme Corp
rate: 150
hours: 40
pdf: false
model: anthropic/claude-haiku-4-5
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
| `--rate` | Hourly rate in dollars. |
| `--hours` | Hours per week worked. |
| `--pdf` | Convert the HTML invoice to a PDF file. |
| `--model` | opencode-formatted model stub for invoice generation. |

The config file and its directory (`~/.invoicer/`) are created automatically if they do not exist.

#### Examples

```bash
# Set vendor and customer
invoicer set config --vendor "Jane Smith" --customer "Acme Corp"

# Set rate and hours
invoicer set config --rate 150 --hours 40

# Enable PDF output by default
invoicer set config --pdf
```

## Invoice Generation

Invoices cover one calendar month and are broken into weekly line items. A week belongs to a month if its **Wednesday** falls in that month. Weeks that span month boundaries are prorated based on the number of working days (Monday–Friday) within the billed month.

The HTML invoice is saved to the current directory as:

```
invoice-<customer>-<year>-<MM>.html
```

If `--pdf` is set, the HTML is also converted to a PDF at:

```
invoice-<customer>-<year>-<MM>.pdf
```

PDF conversion uses `wkhtmltopdf` if available, falling back to `chromium`, `chromium-browser`, `google-chrome`, or `google-chrome-stable` in headless mode.

## Development

```bash
make build   # Build the binary to bin/invoicer
make install # Build and install to $GOPATH/bin
make test    # Run all tests
```
