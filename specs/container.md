# Container Specification

## Purpose
One image runs the invoice CLI and the web app from one binary. A deployment mounts the config file, the database volume, and secrets.

## Requirements

### Requirement: One Image, Two Modes
The image SHALL use `invoicer` as its entrypoint and SHALL pass its arguments through unchanged. It SHALL add no switches or defaults. The argument `web` SHALL run the web app, as [Web](web.md) describes. An image run with no subcommand SHALL generate an invoice, as [Invoices](invoices.md) describes.

#### Scenario: Web mode
- GIVEN the image and a mounted database volume
- WHEN the container starts with `web`
- THEN it runs `invoicer web`
- AND it serves the web app on port 8080

#### Scenario: Invoice mode
- GIVEN the image and the required invoice flags
- WHEN the container starts with no subcommand
- THEN it generates the invoice
- AND the container exits with the command's status

#### Scenario: Arguments pass through
- GIVEN an argument after the image name
- WHEN the container starts
- THEN `invoicer` receives that argument unchanged

### Requirement: Build
The image SHALL build from the repository root with Podman from `deploy/container/Containerfile`. The build SHALL compile the binary from the Go version in `go.mod`.

The build SHALL accept the app version and stamp it into the binary, so `invoicer --version` reports it. A build without an app version SHALL still produce a working image.

#### Scenario: Build from the root
- GIVEN a repository checkout
- WHEN Podman builds from `deploy/container/Containerfile`
- THEN the build completes
- AND the image runs `invoicer`

#### Scenario: Stamped version
- GIVEN an app version
- WHEN Podman builds the image with that version
- THEN `invoicer --version` inside the image prints it

#### Scenario: Build without a version
- GIVEN no app version
- WHEN Podman builds the image
- THEN the build completes
- AND `invoicer --version` inside the image prints a version

### Requirement: Runtime Content
The image SHALL hold the binary, CA certificates, and the base system. It SHALL NOT hold a config file, a database, or a secret. It SHALL NOT need a Go toolchain, a C compiler, or build files at run time.

#### Scenario: SQLite at run time
- GIVEN the image with a mounted database volume
- WHEN the web app stores or reads an invoice
- THEN SQLite works without a build toolchain in the image

#### Scenario: No bundled state
- GIVEN the image and no mounts
- WHEN the CLI runs with the required flags
- THEN it creates the database under `HOME`
- AND no config file, database, or secret came from the image

### Requirement: User and Paths
The image SHALL run as user and group 1000. It SHALL set `HOME` to `/home/invoicer` and SHALL create `/home/invoicer/.invoicer`, so the config path is `/home/invoicer/.invoicer/config.yaml`. It SHALL set no database path. It SHALL expose port 8080.

#### Scenario: Default config path
- GIVEN no mounted config
- WHEN the web app starts
- THEN it reads config from `/home/invoicer/.invoicer/config.yaml`
- AND it starts without sign-in because the file is empty

#### Scenario: Non-root user
- GIVEN a running container
- WHEN the process inspects its user
- THEN it runs as user and group 1000
- AND it is not root

#### Scenario: Mounted config
- GIVEN a config mounted at `/home/invoicer/.invoicer/config.yaml`
- WHEN the web app starts
- THEN it uses the mounted settings

#### Scenario: Default database
- GIVEN no database is configured
- WHEN the CLI stores an invoice
- THEN the database is `/home/invoicer/.invoicer/invoices.db`

### Requirement: Mounted Data
The image SHALL read a mounted config from the config path. It SHALL read and write a database at the configured path. A mounted database SHALL keep its invoices when the container is replaced. The image SHALL work with a read-only root filesystem when config, database, and secrets are mounted.

#### Scenario: Data survives replacement
- GIVEN a database on a mounted volume
- WHEN the container is replaced
- THEN every stored invoice is still present

#### Scenario: Config and database volumes
- GIVEN a mounted config and a mounted database
- WHEN the web app starts
- THEN it reads the config and opens the database
- AND the database is the mounted file

#### Scenario: Read-only root filesystem
- GIVEN a read-only root filesystem with mounted config, database, and secrets
- WHEN the web app serves pages and stores an invoice
- THEN both operations succeed
