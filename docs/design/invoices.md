# Invoices Design

The CLI stores every invoice in SQLite. The stored data is the source of truth. Generate writes no files and produces no HTML.

## Invoice

An invoice records one customer's work for one calendar month:

* vendor
* customer
* project, when set
* year and month
* hourly rate
* weekly line items

An invoice holds data only. Rendering is separate.

The store assigns an opaque string identifier when it first saves an invoice. The identifier never changes.

A week line item records its start date, end date, and hours. Weeks belong to their invoice. The invoice total is the sum of each week's hours times the invoice rate. The total is computed, not stored.

The customer, year, month, and project identify an invoice. Generating the same four values again replaces the stored invoice and keeps its identifier. This matches the old file behavior, where the same name overwrote the previous file.

## Store

The store is one SQLite database, accessed with GORM. Invoices and weeks live in related tables. Saving an invoice and its weeks is one transaction, so a failure leaves no partial invoice.

The store creates and migrates the database on first use. The default path is `~/.invoicer/invoices.db`.

The path comes from the `database` config value or the `--database` flag. The flag wins. `set config --database` writes the config value. See [Config](config.md).

## Generate

Generate resolves its options, builds the week line items, and saves the invoice and its weeks in one transaction. It then prints the invoice identifier.

Generate writes no files and generates no HTML. A failed save leaves the previous invoice unchanged and reports an error.

## Removed

* Invoice HTML and PDF output.
* HTML generation. The command no longer produces invoice HTML.
* The `--pdf` flag, the `pdf` config value, and PDF conversion.
* The `--model` flag and `model` config value. They only configured HTML generation.
* File paths in command output. Generate prints the invoice identifier instead.

## Out of Scope

* Importing existing invoice files.
* Rendering stored invoices. The web app adds that later.
* PDF downloads. A later milestone renders a PDF from stored data.
* Paid status, customer records, and access control.
