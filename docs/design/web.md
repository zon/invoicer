# Web Design

The web app is a second front end over the invoice database. It lists stored invoices, renders a selected invoice, downloads it as a PDF, changes invoice projects, and lets an admin user mark an invoice paid. The project edit and the paid status are the only writes.

## Invoice List

The invoice list is the home page at `/`. It shows every stored invoice with its customer, month and year, project, paid status, and total. The app computes the total from stored data, as [Invoices](invoices.md) describes.

The list orders invoices by year and month, newest first.

Selecting an invoice opens its detail page.

## Invoice Detail

The invoice detail uses the year and month, like `/2027/oct`. The month uses its three-letter lowercase name. The detail shows the vendor, customer, project, month, year, invoice identifier, hourly rate, weekly line items, paid status, and total. Each weekly line item shows its date range and hours. The project is editable while the invoice is unpaid, as [Project Edits](project-edits.md) describes. A paid invoice shows no project form. An admin user changes the paid status, as [Paid Status](paid-status.md) describes.

The detail is a full page. It shows only that invoice and links back to the invoice list. It links to the invoice PDF, as [PDF](pdf.md) describes. A direct request to an invoice detail URL returns the complete page.

The server returns 404 for an unknown year or month, or a month with no invoice. The 404 response is a complete page that says the invoice was not found and links back to the invoice list.

## Rendering

Gin serves HTTP. The server renders HTML with Go's `html/template` package. Pages and the stylesheet live in the binary, as [Stack](../stack.md) describes. The app renders the invoice PDF from stored data, as [PDF](pdf.md) describes.

Every HTML response is a complete page. The app has no JSON API.

Templates escape stored values. The app never renders an invoice field as raw HTML.

## Access

Every page and PDF download needs a valid session. The app signs users in, as [Access](access.md) describes. Every signed-in user reads every invoice. An admin user marks invoices paid, as [Paid Status](paid-status.md) describes. A paid invoice keeps its values and project until an admin marks it unpaid.

## Data

The web app reads through the store. The store lists every invoice with its weeks and returns one invoice for a year and month. The web app writes only the project and the paid status, as [Project Edits](project-edits.md) and [Paid Status](paid-status.md) describe.

## Server

`invoicer web` starts the server. The database path comes from `--database`, then the config file, then the default, as [Config](config.md) describes. `--address` sets the listen address and defaults to `:8080`. The server shuts down gracefully on an interrupt.

## Out of Scope

* Invoice email
* Writes other than the project edit and the paid status
* Containers and deployment
