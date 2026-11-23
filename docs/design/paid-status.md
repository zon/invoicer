# Paid Status Design

An invoice records whether the customer has paid it. The status is paid or unpaid. A new invoice is unpaid. An admin user changes the status. Every signed-in user sees it.

A paid invoice is locked. The system does not replace it or change its project until an admin marks it unpaid.

## Data

The paid status belongs to the stored invoice. The status is not generated data. A paid invoice keeps its values and weeks. The status does not record when the invoice was paid.

## Lock

Generating an invoice for a paid month fails and stores nothing. A project edit for a paid invoice fails with 409. The invoice unlocks when an admin marks it unpaid.

## Store

The store adds a paid flag to the invoice table. Existing invoices are unpaid. The store sets the status for a year and month in one statement. It changes only the flag and keeps the identifier and weeks. It fails with a not-found error when the month holds no invoice. The store leaves the invoice unchanged when the write fails.

The store refuses to replace a paid invoice and refuses to set its project. It reports that the invoice is paid and leaves the invoice unchanged. Setting the status back to unpaid succeeds.

## Display

The invoice list shows a Status column with Paid or Unpaid. The invoice detail shows Paid or Unpaid.

## Form

The detail shows an admin user a form with one button. The button says Mark paid when the invoice is unpaid and Mark unpaid when it is paid. The form posts one `paid` field with `true` or `false` to the paid path, like `/2027/oct/paid`.

The project form shows only while the invoice is unpaid, as [Project Edits](project-edits.md) describes.

A customer user sees the status and no form. Without sign-in, the app treats the request as an admin user, as [Access](access.md) describes.

## Response

A saved status redirects to the detail page with 303. The detail and the invoice list show the new status.

A POST for an unknown year or month returns 404. A `paid` value other than `true` or `false` returns 400. A failed store write returns 500 and leaves the invoice unchanged. A customer POST returns 403 and leaves the invoice unchanged. A POST without a session goes to sign-in, which returns to the detail page without the change.

## Safety

The paid route accepts only POST. The session cookie is SameSite=Lax, so a browser leaves it out of a cross-site form post. The app needs no separate token for the change.
