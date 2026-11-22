# PDF Specification

## Purpose
The web app renders a stored invoice as a downloadable PDF from its stored data.

## Requirements

### Requirement: Invoice PDF
The PDF SHALL show the same data as the invoice detail: vendor, customer, project when set, month and year, hourly rate, every weekly line item, paid status, and total. Each weekly line item SHALL show its date range and hours. The total SHALL follow [Invoice Amount](invoices.md#requirement-invoice-amount). A paid invoice SHALL carry a paid marker.

The PDF SHALL use A4 portrait pages and SHALL contain no form, no link, and no other invoice. A stored value SHALL render as text and never as markup. A long value SHALL wrap and SHALL stay inside the page.

#### Scenario: PDF shows the stored invoice
- GIVEN a stored invoice for October 2027 with a vendor, customer, project, rate, and weekly hours
- WHEN its PDF is requested
- THEN the PDF shows the vendor, customer, project, month, year, rate, each week's date range and hours, paid status, and total
- AND the PDF shows no other invoice

#### Scenario: Total follows stored data
- GIVEN a stored invoice with weekly hours and an hourly rate
- WHEN its PDF is requested
- THEN the total is the sum of each week's hours times the rate

#### Scenario: Paid marker
- GIVEN a stored paid invoice
- WHEN its PDF is requested
- THEN the PDF shows the paid status and carries a paid marker

#### Scenario: Invoice without a project
- GIVEN a stored invoice with no project
- WHEN its PDF is requested
- THEN the PDF omits the project

#### Scenario: Long value wraps
- GIVEN a stored invoice with a project name longer than one line
- WHEN its PDF is requested
- THEN the project wraps inside the page

#### Scenario: Stored markup is text
- GIVEN a customer named `<script>alert(1)</script>`
- WHEN the invoice PDF renders
- THEN the PDF shows the name as text and contains no markup

### Requirement: PDF Download
The app SHALL serve the PDF at the invoice detail address plus `/pdf`, like `/2027/oct/pdf`. The response SHALL use the PDF media type and SHALL carry a download filename of `invoice-2027-10.pdf`. The month in the filename SHALL use two digits.

A download SHALL change nothing. Every download SHALL render the latest stored values.

#### Scenario: Download an invoice
- GIVEN a stored invoice for October 2027
- WHEN `/2027/oct/pdf` is requested
- THEN the response is a PDF file named `invoice-2027-10.pdf`

#### Scenario: Two-digit month
- GIVEN a stored invoice for January 2028
- WHEN `/2028/jan/pdf` is requested
- THEN the response is a PDF file named `invoice-2028-01.pdf`

#### Scenario: Latest stored values
- GIVEN a stored invoice whose project changes from the detail page
- WHEN its PDF is downloaded
- THEN the PDF shows the new project

#### Scenario: Download changes nothing
- GIVEN a stored invoice
- WHEN its PDF is downloaded
- THEN the invoice keeps its values, weeks, paid status, and total

### Requirement: Missing Invoice
The app SHALL return 404 for an unknown year, an unknown month, or a month with no stored invoice.

#### Scenario: Month has no invoice
- GIVEN no stored invoice for October 2027
- WHEN `/2027/oct/pdf` is requested
- THEN the app returns 404

#### Scenario: Invalid month
- GIVEN the address `/2027/foo/pdf`
- WHEN the app handles the request
- THEN the app returns 404

### Requirement: PDF Access
Every download SHALL need a valid session, as [Access](access.md#requirement-sign-in-required) describes. Any signed-in user SHALL download any invoice they can read. The paid status SHALL NOT restrict a download. A request without a session SHALL go to sign-in and SHALL return to the download address after sign-in.

#### Scenario: Signed-in user downloads
- GIVEN a signed-in admin or customer user and a stored invoice
- WHEN its PDF is requested
- THEN the app returns the PDF

#### Scenario: Paid invoice downloads
- GIVEN a stored paid invoice
- WHEN its PDF is requested
- THEN the app returns the PDF

#### Scenario: Download without a session
- GIVEN a request without a session
- WHEN `/2027/oct/pdf` is requested
- THEN the app sends the user to `/login`
- AND the app returns to `/2027/oct/pdf` after sign-in

#### Scenario: No sign-in mode
- GIVEN a config without access settings
- WHEN a PDF is requested
- THEN the app returns the PDF

### Requirement: Self-Contained Rendering
The binary SHALL render invoice PDFs. Rendering SHALL need no browser, no external tool, and no support file beside the binary. The font SHALL be embedded in the binary.

#### Scenario: Binary alone renders a PDF
- GIVEN the binary alone in an empty directory and a valid access configuration
- WHEN a signed-in user downloads an invoice PDF
- THEN the PDF renders

### Requirement: Render Failure
A failed render SHALL respond with an error and SHALL send no PDF and no partial document.

#### Scenario: Failed render
- GIVEN a PDF render that fails
- WHEN the PDF is requested
- THEN the app returns 500
- AND the response holds no PDF
