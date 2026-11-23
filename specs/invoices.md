# Invoices Specification

## Purpose
Invoices record one customer's contract work for one calendar month. The invoice database stores them and their paid status.

## Requirements

### Requirement: Invoice Storage
The system SHALL save every generated invoice and its weekly line items in the invoice database. The stored invoice is the source of truth. Generating an invoice SHALL write no HTML, PDF, or other invoice files.

A stored invoice SHALL record a paid status. A new invoice SHALL start unpaid.

Saving an invoice and its weeks SHALL be one transaction. A failed save SHALL leave any previously stored invoice unchanged.

#### Scenario: New invoice is stored
- GIVEN no invoice exists for the year and month
- WHEN an invoice is generated
- THEN the system stores the invoice fields and every weekly line item
- AND the invoice's paid status is unpaid
- AND the system writes no invoice file

#### Scenario: Failed save leaves the old invoice
- GIVEN a stored invoice
- WHEN a save for the same invoice fails
- THEN the previously stored invoice and its weeks remain unchanged

### Requirement: Invoice Identity
The store SHALL assign an opaque string identifier to an invoice when it first saves the invoice. The store SHALL keep the identifier in lowercase, and every display of the identifier SHALL use uppercase. The identifier SHALL never change.

A year and month SHALL identify an invoice. The database SHALL hold at most one invoice per year and month. Saving an invoice for an existing year and month SHALL replace the stored values and weekly line items, and SHALL keep the identifier. The new customer and project SHALL replace the stored customer and project. A paid invoice SHALL NOT be replaced. A save for a paid month SHALL fail and leave the stored invoice unchanged.

#### Scenario: First save assigns an identifier
- GIVEN no invoice exists for the year and month
- WHEN the store saves an invoice
- THEN the store assigns an identifier
- AND the store returns the identifier to the caller

#### Scenario: Identifier displays in uppercase
- GIVEN a stored invoice whose identifier is lowercase
- WHEN generate prints the identifier, the detail page shows it, or the PDF renders it
- THEN the identifier appears in uppercase
- AND the stored identifier stays lowercase

#### Scenario: Saving the same invoice replaces it
- GIVEN a stored unpaid invoice
- WHEN the store saves an invoice for the same year and month with a new customer, project, rate, or hours
- THEN the store replaces the stored values and weeks
- AND the identifier stays the same

#### Scenario: Paid invoice is not replaced
- GIVEN a stored paid invoice
- WHEN an invoice is generated for the same year and month
- THEN the save fails
- AND the stored values, weeks, and paid status stay the same

#### Scenario: Project change replaces the invoice
- GIVEN a stored unpaid invoice for a project
- WHEN the store saves an invoice for the same year and month with a different project
- THEN one invoice remains for that month
- AND the stored project changes
- AND the identifier stays the same

#### Scenario: Opening a database with duplicate months
- GIVEN a database holding several invoices for one year and month
- WHEN the store opens the database
- THEN the most recently created invoice remains for that month
- AND the other invoices and their weeks are removed

### Requirement: Weekly Line Items
The system SHALL assign a week to the month that holds its Wednesday. A week SHALL run Monday through Sunday. Each week SHALL bill the full configured hours per week.

#### Scenario: Week boundaries follow the Wednesday
- GIVEN January 2025, whose first day is a Wednesday
- WHEN the system builds the weeks for January 2025
- THEN the first week runs from Monday December 30, 2024 through Sunday January 5, 2025

#### Scenario: Weeks from adjacent months are excluded
- GIVEN March 2025, whose first Wednesday is March 5
- WHEN the system builds the weeks for March 2025
- THEN no week contains a Wednesday in February 2025
- AND every week contains a Wednesday in March 2025

#### Scenario: Every week bills full hours
- GIVEN April 2026 with five Wednesdays and 20 hours per week
- WHEN the system builds the weeks for April 2026
- THEN the invoice holds five weeks
- AND each week bills 20 hours

### Requirement: Invoice Amount
An invoice total SHALL be the sum of each week's hours multiplied by the invoice's hourly rate. The system SHALL compute the total from stored data and SHALL NOT store it.

#### Scenario: Total follows stored data
- GIVEN a stored invoice with weekly hours and an hourly rate
- WHEN the invoice total is requested
- THEN the total is the sum of each week's hours times the rate

#### Scenario: Replacement changes the total
- GIVEN a stored unpaid invoice
- WHEN the invoice is replaced with different weekly hours or rate
- THEN the total reflects the replacement values

### Requirement: Generate Output
Generating an invoice SHALL save it and print whether the invoice was created or edited, the invoice's month and year, and its identifier. Generate SHALL NOT print file paths or report HTML or PDF output.

Generate SHALL default to the previous calendar month in UTC.

When the stored invoice for the month is paid, generate SHALL report that the invoice is paid, store nothing, and exit non-zero.

#### Scenario: Successful generation
- GIVEN valid vendor, customer, rate, and hours
- WHEN a new invoice is generated
- THEN the system stores the invoice
- AND the output reports that it was created, with the month, year, and identifier

#### Scenario: Default month
- GIVEN no month or year
- WHEN generate runs on 1 April 2026
- THEN it generates the invoice for March 2026

#### Scenario: Paid invoice blocks generate
- GIVEN a stored paid invoice
- WHEN generate creates the same year and month
- THEN generate reports that the invoice is paid
- AND generate stores no change
- AND the command exits non-zero

#### Scenario: Existing invoice is regenerated
- GIVEN a stored unpaid invoice
- WHEN generate creates the same year and month again
- THEN the output reports that it was edited, with the same identifier

#### Scenario: Invalid input stores nothing
- GIVEN an invalid month, or a missing required value
- WHEN an invoice is generated
- THEN generate reports an error
- AND generate stores no invoice

### Requirement: Invoice Removal
The system SHALL delete a stored invoice and its weekly line items when remove receives the invoice's identifier. Remove SHALL accept the identifier in either case. Remove SHALL delete a paid invoice as well as an unpaid one.

Remove SHALL report an error and exit non-zero when no invoice has the identifier, and SHALL delete nothing.

#### Scenario: Stored invoice is removed
- GIVEN a stored invoice
- WHEN remove receives its identifier
- THEN the invoice and its weekly line items are deleted
- AND the output reports the invoice's month, year, and identifier

#### Scenario: Identifier case does not matter
- GIVEN a stored invoice whose identifier is lowercase
- WHEN remove receives the identifier in uppercase
- THEN the invoice is deleted

#### Scenario: Paid invoice is removed
- GIVEN a stored paid invoice
- WHEN remove receives its identifier
- THEN the invoice is deleted

#### Scenario: Unknown identifier removes nothing
- GIVEN stored invoices
- WHEN remove receives an identifier that no invoice has
- THEN remove reports an error
- AND no invoice is deleted
- AND the command exits non-zero
