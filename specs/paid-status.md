# Paid Status Specification

## Purpose
The web app shows whether a stored invoice is paid, an admin user changes the status, and a paid invoice stays locked until it is marked unpaid.

## Requirements

### Requirement: Paid Status Display
The invoice list SHALL show each invoice's paid status. The invoice detail SHALL show the invoice's paid status. The status SHALL read Paid or Unpaid.

#### Scenario: List shows the status
- GIVEN stored invoices, one paid and one unpaid
- WHEN `/` is requested
- THEN each invoice shows its paid status

#### Scenario: Detail shows the status
- GIVEN a paid invoice
- WHEN its detail is requested
- THEN the detail shows Paid

#### Scenario: Unpaid status
- GIVEN a stored unpaid invoice
- WHEN the list and the detail are requested
- THEN both show Unpaid

### Requirement: Paid Status Update
The invoice detail SHALL show an admin user a form with one button. The button SHALL read Mark paid when the invoice is unpaid and Mark unpaid when it is paid. The form SHALL post one `paid` field with `true` or `false` to the paid path, like `/2027/oct/paid`.

The app SHALL accept the status only from a POST to the paid path. A save SHALL change only the paid status. The identifier, year, month, customer, project, rate, weeks, and total SHALL stay the same. After a save, the app SHALL redirect with 303 to the invoice detail.

A POST for an unknown year, an unknown month, or a month with no invoice SHALL return 404. A `paid` value other than `true` or `false` SHALL return 400. A failed save SHALL return 500 and leave the invoice unchanged.

#### Scenario: Mark an invoice paid
- GIVEN a stored unpaid invoice for October 2027 and a signed-in admin user
- WHEN `/2027/oct/paid` receives a POST with `paid=true`
- THEN the app saves the paid status
- AND the app redirects with 303 to `/2027/oct`
- AND the detail and the invoice list show Paid
- AND the invoice keeps its identifier, values, weeks, and total

#### Scenario: Mark an invoice unpaid
- GIVEN a stored paid invoice for October 2027 and a signed-in admin user
- WHEN `/2027/oct/paid` receives a POST with `paid=false`
- THEN the app saves the unpaid status
- AND the detail and the invoice list show Unpaid

#### Scenario: Missing invoice
- GIVEN no stored invoice for October 2027
- WHEN `/2027/oct/paid` receives a POST
- THEN the app returns 404

#### Scenario: Invalid month
- GIVEN the paid path `/2027/foo/paid`
- WHEN the app receives a POST for it
- THEN the app returns 404

#### Scenario: Invalid value
- GIVEN a stored invoice
- WHEN its paid path receives a POST with a `paid` value other than `true` or `false`
- THEN the app returns 400
- AND the invoice keeps its stored status

#### Scenario: Failed save
- GIVEN a store that fails to save
- WHEN the paid path receives a POST
- THEN the app returns 500
- AND the invoice keeps its stored status

### Requirement: Paid Invoice Lock
A paid invoice SHALL reject changes to its generated values and its project, as [Invoices](invoices.md#requirement-invoice-identity) and [Project Edits](project-edits.md#requirement-project-update) describe. The paid status form SHALL stay available while the invoice is paid, so an admin user marks it unpaid.

#### Scenario: Paid invoice blocks generate
- GIVEN a stored paid invoice
- WHEN an invoice is generated for the same month
- THEN the change fails
- AND the invoice keeps its values, weeks, and status

#### Scenario: Mark unpaid unlocks the invoice
- GIVEN a stored paid invoice
- WHEN an admin marks it unpaid
- THEN the detail shows the project form
- AND generating the invoice replaces it

### Requirement: Paid Status Access
Every signed-in user SHALL read the paid status. Only an admin user SHALL change it. Without sign-in, the app SHALL treat the request as an admin user, as [Access](access.md#requirement-access-configuration) describes.

The detail SHALL NOT show the form to a customer user. A POST from a customer user SHALL return 403 and SHALL leave the status unchanged.

A POST without a session SHALL go to sign-in and SHALL return to the detail page without saving.

#### Scenario: Customer reads the status
- GIVEN a signed-in customer user and a stored invoice
- WHEN its detail is requested
- THEN the detail shows the paid status
- AND the detail shows no paid status form

#### Scenario: Customer cannot change the status
- GIVEN a signed-in customer user and a stored unpaid invoice
- WHEN the paid path receives a POST with `paid=true`
- THEN the app returns 403
- AND the invoice stays unpaid

#### Scenario: No sign-in mode
- GIVEN a config without access settings
- WHEN the detail is requested
- THEN the detail shows the paid status form
- AND a POST to the paid path saves the status

#### Scenario: Post without a session
- GIVEN a request without a session
- WHEN the paid path receives a POST with `paid=true`
- THEN the app sends the user to `/login`
- AND the app returns to the detail page after sign-in
- AND the invoice keeps its stored status
