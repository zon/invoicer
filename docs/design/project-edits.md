# Project Edits Design

A signed-in user changes the project on any unpaid stored invoice. The edit changes only the project. The identifier, year, month, customer, rate, and weekly line items stay the same. The total stays the same. A paid invoice keeps its project.

## Access

Admin users and customer users change the project on any unpaid invoice. A paid invoice rejects the edit. Read access stays the same, as [Access](access.md) describes. The project edit and the paid status are the only writes the web app offers. See [Paid Status](paid-status.md).

## Store

The store sets the project for a year and month in one statement. It touches only the project column. It keeps the identifier and the weeks. It fails with a not-found error when the month holds no invoice. It fails when the invoice is paid. The store leaves the invoice unchanged when the write fails.

## Form

The detail page shows the project in a form with a text input and a Save button while the invoice is unpaid. The input starts with the current project. The form posts one `project` field to the detail path, like `/2027/oct`. A paid invoice shows no form.

The app trims whitespace around the value. An empty value clears the project.

## Response

A saved edit redirects to the detail page with 303. The page shows the new project. The invoice list shows the new project on the next request.

A POST for an unknown year or month returns 404. A POST for a paid invoice returns 409 and leaves the project unchanged. A failed store write returns an error and leaves the invoice unchanged. A POST without a session goes to sign-in, which returns to the detail page without the edit.

## Safety

The edit route accepts only POST. The session cookie is SameSite=Lax, so a browser leaves it out of a cross-site form post. The app needs no separate token for the edit.

## Out of Scope

* Editing other invoice fields
* Editing from the invoice list
* A project history or audit trail
* Conflict detection when two users edit one invoice
* Restricting a customer user to their own invoices
* JavaScript
