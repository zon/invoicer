# Web Design

The web app is a second front end over the invoice database. It lists stored invoices and renders a selected invoice. The web app is read-only in this milestone.

## Invoice List

The invoice list is the home page at `/`. It shows every stored invoice with its customer, month and year, project, and total. The app computes the total from stored data, as [Invoices](invoices.md) describes.

The list orders invoices by year and month, newest first.

Selecting an invoice opens its detail page.

## Invoice Detail

The invoice detail uses the year and month, like `/2027/oct`. The month uses its three-letter lowercase name. The detail shows the vendor, customer, project, month, year, hourly rate, weekly line items, and total. Each weekly line item shows its date range and hours.

The detail is a full page. It shows only that invoice and links back to the invoice list. A direct request to an invoice detail URL returns the complete page.

The server returns 404 for an unknown year or month, or a month with no invoice.

## Rendering

Gin serves HTTP. The server renders HTML with Go's `html/template` package. Pages and the stylesheet live in the binary, as [Stack](../stack.md) describes.

Every response is a complete HTML page. The app has no JSON API.

Templates escape stored values. The app never renders an invoice field as raw HTML.

## Access

Every page needs a valid session. The app signs users in, as [Access](access.md) describes. Every signed-in user reads every invoice.

## Data

The web app reads through the store. The store lists every invoice with its weeks and returns one invoice for a year and month. The web app does not write invoices.

## Server

`invoicer web` starts the server. The database path comes from `--database`, then the config file, then the default, as [Config](config.md) describes. `--address` sets the listen address and defaults to `:8080`. The server shuts down gracefully on an interrupt.

## Out of Scope

* Project name edits
* Paid status
* PDF downloads
* Invoice email
* Writes from the web app
* Containers and deployment
