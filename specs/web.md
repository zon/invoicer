# Web Specification

## Purpose
The web app lists invoices from the invoice database, renders a selected invoice from its stored data, and changes an invoice's project.

## Requirements

### Requirement: Invoice List
The app SHALL serve the invoice list at `/`. The list SHALL show every stored invoice with its customer, month and year, project when set, and total. It SHALL order invoices newest first by year and month. Each invoice SHALL link to its detail page.

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
The detail address SHALL use the year and three-letter lowercase month name, like `/2027/oct`. The detail SHALL show vendor, customer, project when set, month, year, hourly rate, every weekly line item, and total. Each weekly line item SHALL show its date range and hours. The detail SHALL show the project edit form, as [Project Edits](project-edits.md#requirement-project-edit-form) describes.

The detail SHALL be a full page. It SHALL show only the invoice and SHALL link back to the invoice list.

#### Scenario: Direct detail request
- GIVEN a stored invoice for October 2027
- WHEN `/2027/oct` is requested
- THEN the response is a complete page showing that invoice
- AND the page shows no other invoice
- AND the page links back to the invoice list
- AND the detail shows vendor, customer, project, month, year, rate, each week's date range and hours, and total

#### Scenario: Returning to the list
- GIVEN the detail page for October 2027 is open
- WHEN the user follows the link back to the list
- THEN the invoice list opens

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

### Requirement: HTML Pages
The app SHALL return a complete HTML page for every response and SHALL NOT expose a JSON API.

#### Scenario: Pages are HTML
- GIVEN a stored invoice
- WHEN `/` or its detail address is requested
- THEN the response is a complete HTML page

### Requirement: Escaping
The app SHALL escape stored values in every response. It SHALL never render an invoice field as raw HTML.

#### Scenario: Stored markup is text
- GIVEN a vendor named `<script>alert(1)</script>`
- WHEN the invoice renders
- THEN the page shows the name as text
- AND the browser runs no script

### Requirement: Invoice Access
The app SHALL require a valid session for every page and SHALL allow every signed-in user to read every invoice. It SHALL NOT change stored invoices except the project, as [Project Edits](project-edits.md) describes. Sign-in, roles, sessions, and access configuration SHALL follow [Access](access.md).

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

The server SHALL serve pages and the stylesheet from the binary. It SHALL need no support files beside the binary.

When the server starts, the command SHALL print one line that gives the app's local URL as a link. It SHALL NOT print request logs, route lists, or framework diagnostics.

The command SHALL report an error and exit non-zero when the database cannot open or the address cannot be used.

An interrupt SHALL stop the server gracefully.

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
- WHEN a signed-in user requests pages and the stylesheet
- THEN they load

#### Scenario: Database cannot open
- GIVEN a database path that cannot be opened
- WHEN `invoicer web` starts
- THEN the command reports an error
- AND exits non-zero

#### Scenario: Interrupt stops the server
- GIVEN a running server
- WHEN the process receives an interrupt
- THEN the server stops and the command exits
