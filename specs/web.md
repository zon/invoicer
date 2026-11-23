# Web Specification

## Purpose
The web app lists invoices from the invoice database, renders a selected invoice from its stored data, downloads it as a PDF, changes an invoice's project, and shows its paid status.

## Requirements

### Requirement: Invoice List
The app SHALL serve the invoice list at `/`. The list SHALL show every stored invoice with its customer, month and year, project when set, paid status, and total. It SHALL order invoices newest first by year and month. Each invoice SHALL link to its detail page.

The total SHALL follow [Invoice Amount](invoices.md#requirement-invoice-amount).

#### Scenario: List shows every invoice
- GIVEN stored invoices for several months
- WHEN `/` is requested
- THEN the page lists every invoice with its customer, month, year, project, paid status, and total

#### Scenario: Newest invoice comes first
- GIVEN invoices for October 2027 and January 2028
- WHEN `/` is requested
- THEN the January 2028 invoice appears before the October 2027 invoice

#### Scenario: No stored invoices
- GIVEN an empty database
- WHEN `/` is requested
- THEN the page shows an empty list

### Requirement: Invoice Detail
The detail address SHALL use the year and three-letter lowercase month name, like `/2027/oct`. The detail SHALL show vendor, customer, project when set, month, year, invoice identifier, hourly rate, every weekly line item, paid status, and total. Each weekly line item SHALL show its date range and hours. The detail SHALL show the project edit form while the invoice is unpaid, as [Project Edits](project-edits.md#requirement-project-edit-form) describes, and the paid status form for an admin user, as [Paid Status](paid-status.md#requirement-paid-status-update) describes.

The detail SHALL be a full page. It SHALL show only the invoice and SHALL link back to the invoice list. It SHALL link to the invoice PDF at the detail address plus `/pdf`, as [PDF](pdf.md) describes.

#### Scenario: Direct detail request
- GIVEN a stored invoice for October 2027
- WHEN `/2027/oct` is requested
- THEN the response is a complete page showing that invoice
- AND the page shows no other invoice
- AND the page links back to the invoice list
- AND the page links to `/2027/oct/pdf`
- AND the detail shows vendor, customer, project, month, year, invoice identifier, rate, each week's date range and hours, paid status, and total

#### Scenario: Returning to the list
- GIVEN the detail page for October 2027 is open
- WHEN the user follows the link back to the list
- THEN the invoice list opens

#### Scenario: Detail for an invoice without a project
- GIVEN a stored invoice with no project
- WHEN its detail is requested
- THEN the detail omits the project

#### Scenario: Paid invoice hides the project form
- GIVEN a stored paid invoice
- WHEN its detail is requested
- THEN the detail shows the paid status
- AND the detail shows no project form

### Requirement: Missing Invoice
The app SHALL return 404 for an unknown year, an unknown month, or a month with no stored invoice. The 404 response SHALL be a complete HTML page that says the invoice was not found and links back to the invoice list.

#### Scenario: Month has no invoice
- GIVEN no stored invoice for October 2027
- WHEN `/2027/oct` is requested
- THEN the app returns 404
- AND the response is an HTML page that links to the invoice list

#### Scenario: Invalid month
- GIVEN the address `/2027/foo`
- WHEN the app handles the request
- THEN the app returns 404

### Requirement: HTML Pages
The app SHALL return a complete HTML page for every page response and SHALL NOT expose a JSON API. The PDF download SHALL follow [PDF](pdf.md).

#### Scenario: Pages are HTML
- GIVEN a stored invoice
- WHEN `/` or its detail address is requested
- THEN the response is a complete HTML page

### Requirement: Escaping
The app SHALL escape stored values in every page. It SHALL never render an invoice field as raw HTML. The PDF SHALL render stored values as text, as [PDF](pdf.md#requirement-invoice-pdf) describes.

#### Scenario: Stored markup is text
- GIVEN a vendor named `<script>alert(1)</script>`
- WHEN the invoice renders
- THEN the page shows the name as text
- AND the browser runs no script

### Requirement: Invoice Access
The app SHALL require a valid session for every page and every PDF download and SHALL allow every signed-in user to read every invoice. It SHALL change only the project and the paid status. Every signed-in user changes the project on an unpaid invoice, as [Project Edits](project-edits.md) describes. Only an admin user changes the paid status, as [Paid Status](paid-status.md#requirement-paid-status-access) describes. A paid invoice keeps its values and project until an admin marks it unpaid. Sign-in, roles, sessions, and access configuration SHALL follow [Access](access.md).

#### Scenario: Signed-in user reads the list
- GIVEN a signed-in admin or customer user
- WHEN `/` is requested
- THEN the app returns the invoice list

#### Scenario: No session
- GIVEN a request without a session
- WHEN `/` is requested
- THEN the app sends the user to `/login`

### Requirement: Web Command
`invoicer web` SHALL start the web server. The database path SHALL come from `--database`, then the `database` config value, then the default, as [Database Location](config.md#requirement-database-location) describes. `--address` SHALL set the listen address and SHALL default to `:8080`.

The server SHALL serve pages, the stylesheet, and the PDF from the binary, with the PDF font embedded. It SHALL need no support files beside the binary.

When the server starts, the command SHALL print one line that gives the app's local URL as a link. It SHALL NOT print request logs, route lists, or framework diagnostics.

The command SHALL report an error and exit non-zero when the database cannot open or the address cannot be used.

An interrupt SHALL stop the server gracefully within 10 seconds.

#### Scenario: Default address
- GIVEN no address is passed
- WHEN `invoicer web` starts
- THEN the server listens on `:8080`
- AND the command prints `invoicer web ready at http://localhost:8080/`

#### Scenario: Custom address
- GIVEN `--address 127.0.0.1:9000`
- WHEN the server starts
- THEN it listens on `127.0.0.1:9000`
- AND the command prints `invoicer web ready at http://127.0.0.1:9000/`

#### Scenario: Database flag wins
- GIVEN a config database path
- WHEN `invoicer web` runs with a different `--database` path
- THEN the app lists invoices from the flag path

#### Scenario: Binary needs no support files
- GIVEN the binary alone in an empty directory and a valid access configuration
- WHEN a signed-in user requests pages, the stylesheet, and a PDF
- THEN they load

#### Scenario: Database cannot open
- GIVEN a database path that cannot be opened
- WHEN `invoicer web` starts
- THEN the command reports an error
- AND exits non-zero

#### Scenario: Interrupt stops the server
- GIVEN a running server
- WHEN the process receives an interrupt
- THEN the server stops within 10 seconds
- AND the command exits
