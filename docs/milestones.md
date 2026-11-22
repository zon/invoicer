# Milestones

High level objectives, worked in order. See [process](zpecs/process.md).

## SQLite invoice store

The CLI writes each invoice to a SQLite database. The database stores the invoice fields and its weekly line items. Each invoice has a unique identifier. The database path is configurable. The CLI no longer generates HTML or PDF, and the `--pdf`, `--model`, `pdf`, and `model` options are removed.

## Invoice web app

A web app lists invoices from the database and renders a selected invoice from its stored data. The list shows the customer, month, project, and total. The app has no login in this milestone.

## OIDC access control

The web app signs users in with OIDC. Users in the admin group read every invoice. Client users read the customer's invoices. Requests without a valid session are rejected. There is one customer, so no group-to-customer mapping is needed.

## Client project edits

A client can change the project name on any invoice. The new name is saved and shown on the invoice.

## Paid status

An admin can mark an invoice paid and mark it unpaid again. The invoice list and detail show the paid status. Clients cannot change the status.

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
