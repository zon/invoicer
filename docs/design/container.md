# Container Design

One container image runs the CLI and the web app. The image holds one binary, `invoicer`, and starts it as the entrypoint. The arguments choose the mode. `invoicer web` serves the web app. `invoicer` with no subcommand generates an invoice. The image behaves like the binary and adds no switches of its own.

The Containerfile at `deploy/container/Containerfile` builds the image.

## Build

A build stage compiles the binary. A runtime stage holds the binary, CA certificates, and the base system.

The store uses the CGO SQLite driver, so the build enables CGO and links against musl. The build stage pins the Go version from `go.mod` and uses the latest Alpine release. The runtime stage uses the same Alpine release, as the [container standard](../zpecs/containers.md) describes.

The image runs as a non-root user with the fixed ID 1000. It sets `HOME` to `/home/invoicer` and creates `/home/invoicer/.invoicer`, so the config file resolves to `/home/invoicer/.invoicer/config.yaml`. The image sets no database path. A deployment mounts a volume and names the path in config.

The image exposes port 8080, the default web address.

## Paths

* binary — `/usr/local/bin/invoicer`
* config — `/home/invoicer/.invoicer/config.yaml`
* database — a mounted volume, named by `database` in the config
* secrets — mounted files, named by the `oidc` settings in the config

The image contains no config, database, or secret. A run without mounts uses the built-in defaults and creates the database under `HOME`.

## Local Use

Podman builds the image from the repository root. A local run mounts a config file and a data directory, as the [chart](chart.md) does. `podman run` with no mounts generates an invoice from flags.

## Out of Scope

* Publishing image tags
* Vulnerability scanning
* Deployment manifests, which belong to the [chart](chart.md)
* The monthly schedule
