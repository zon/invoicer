# Chart Design

The Helm chart deploys the web app. The chart lives at `deploy/chart` and deploys the [container image](container.md). It targets the lab cluster.

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
  project: Website Redesign
  rate: 150
  hours: 40
  database: /data/invoices.db

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

An empty `image.tag` uses the chart app version. The chart attaches named pull secrets to the pod. The chart version and app version follow [Release](release.md).

## Config

The chart renders one config file from `config`, `publicURL`, and the access settings. It mounts the file read-only at the image config path, `/home/invoicer/.invoicer/config.yaml`.

The config holds `vendor`, `customer`, `project`, `rate`, `hours`, and `database`, as [Config](config.md) describes. It also holds `baseUrl` and the `oidc` settings, as [Access](access.md#configuration) describes. `baseUrl` comes from `publicURL`. `database` defaults to `/data/invoices.db`.

The config names the mounted paths `/run/secrets/oidc-client-secret` and `/run/secrets/cookie-key`.

The chart does not hold secrets. It mounts the client secret and cookie key from the existing Secret named by `access.secretName` at those paths. The keys default to `client-secret` and `cookie-key`, which the lab's `scripts/invoicer-oidc-secret.sh` writes.

The chart owns the config file. A deployment does not use `invoicer set config`.

## Storage

The chart creates a PersistentVolumeClaim and mounts it at `/data`, where the database lives. The claim uses the `reliable` Longhorn class, because the database needs backups. `data.existingClaim` reuses a claim instead of creating one. `data.size` defaults to 1Gi.

The volume holds the database only.

The pod runs as user 1000 with group 1000 and fsGroup 1000, so the app can write the database.

The chart runs one web pod with the `Recreate` strategy. SQLite allows one writer, and the claim is ReadWriteOnce.

## Web App

The Deployment runs in the `invoicer` namespace and starts the container with the `web` argument. The chart does not create the namespace. The container listens on 8080. TCP readiness and liveness probes use that port.

The chart applies a non-root security context with a read-only root filesystem, the default seccomp profile, and no Linux capabilities.

The Deployment carries a checksum of the config, so a config change restarts the pod.

The Service is ClusterIP on `service.port` and forwards to 8080.

The chart sets a grace period that covers the web app's shutdown timeout, as [Web](web.md) describes.

## Ingress and TLS

The chart enables Ingress by default. It uses the `traefik` class and routes `hostname` to the Service.

The chart creates a cert-manager Certificate for `hostname` with the `lets-encrypt` ClusterIssuer. cert-manager stores the certificate in `invoicer-tls`, and the Ingress serves it. Set `certificate.enabled` to false when TLS terminates before the Ingress.

The lab Terraform owns the DNS record for `hostname`.

`publicURL` is the public address of the web app. Its host is `hostname`. The OIDC client registers `publicURL` plus `/callback` as the callback address.

## Out of Scope

* Building and publishing the image, which belongs to [Release](release.md)
* Vulnerability scanning
* The monthly schedule
* DNS records
* TLS certificates outside cert-manager
* Database backups
