# Chart Design

The Helm chart deploys the web app and the monthly schedule. The chart lives at `deploy/chart` and deploys the [container image](container.md). It targets the lab cluster.

## Values

```yaml
image:
  repository: ghcr.io/zon/invoicer
  tag: ""
  pullPolicy: IfNotPresent
  pullSecrets: []

hostname: invoices.haralovich.com
publicURL: https://invoices.haralovich.com

config:
  vendor: Jane Smith
  customer: Acme Corp
  customerEmail: billing@acme.com
  project: Website Redesign
  rate: 150
  hours: 40
  database: /data/invoices.db

resend:
  from: invoicer@example.com
  secretName: invoicer-resend
  apiKeyKey: api-key

access:
  issuer: https://id.haralovich.org
  clientId: invoicer
  adminGroup: invoicer-admins
  customerGroup: invoicer-customers
  groupsClaim: groups
  secretName: invoicer-oidc
  clientSecretKey: client-secret
  cookieKeyKey: cookie-key

data:
  size: 1Gi
  storageClass: reliable
  existingClaim: ""

schedule:
  enabled: true
  cron: "0 9 1 * *"

service:
  port: 80

ingress:
  enabled: true
  className: traefik

certificate:
  enabled: true
  name: invoicer
  issuer: lets-encrypt

resources:
  requests:
    cpu: 50m
    memory: 64Mi
  limits:
    cpu: 500m
    memory: 256Mi
```

An empty `image.tag` uses the chart app version. The chart attaches named pull secrets to each pod. The chart version and app version follow [Release](release.md).

## Config

The chart renders one config file from `config`, `publicURL`, the access settings, and the Resend settings. It mounts the file read-only at the image config path, `/home/invoicer/.invoicer/config.yaml`.

The config holds `vendor`, `customer`, `customerEmail`, `project`, `rate`, `hours`, and `database`, as [Config](config.md) describes. It also holds `baseUrl` and the `oidc` settings, as [Access](access.md#configuration) describes. `baseUrl` comes from `publicURL`. `database` defaults to `/data/invoices.db`.

When `resend.from` is set, the config holds the `resend` block, as [Email](email.md#configuration) describes. The schedule pod mounts the API key at `/run/secrets/resend-api-key`. The web pod does not send email and mounts no Resend secret.

The config names the mounted paths `/run/secrets/oidc-client-secret` and `/run/secrets/cookie-key`. When it sets Resend, it also names `/run/secrets/resend-api-key`.

The chart does not hold secrets. It mounts the client secret and cookie key from the existing Secret named by `access.secretName` at those paths. The keys default to `client-secret` and `cookie-key`, which the lab's `scripts/invoicer-oidc-secret.sh` writes. It mounts the Resend API key from the existing Secret named by `resend.secretName`, with key `resend.apiKeyKey`.

The chart owns the config file. A deployment does not use `invoicer set config`.

## Storage

The chart creates a PersistentVolumeClaim and mounts it at `/data`, where the database lives. The claim uses the `reliable` Longhorn class, because the database needs backups. `data.existingClaim` reuses a claim instead of creating one. `data.size` defaults to 1Gi.

The volume holds the database only.

The pods run as user 1000 with group 1000 and fsGroup 1000, so the app can write the database.

The chart runs one web pod with the `Recreate` strategy, so a rollout releases the claim before the next pod mounts it. The claim is ReadWriteOnce, so the schedule pod runs on the web pod's node.

## Web App

The Deployment runs in the `invoicer` namespace and starts the container with the `web` argument. The chart does not create the namespace. The container listens on 8080. TCP readiness and liveness probes use that port.

The chart applies a non-root security context with a read-only root filesystem, the default seccomp profile, and no Linux capabilities.

The Deployment carries a checksum of the config, so a config change restarts the pod.

The Service is ClusterIP on `service.port` and forwards to 8080.

The chart sets a grace period that covers the web app's shutdown timeout, as [Web](web.md) describes.

## Schedule

The chart renders a CronWorkflow when `schedule.enabled` is true. `schedule.cron` sets the time and defaults to 09:00 UTC on the first day of each month. The workflow uses UTC, matching the invoice month and week dates.

The CronWorkflow runs one pod from the same image with the `generate` argument. The pod mounts the config file and the data claim, as the web app does. It carries the image pull secrets and the same security context and resources. It uses pod affinity to run on the web pod's node, because the claim is ReadWriteOnce.

The chart does not install Argo Workflows. A cluster without Argo sets `schedule.enabled` to false and runs the CLI by hand.

## Ingress and TLS

The chart enables Ingress by default. It uses the `traefik` class and routes `hostname` to the Service.

The chart creates a cert-manager Certificate for `hostname` with the `lets-encrypt` ClusterIssuer. cert-manager stores the certificate in `invoicer-tls`, and the Ingress serves it. Set `certificate.enabled` to false when TLS terminates before the Ingress.

The lab Terraform owns the DNS record for `hostname`.

`publicURL` is the public address of the web app. Its host is `hostname`. The OIDC client registers `publicURL` plus `/callback` as the callback address.
