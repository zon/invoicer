# Schedule Specification

## Purpose
The deployment generates each month's invoice without a manual run.

## Requirements

### Requirement: Monthly Run
The deployment SHALL generate the previous calendar month's invoice once a month without a manual run. The run SHALL use the configured vendor, customer, project, rate, and hours, as [Config](config.md) describes, and the deployed database.

The run SHALL use UTC and SHALL invoice the month before its start date.

#### Scenario: First day of the month
- GIVEN the schedule runs on 1 April 2026
- WHEN the run completes
- THEN it stores the invoice for March 2026

#### Scenario: Year boundary
- GIVEN the schedule runs on 1 January 2027
- WHEN the run completes
- THEN it stores the invoice for December 2026

#### Scenario: Mid-month deployment
- GIVEN a deployment installed on 15 March
- WHEN no manual run happens
- THEN the first invoice comes from the run on 1 April
- AND it covers March

### Requirement: Shared Database
The run SHALL use the same config file and database as the web app, as [Chart](chart.md#requirement-schedule-workload) describes.

#### Scenario: Web app sees the new invoice
- GIVEN a completed run
- WHEN the web app opens the invoice list
- THEN the new invoice is present

#### Scenario: Web app runs during a run
- GIVEN a running web app
- WHEN a run writes the invoice
- THEN the run waits for the database lock
- AND both pods keep the database working

### Requirement: Repeat Runs
A run for a month that holds an invoice SHALL edit the stored invoice, as [Invoices](invoices.md#requirement-invoice-identity) describes. A run SHALL NOT store a duplicate.

A run for a paid month SHALL fail and store nothing.

#### Scenario: Second run edits
- GIVEN a stored unpaid invoice for March 2026
- WHEN the schedule runs again for March 2026
- THEN the stored identifier stays the same
- AND one invoice remains for March 2026

#### Scenario: Paid month fails
- GIVEN a stored paid invoice for March 2026
- WHEN the schedule runs for March 2026
- THEN the run fails
- AND the stored invoice and its paid status stay the same

### Requirement: Failure
A failed run SHALL change nothing. The deployment SHALL retry a failed run, as [Chart](chart.md#requirement-schedule-workload) describes.

A month that still fails SHALL wait for a manual run.

#### Scenario: Retry succeeds
- GIVEN a run whose pod fails
- WHEN the retry succeeds
- THEN the invoice is stored once

#### Scenario: Failure stays visible
- GIVEN a run that fails twice
- WHEN the retry limit is reached
- THEN the failed run and its log remain
