# Invoices Specification

## Purpose
Invoices record one customer's contract work for one calendar month and are kept in the invoice database.

## Requirements

### Requirement: Invoice Storage
The system SHALL save every generated invoice and its weekly line items in the invoice database. The stored invoice is the source of truth. Generating an invoice SHALL write no HTML, PDF, or other invoice files.

Saving an invoice and its weeks SHALL be one transaction. A failed save SHALL leave any previously stored invoice unchanged.

#### Scenario: New invoice is stored
- GIVEN no invoice exists for the customer, month, and project
- WHEN an invoice is generated
- THEN the invoice fields and every weekly line item are stored
- AND no invoice file is written

#### Scenario: Failed save leaves the old invoice
- GIVEN a stored invoice
- WHEN a save for the same invoice fails
- THEN the previously stored invoice and its weeks remain unchanged

### Requirement: Invoice Identity
The store SHALL assign an opaque string identifier to an invoice when it is first saved. The identifier SHALL never change.

A customer, year, month, and project SHALL identify an invoice. Saving the same four values again SHALL replace the stored values and weekly line items, and SHALL keep the identifier. A different project SHALL identify a different invoice.

#### Scenario: First save assigns an identifier
- GIVEN no invoice exists for the customer, month, and project
- WHEN the invoice is saved
- THEN it receives an identifier
- AND the identifier is returned to the caller

#### Scenario: Saving the same invoice replaces it
- GIVEN a stored invoice
- WHEN an invoice with the same customer, year, month, and project is saved with new rate or hours
- THEN the stored values and weeks are replaced
- AND the identifier is unchanged

#### Scenario: Project separates invoices
- GIVEN a stored invoice for a project
- WHEN another invoice is saved for the same customer, year, and month with a different project
- THEN both invoices remain stored with different identifiers

### Requirement: Invoice Amount
An invoice total SHALL be the sum of each week's hours multiplied by the invoice's hourly rate. The total SHALL be computed from stored data and SHALL NOT be stored.

#### Scenario: Total follows stored data
- GIVEN a stored invoice with weekly hours and an hourly rate
- WHEN the invoice total is requested
- THEN the total is the sum of each week's hours times the rate

#### Scenario: Replacement changes the total
- GIVEN a stored invoice
- WHEN the invoice is replaced with different weekly hours or rate
- THEN the total reflects the replacement values

### Requirement: Generate Output
Generating an invoice SHALL save it and print its identifier. Generate SHALL NOT print file paths or report HTML or PDF output.

#### Scenario: Successful generation
- GIVEN valid vendor, customer, rate, and hours
- WHEN an invoice is generated
- THEN the invoice is stored
- AND the invoice identifier is printed

#### Scenario: Invalid input stores nothing
- GIVEN an invalid month, or a missing required value
- WHEN an invoice is generated
- THEN an error is reported
- AND no invoice is stored
