# Milestones

High level objectives. Work through them in order. See [process](zpecs/process.md).

## Customer project edits

A customer can change the project name on any invoice. The system saves the new name and shows it on the invoice.

## Paid status

An admin can mark an invoice paid and mark it unpaid again. The invoice list and detail show the paid status. Customers cannot change the status.

## PDF downloads

A user can download an invoice as a PDF. The PDF renders the invoice from its stored data. A user can download only invoices they can read.

## Container and chart

One container image runs the CLI and the web app. A Helm chart deploys the web app with a persistent volume for the SQLite database and its OIDC configuration. The chart can also run the CLI against the same database.

## Build and deploy pipeline

CI builds the container image, scans it for vulnerabilities, and packages the Helm chart. A deploy workflow releases the web app to the cluster. High and critical vulnerabilities stop the build.

## Monthly invoice schedule

A monthly schedule runs the CLI against the deployed database. It creates the previous month's invoice from the configured options. A second run for the same month does not create a duplicate.

## Invoice email

The system stores the customer's contact email. Creating an invoice sends an email to the customer. The email links to the invoice in the web app. The CLI and the monthly schedule both send the email when they create an invoice.
