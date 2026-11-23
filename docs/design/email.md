# Email Design

Creating an invoice sends the customer an email that links to the invoice in the [web app](web.md). The [CLI](invoices.md#generate) and the monthly [schedule](schedule.md) send the same email.

## Address

An invoice records the customer's contact email. Generate gets the address from the `--customer-email` flag, then the `customerEmail` config value. It stores the address with the invoice. An edit stores the new address. See [Config](config.md#email).

The send uses the stored address, not the current config.

## Send

Generate saves the invoice, prints its identifier, and then sends the email. It prints the recipient and the Resend email identifier after a successful send. The send runs after the save transaction, so the database lock is not held during the network call.

The store records the send time on the invoice. Generate sends when the invoice has no send time, whether it created or edited the invoice. A run after a failed send sends again. A run after a successful send sends nothing, so a schedule retry does not send a second email. Editing a sent invoice does not send again.

Existing invoices keep no contact email and no send time. A later generate stores the contact email and sends. Generate does not edit a paid invoice, so it sends no email for one.

## Failure

A failed send reports an error and exits nonzero. The invoice stays stored with no send time, so a later run sends it. A failure to record the send time also reports an error. The system prefers a second email over a missing one.

## Message

The message is HTML with a plain-text alternative, so customer names never render as markup. It has one recipient and no attachments. The subject is `Invoice for October 2026`.

```html
<p>Invoice 0123456789 for October 2026 from Jane Smith.</p>
<p>Total: $4800.00</p>
<p><a href="https://invoices.example.com/2026/oct">View the invoice</a></p>
```

Stored values are HTML-escaped, so a vendor named `<b>Jane</b>` shows as text. The plain-text alternative holds the same content, with the link written out after `View the invoice: `.

The link is `baseUrl` with no trailing slash plus the detail path, like `/2026/oct`. It opens the invoice detail and needs sign-in, as [Access](access.md) describes. The customer group may sign in.

The total comes from stored data, as [Invoices](invoices.md) describes, and shows in dollars with two decimals. The sender is `resend.from`.

## Delivery

The app sends with the Resend REST API over HTTPS, through the official Go SDK. It does not use the SMTP relay. It reads the API key from a file. The key belongs to a Resend team with a verified sending domain. See [Stack](../stack.md).

The SDK returns the email identifier when Resend accepts the message. The app records the send time after that response.

Each request carries an idempotency key, `invoice/<identifier>`. The key backs up the send time for an ambiguous failure. A retry within 24 hours returns the first result and sends no second email. The key expires after 24 hours, so the stored send time remains the main guard against a repeat.

Sending has a timeout. A timeout fails the send.

## Configuration

Email sending is optional. Without `resend` settings, generate stores the contact email and sends nothing. Generate fails before it saves when the email settings are incomplete. [Config](config.md#email) describes the values.

## Deployment

The [chart](chart.md#config) renders the contact email, `resend.from`, and the API key path into the config file. The schedule pod mounts the API key from a Secret at `/run/secrets/resend-api-key`. `baseUrl` comes from `publicURL`, as [Chart](chart.md) describes.
