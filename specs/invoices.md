# Invoices Specification

## Purpose
Invoices record one customer's contract work for one calendar month. The invoice database stores them.

## Requirements

### Requirement: Invoice Storage
The system SHALL save every generated invoice and its weekly line items in the invoice database. The stored invoice is the source of truth. Generating an invoice SHALL write no HTML, PDF, or other invoice files.

Saving an invoice and its weeks SHALL be one transaction. A failed save SHALL leave any previously stored invoice unchanged.

#### Scenario: New invoice is stored
- GIVEN no invoice exists for the year and month
- WHEN an invoice is generated
- THEN the system stores the invoice fields and every weekly line item
- AND the system writes no invoice file

#### Scenario: Failed save leaves the old invoice
- GIVEN a stored invoice
- WHEN a save for the same invoice fails
- THEN the previously stored invoice and its weeks remain unchanged

### Requirement: Invoice Identity
The store SHALL assign an opaque string identifier to an invoice when it first saves the invoice. The identifier SHALL never change.

A year and month SHALL identify an invoice. The database SHALL hold at most one invoice per year and month. Saving an invoice for an existing year and month SHALL replace the stored values and weekly line items, and SHALL keep the identifier. The new customer and project SHALL replace the stored customer and project.

#### Scenario: First save assigns an identifier
- GIVEN no invoice exists for the year and month
- WHEN the store saves an invoice
- THEN the store assigns an identifier
- AND the store returns the identifier to the caller

#### Scenario: Saving the same invoice replaces it
- GIVEN a stored invoice
- WHEN the store saves an invoice for the same year and month with a new customer, project, rate, or hours
- THEN the store replaces the stored values and weeks
- AND the identifier stays the same

#### Scenario: Project change replaces the invoice
- GIVEN a stored invoice for a project
- WHEN the store saves an invoice for the same year and month with a different project
- THEN one invoice remains for that month
- AND the stored project changes
- AND the identifier stays the same

### Requirement: Invoice Amount
An invoice total SHALL be the sum of each week's hours multiplied by the invoice's hourly rate. The system SHALL compute the total from stored data and SHALL NOT store it.

#### Scenario: Total follows stored data
- GIVEN a stored invoice with weekly hours and an hourly rate
- WHEN the invoice total is requested
- THEN the total is the sum of each week's hours times the rate

#### Scenario: Replacement changes the total
- GIVEN a stored invoice
- WHEN the invoice is replaced with different weekly hours or rate
- THEN the total reflects the replacement values

### Requirement: Generate Output
Generating an invoice SHALL save it and print whether the invoice was created or edited, the invoice's month and year, and its identifier. Generate SHALL NOT print file paths or report HTML or PDF output.

#### Scenario: Successful generation
- GIVEN valid vendor, customer, rate, and hours
- WHEN a new invoice is generated
- THEN the system stores the invoice
- AND the output reports that it was created, with the month, year, and identifier

#### Scenario: Existing invoice is regenerated
- GIVEN a stored invoice
- WHEN generate creates the same year and month again
- THEN the output reports that it was edited, with the same identifier

#### Scenario: Invalid input stores nothing
- GIVEN an invalid month, or a missing required value
- WHEN an invoice is generated
- THEN generate reports an error
- AND generate stores no invoice
