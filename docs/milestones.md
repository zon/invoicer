# Milestones

High level objectives. Work through them in order. See [process](zpecs/process.md).

## Container and chart

One container image runs the CLI and the web app. A Helm chart deploys the web app with a persistent volume for the SQLite database and its OIDC configuration. The chart can also run the CLI against the same database.

## Build and deploy pipeline

CI builds the container image, scans it for vulnerabilities, and packages the Helm chart. A deploy workflow releases the web app to the cluster. High and critical vulnerabilities stop the build.

## Monthly invoice schedule

A monthly schedule runs the CLI against the deployed database. It creates the previous month's invoice from the configured options. A second run for the same month does not create a duplicate.

## Invoice email

The system stores the customer's contact email. Creating an invoice sends an email to the customer. The email links to the invoice in the web app. The CLI and the monthly schedule both send the email when they create an invoice.
