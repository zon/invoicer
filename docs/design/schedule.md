# Schedule Design

The deployment creates each month's invoice without a manual run. An Argo workflow runs the CLI, so it generates the invoice as [Invoices](invoices.md) describes.

## Workflow

The lab cluster runs Argo Workflows, as [Stack](../stack.md) describes. The chart renders one CronWorkflow in the `invoicer` namespace. The chart does not install Argo.

The CronWorkflow starts each month at 09:00 UTC on the first day. `schedule.cron` sets the time, as [Chart](chart.md#schedule) describes. The workflow runs one pod that starts the image with the `generate` argument, the same as a manual run. The pod uses the web app's config file, image pull secrets, security context, and resources.

The CronWorkflow allows one run at a time. Argo removes a successful run after a day and keeps a failed one for its log.

## Month

The workflow and the CLI use UTC. Generate takes no month argument, so it invoices the previous calendar month. A run on 1 January invoices December.

A deployment added in the middle of a month waits for the next first day. Run the CLI to fill the month sooner.

## Database

The workflow pod mounts the database claim at `/data` and the config file at the image config path. The claim is ReadWriteOnce and the web pod holds it, so the pod uses pod affinity to run on the web pod's node. It waits while the web app runs no pod.

Both pods mount one filesystem, so SQLite's file locks keep the writers apart. The store waits on a lock instead of failing, and a run writes for a moment only. The web app reads the new invoice as soon as it is stored.

## Repeat

A run for a month that holds an invoice edits the stored invoice. It updates the vendor, customer, contact email, rate, and weeks from the config and keeps the identifier and the stored project. A second run never stores a duplicate.

A retry sends the email when an earlier run stored the invoice without sending it, as [Email](email.md) describes.

A run for a paid month fails and changes nothing. Argo shows the failure.

## Failure

The workflow retries a failed pod twice. A failed save changes nothing. A failed send leaves the invoice stored, and the retry sends it. A month that still fails waits for a manual run.
