# Chart Specification

## Purpose
The Helm chart deploys the web app on the lab cluster.

## Requirements

### Requirement: Chart Location and Install
The chart SHALL live at `deploy/chart` and SHALL deploy the [container image](container.md). It SHALL install into an existing `invoicer` namespace and SHALL NOT create the namespace. A default install with the required Secret SHALL produce the web Deployment, Service, PersistentVolumeClaim, Ingress, and Certificate.

#### Scenario: Default install
- GIVEN the `invoicer` namespace and the required Secret
- WHEN the chart installs with default values
- THEN it creates a web Deployment, Service, claim, Ingress, and Certificate

#### Scenario: Missing namespace
- GIVEN a cluster without the `invoicer` namespace
- WHEN the chart installs
- THEN the install fails
- AND the chart does not create the namespace

### Requirement: Values
The chart SHALL accept `image`, `hostname`, `publicURL`, `config`, `access`, `data`, `service`, `ingress`, `certificate`, and `resources`. An empty `image.tag` SHALL use the chart app version. The chart SHALL attach every `image.pullSecrets` entry to the pod.

#### Scenario: Default image tag
- GIVEN an empty `image.tag`
- WHEN the chart renders
- THEN the Deployment uses the chart app version as the image tag

#### Scenario: Image tag override
- GIVEN a set `image.tag`
- WHEN the chart renders
- THEN the Deployment uses that tag

#### Scenario: Pull secrets
- GIVEN one or more `image.pullSecrets` entries
- WHEN the chart renders
- THEN the pod names every one of them

### Requirement: Config Rendering
The chart SHALL render one config file from `config`, `publicURL`, and `access`. It SHALL mount the file read-only at `/home/invoicer/.invoicer/config.yaml`.

The config SHALL hold `vendor`, `customer`, `project`, `rate`, `hours`, and `database`, as [Config](config.md) describes. It SHALL hold `baseUrl` from `publicURL` and the `oidc` settings from `access`, as [Access](access.md#requirement-access-configuration) describes. `database` SHALL default to `/data/invoices.db`.

The config SHALL name the client secret and cookie key at `/run/secrets/oidc-client-secret` and `/run/secrets/cookie-key`.

The chart SHALL own the config file and SHALL NOT run `invoicer set config`.

#### Scenario: Default config
- GIVEN default values
- WHEN the chart renders
- THEN the config holds the configured vendor, customer, project, rate, hours, and `/data/invoices.db`
- AND the config holds `baseUrl` equal to `publicURL`
- AND the config holds the `oidc` settings
- AND the config names both secret paths

#### Scenario: Config change restarts the pod
- GIVEN an installed chart
- WHEN a config value changes
- THEN the Deployment restarts its pod

### Requirement: Secrets
The chart SHALL NOT hold secret values. It SHALL mount the existing Secret named by `access.secretName` at `/run/secrets/oidc-client-secret` and `/run/secrets/cookie-key`. The keys SHALL default to `client-secret` and `cookie-key`.

#### Scenario: Secret values come from an existing Secret
- GIVEN an existing Secret with a client secret and a cookie key
- WHEN the chart renders
- THEN the pod mounts both keys at the named paths
- AND the rendered manifests hold no secret value

#### Scenario: Missing Secret
- GIVEN the named Secret does not exist
- WHEN the web pod starts
- THEN the pod does not start

### Requirement: Storage
The chart SHALL create a PersistentVolumeClaim and SHALL mount it at `/data`. The claim SHALL default to the `reliable` class and 1Gi. `data.existingClaim` SHALL reuse a claim instead of creating one. The volume SHALL hold the database only.

The chart SHALL run one web pod with the `Recreate` strategy.

#### Scenario: Default claim
- GIVEN no existing claim
- WHEN the chart installs
- THEN it creates a claim at `/data`

#### Scenario: Existing claim
- GIVEN `data.existingClaim` names a claim
- WHEN the chart installs
- THEN it mounts that claim at `/data`
- AND it creates no claim

#### Scenario: Database survives an upgrade
- GIVEN stored invoices on the claim
- WHEN the chart upgrades
- THEN every stored invoice is still present

### Requirement: Web Workload
The Deployment SHALL run the container with the `web` argument. It SHALL run one pod and SHALL listen on 8080. It SHALL have TCP readiness and liveness probes on that port.

The pod SHALL run as user 1000 with group 1000 and fsGroup 1000. It SHALL use a read-only root filesystem, the default seccomp profile, and no Linux capabilities.

The web container SHALL use `resources` as its CPU and memory requests and limits.

The chart SHALL set a grace period that covers the web app's shutdown timeout, as [Web](web.md#requirement-web-command) describes.

#### Scenario: Web pod
- GIVEN an installed chart
- WHEN the Deployment renders
- THEN the container runs `web`
- AND the pod has one replica
- AND both probes use port 8080

#### Scenario: Restricted pod
- GIVEN the rendered pod
- WHEN its security context is checked
- THEN it runs as user and group 1000
- AND its root filesystem is read-only
- AND it has no Linux capabilities

#### Scenario: Graceful shutdown
- GIVEN a pod that receives a termination request
- WHEN the web app is still finishing a request
- THEN the pod stays alive until the app's shutdown timeout ends

#### Scenario: Resources
- GIVEN a set `resources` request and limit
- WHEN the chart renders
- THEN the web container uses them

### Requirement: Service
The chart SHALL create a ClusterIP Service on `service.port` that forwards to 8080.

#### Scenario: Service port
- GIVEN `service.port` is 80
- WHEN the chart renders
- THEN the Service listens on 80
- AND it forwards to 8080

### Requirement: Ingress and TLS
The chart SHALL enable an Ingress by default with the `traefik` class. The Ingress SHALL route `hostname` to the Service.

The chart SHALL create a cert-manager Certificate for `hostname` with the `lets-encrypt` issuer by default. The Certificate SHALL be named `invoicer` and SHALL store its TLS Secret as `invoicer-tls`. The Ingress SHALL serve that Secret.

Setting `certificate.enabled` to false SHALL omit the Certificate and its TLS configuration.

#### Scenario: Default Ingress
- GIVEN default values
- WHEN the chart renders
- THEN an Ingress routes the hostname to the Service
- AND it uses the `invoicer-tls` Secret

#### Scenario: Default Certificate
- GIVEN default values
- WHEN the chart renders
- THEN a Certificate names the hostname and the `lets-encrypt` issuer
- AND cert-manager stores it as `invoicer-tls`

#### Scenario: TLS ends before the Ingress
- GIVEN `certificate.enabled` is false
- WHEN the chart renders
- THEN it creates no Certificate
- AND the Ingress has no TLS configuration
