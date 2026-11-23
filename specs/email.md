# Email Specification

## Purpose
Creating an invoice sends the customer an email that links to the invoice in the web app. The CLI and the monthly schedule send the same email.

## Requirements

### Requirement: Contact Address
Generate SHALL resolve the contact email from the `--customer-email` flag, then the `customerEmail` config value, as [Config](config.md#requirement-email-configuration) describes. It SHALL store the address with the invoice. The address MAY be empty when email sending is off. An edit SHALL store the new address.

The send SHALL use the stored address, not the current config.

#### Scenario: Flag wins over config
- GIVEN a config with a `customerEmail`
- WHEN generate runs with a different `--customer-email`
- THEN the invoice stores the flag address

#### Scenario: Edit stores the new address
- GIVEN a stored invoice with one contact email
- WHEN generate runs for the same month with another address
- THEN the stored invoice holds the new address

#### Scenario: Send uses the stored address
- GIVEN a stored invoice with a contact email
- WHEN generate sends
- THEN the recipient is the stored contact email

### Requirement: Send Once
Generate SHALL save the invoice, print its identifier, and then send the email. It SHALL print the recipient and the Resend email identifier after a successful send.

The store SHALL record the send time on the invoice. Generate SHALL send when the invoice has no send time, whether it created or edited the invoice. A run after a failed send SHALL send again. A run after a successful send SHALL send nothing. Editing a sent invoice SHALL not send again.

#### Scenario: Created invoice sends
- GIVEN no stored invoice for the month
- WHEN generate succeeds
- THEN it sends one email
- AND it records the send time
- AND the output names the recipient and the email identifier

#### Scenario: Second run sends nothing
- GIVEN a stored invoice with a send time
- WHEN generate edits it
- THEN it sends no email
- AND the invoice keeps its send time

#### Scenario: Failed send sends again
- GIVEN a stored invoice with no send time
- WHEN generate runs again
- THEN it sends the email

#### Scenario: Existing invoice sends on a later run
- GIVEN a stored invoice with no contact email and no send time
- WHEN generate runs for its month
- THEN it stores the contact email
- AND it sends the email

#### Scenario: Paid invoice sends nothing
- GIVEN a stored paid invoice
- WHEN generate runs for its month
- THEN it sends no email

### Requirement: Message
The message SHALL have an HTML body and a plain-text alternative, with one recipient and no attachments. The subject SHALL be `Invoice for October 2026`, with the invoice's month and year.

The HTML body SHALL follow this format:

```html
<p>Invoice 0123456789 for October 2026 from Jane Smith.</p>
<p>Total: $4800.00</p>
<p><a href="https://invoices.example.com/2026/oct">View the invoice</a></p>
```

Stored values SHALL be HTML-escaped, so a vendor named `<b>Jane</b>` shows as text. The plain-text alternative SHALL hold the same content, with the link written out after `View the invoice: `.

The identifier SHALL appear in uppercase, as [Invoice Identity](invoices.md#requirement-invoice-identity) describes. The total SHALL follow [Invoice Amount](invoices.md#requirement-invoice-amount) and SHALL show dollars with two decimals.

The link SHALL be `baseUrl` with no trailing slash plus the detail path, as [Web](web.md#requirement-invoice-detail) describes. The link SHALL need sign-in, as [Access](access.md) describes. The sender SHALL be `resend.from`.

#### Scenario: Message content
- GIVEN an invoice for October 2026 with a total of $4800.00
- WHEN the email sends
- THEN the subject is `Invoice for October 2026`
- AND the HTML body names the uppercase identifier, month, year, vendor, total, and clickable link

#### Scenario: Trailing slash
- GIVEN a `baseUrl` ending in a slash
- WHEN the email sends
- THEN the link has one slash before the detail path

#### Scenario: Stored markup is text
- GIVEN a vendor named `<b>Jane</b>`
- WHEN the email sends
- THEN the vendor appears as text in the HTML body
- AND the vendor appears as written in the plain-text alternative

### Requirement: Delivery
The system SHALL send through the Resend REST API over HTTPS. It SHALL NOT use the SMTP relay. It SHALL read the API key from the file named by `resend.apiKeyFile`.

The system SHALL record the send time after Resend accepts the message and returns the email identifier.

Each request SHALL carry the idempotency key `invoice/<identifier>`, using the stored identifier. A retry within 24 hours SHALL return the first result and SHALL send no second email. The key SHALL expire after 24 hours, so the stored send time SHALL remain the guard against a repeat.

Sending SHALL have a timeout. A timeout SHALL fail the send.

#### Scenario: Accepted message records the send time
- GIVEN Resend accepts a message
- WHEN it returns the email identifier
- THEN the invoice records the send time

#### Scenario: Idempotent retry
- GIVEN Resend accepted a message whose response was lost
- WHEN generate retries within 24 hours
- THEN Resend returns the first result
- AND no second email is sent
- AND the invoice records the send time

#### Scenario: Timeout fails the send
- GIVEN a send that does not finish within the timeout
- WHEN the timeout expires
- THEN the send fails
- AND the invoice records no send time

### Requirement: Send Failure
A failed send SHALL report an error and exit non-zero. The invoice SHALL stay stored with no send time, so a later run SHALL send it.

A failure to record the send time SHALL report an error. The system SHALL prefer a second email over a missing one.

#### Scenario: Send error leaves the invoice
- GIVEN a stored invoice and a send that Resend rejects
- WHEN generate finishes
- THEN the command reports an error
- AND the command exits non-zero
- AND the invoice stays stored with no send time

#### Scenario: Recording error reports failure
- GIVEN Resend accepts a message but the store cannot record the send time
- WHEN generate finishes
- THEN the command reports an error
- AND a later run sends the email again

### Requirement: Optional Sending
When the config sets no `resend` settings, generate SHALL store the contact email and send nothing.

When the config sets any `resend` setting, generate SHALL need `customerEmail`, `baseUrl`, `resend.apiKeyFile`, and `resend.from`, as [Config](config.md#requirement-email-configuration) describes. Generate SHALL fail before it saves when one is missing.

#### Scenario: No Resend settings
- GIVEN a config with no `resend` settings
- WHEN generate runs
- THEN it stores the invoice
- AND it sends no email

#### Scenario: Incomplete settings
- GIVEN a config with some `resend` settings and a missing required value
- WHEN generate runs
- THEN it reports an error before it saves
- AND it stores no invoice
