# Web Specification

## Purpose
The web app lists invoices from the invoice database and renders a selected invoice from its stored data. It does not write invoices.

## Requirements

### Requirement: Invoice List
The app SHALL serve the invoice list at `/`. The list SHALL show every stored invoice with its customer, month and year, project when set, and total. It SHALL order invoices newest first by year and month. Each invoice SHALL link to its detail.

The total SHALL follow [Invoice Amount](invoices.md#requirement-invoice-amount).

#### Scenario: List shows every invoice
- GIVEN stored invoices for several months
- WHEN `/` is requested
- THEN the page lists every invoice with its customer, month, year, project, and total

#### Scenario: Newest invoice comes first
- GIVEN invoices for October 2027 and January 2028
- WHEN `/` is requested
- THEN the January 2028 invoice appears before the October 2027 invoice

#### Scenario: No stored invoices
- GIVEN an empty database
- WHEN `/` is requested
- THEN the page shows an empty list

### Requirement: Invoice Detail
The detail address SHALL use the year and three-letter lowercase month name, like `/2027/oct`. The detail SHALL show vendor, customer, project when set, month, year, hourly rate, every weekly line item, and total. Each weekly line item SHALL show its date range and hours.

The list and the selected detail SHALL share one page. The list SHALL stay visible and the detail SHALL render in a panel beside it. A direct request SHALL return the complete page with that invoice selected.

#### Scenario: Direct detail request
- GIVEN a stored invoice for October 2027
- WHEN `/2027/oct` is requested
- THEN the response is the complete page
- AND the list stays visible
- AND the October 2027 invoice is selected in the detail panel
- AND the detail shows vendor, customer, project, month, year, rate, each week's date range and hours, and total

#### Scenario: Detail for an invoice without a project
- GIVEN a stored invoice with no project
- WHEN its detail is requested
- THEN the detail omits the project

### Requirement: Missing Invoice
The app SHALL return 404 for an unknown year, an unknown month, or a month with no stored invoice.

#### Scenario: Month has no invoice
- GIVEN no stored invoice for October 2027
- WHEN `/2027/oct` is requested
- THEN the app returns 404

#### Scenario: Invalid month
- GIVEN the address `/2027/foo`
- WHEN the app handles the request
- THEN the app returns 404

### Requirement: Partial Updates
A request with `HX-Request: true` SHALL return only the detail panel fragment. The fragment SHALL be a complete HTML element rather than a document. The app SHALL return HTML for every response and SHALL NOT expose a JSON API.

Selecting an invoice with htmx SHALL load the detail fragment into the panel and update the browser address. A missing invoice in a partial request SHALL return a not found fragment with status 404.

#### Scenario: Invoice selection updates the panel
- GIVEN the list page is open in a browser with htmx
- WHEN the user selects an invoice
- THEN the panel shows that invoice's detail
- AND the list stays unchanged
- AND the browser address changes to the invoice detail address

#### Scenario: Partial request
- GIVEN a stored invoice
- WHEN its detail address is requested with `HX-Request: true`
- THEN the response is one HTML element containing the detail
- AND the response contains no page document

#### Scenario: Missing invoice partial
- GIVEN no stored invoice for a requested month
- WHEN the detail address is requested with `HX-Request: true`
- THEN the response is a not found fragment
- AND the status is 404

#### Scenario: Selection without htmx
- GIVEN the list page is open in a browser without htmx
- WHEN the user selects an invoice
- THEN the browser opens the complete page at the invoice detail address

### Requirement: Escaping
The app SHALL escape stored values in every response. It SHALL never render an invoice field as raw HTML.

#### Scenario: Stored markup is text
- GIVEN a vendor named `<script>alert(1)</script>`
- WHEN the invoice renders
- THEN the page shows the name as text
- AND the browser runs no script

### Requirement: Read Access
The app SHALL allow every request to read every invoice. It SHALL NOT require sign-in and SHALL NOT change stored invoices.

#### Scenario: No sign-in
- GIVEN a request without a session
- WHEN `/` is requested
- THEN the app returns the invoice list

### Requirement: Web Command
`invoicer web` SHALL start the web server. The database path SHALL come from `--database`, then the `database` config value, then the default, as [Database Location](config.md#requirement-database-location) describes. `--address` SHALL set the listen address and SHALL default to `:8080`.

The server SHALL serve pages, fragments, the stylesheet, and htmx from the binary. It SHALL need no support files beside the binary.

The command SHALL report an error and exit non-zero when the database cannot open or the address cannot be used.

An interrupt SHALL stop the server gracefully.

#### Scenario: Default address
- GIVEN no address is passed
- WHEN `invoicer web` starts
- THEN the server listens on `:8080`

#### Scenario: Custom address
- GIVEN `--address 127.0.0.1:9000`
- WHEN the server starts
- THEN it listens on `127.0.0.1:9000`

#### Scenario: Database flag wins
- GIVEN a config database path
- WHEN `invoicer web` runs with a different `--database` path
- THEN the app lists invoices from the flag path

#### Scenario: Binary needs no support files
- GIVEN the binary alone in an empty directory
- WHEN the server starts
- THEN pages and the stylesheet load

#### Scenario: Database cannot open
- GIVEN a database path that cannot be opened
- WHEN `invoicer web` starts
- THEN the command reports an error
- AND exits non-zero

#### Scenario: Interrupt stops the server
- GIVEN a running server
- WHEN the process receives an interrupt
- THEN the server stops and the command exits
