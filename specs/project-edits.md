# Project Edits Specification

## Purpose
Signed-in users change the project on an unpaid stored invoice from the invoice detail page.

## Requirements

### Requirement: Project Edit Form
The invoice detail SHALL show a project form while the invoice is unpaid. The form SHALL hold a text input with the current project and a Save button. It SHALL post one `project` field to the detail path, like `/2027/oct`. A paid invoice SHALL show no project form.

#### Scenario: Detail shows the current project
- GIVEN a stored unpaid invoice for October 2027 with a project
- WHEN `/2027/oct` is requested
- THEN the detail shows the project in a text input
- AND the form posts to `/2027/oct`

#### Scenario: Detail shows an empty input
- GIVEN a stored unpaid invoice with no project
- WHEN its detail is requested
- THEN the text input is empty

#### Scenario: Paid invoice hides the form
- GIVEN a stored paid invoice
- WHEN its detail is requested
- THEN the detail shows no project form

### Requirement: Project Update
The app SHALL accept a project only from a POST to the detail path while the invoice is unpaid. It SHALL trim whitespace around the value and SHALL save the result. An empty result SHALL clear the project.

A save SHALL change only the project. The identifier, year, month, customer, rate, weeks, and total SHALL stay the same. After a save, the app SHALL redirect with 303 to the invoice detail.

A POST for an unknown year, an unknown month, or a month with no invoice SHALL return 404. A POST for a paid invoice SHALL return 409 and SHALL keep the project. A failed save SHALL return 500 and leave the invoice unchanged. A POST without a session SHALL go to sign-in and SHALL return to the detail page without saving.

#### Scenario: Change the project
- GIVEN a stored unpaid invoice for October 2027 with a project
- WHEN `/2027/oct` receives a POST with a new project
- THEN the app saves the project
- AND the app redirects with 303 to `/2027/oct`
- AND the detail and the invoice list show the new project
- AND the invoice keeps its identifier, weeks, and total

#### Scenario: Paid invoice rejects the edit
- GIVEN a stored paid invoice for October 2027 with a project
- WHEN `/2027/oct` receives a POST with a new project
- THEN the app returns 409
- AND the invoice keeps its project
- AND the other stored values stay the same

#### Scenario: Clear the project
- GIVEN a stored unpaid invoice with a project
- WHEN its form posts an empty project
- THEN the app clears the project
- AND the detail omits the project

#### Scenario: Trim whitespace
- GIVEN a stored unpaid invoice
- WHEN its form posts a project with leading or trailing whitespace
- THEN the app saves the project without the surrounding whitespace

#### Scenario: Missing invoice
- GIVEN no stored invoice for October 2027
- WHEN `/2027/oct` receives a POST with a project
- THEN the app returns 404

#### Scenario: Invalid month
- GIVEN the address `/2027/foo`
- WHEN the app receives a POST for it
- THEN the app returns 404

#### Scenario: Failed save
- GIVEN a store that fails to save
- WHEN its form posts a project
- THEN the app returns 500
- AND the invoice keeps its stored project

#### Scenario: Post without a session
- GIVEN a request without a session
- WHEN `/2027/oct` receives a POST with a project
- THEN the app sends the user to `/login`
- AND the app returns to `/2027/oct` after sign-in
- AND the invoice keeps its stored project
