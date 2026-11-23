# Milestones

High level objectives. Work through them in order. See [process](zpecs/process.md).

## Monthly invoice schedule

A monthly schedule runs the CLI against the deployed database. It creates the previous month's invoice from the configured options. A second run for the same month does not create a duplicate.

## Invoice email

The system stores the customer's contact email. Creating an invoice sends an email to the customer. The email links to the invoice in the web app. The CLI and the monthly schedule both send the email when they create an invoice.
