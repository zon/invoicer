# Invoices Design

The CLI stores every invoice in SQLite. The stored data is the source of truth.

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

A week line item records its start date, end date, and hours. Weeks belong to their invoice. The invoice total is the sum of each week's hours times the invoice rate. The system computes the total from stored data and never stores it.

The year and month identify an invoice. Generating the same year and month again replaces the stored invoice and keeps its identifier. The new customer, project, rate, and weeks replace the stored values.

## Store

The store uses one SQLite database and GORM. Invoices and weeks live in related tables. The invoice table has a unique key on year and month, so a month holds one invoice. The store replaces the existing invoice for a month instead of inserting a second one. When an older database holds duplicate invoices for a month, opening it keeps the newest invoice and removes the others. Saving an invoice and its weeks is one transaction, so a failure leaves no partial invoice.

The store creates and migrates the database on first use. The default path is `~/.invoicer/invoices.db`.

The path comes from the `database` config value or the `--database` flag. The flag wins. `set config --database` writes the config value. See [Config](config.md).

## Generate

Generate resolves its options, builds the week line items, and saves the invoice and its weeks in one transaction. It then prints the invoice identifier.

Generate writes no files and generates no HTML. A failed save leaves the previous invoice unchanged and reports an error.

## Out of Scope

* Importing existing invoice files.
* Rendering stored invoices. The [web app](web.md) renders them.
* PDF downloads. A later milestone renders a PDF from stored data.
* Paid status, customer records, and access control.
