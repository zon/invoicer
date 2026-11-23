# Build Design

CI builds and publishes the container image and the Helm chart. It runs on GitHub Actions. A build publishes nothing until every check passes.

## Trigger

A version tag builds and publishes the release tags, as [Release](release.md) describes. A push to `main` starts the [release](release.md) workflow, which tags an unreleased version and then calls the build. A pull request runs no workflow.

## Version

The build reads the app version from the release tag, or takes it from the release workflow, as [Release](release.md) describes. It stamps the version into the binary, so `invoicer --version` reports it. The image and the chart carry the same version.

The build reads the short commit for the image tags.

## Checks

The build runs the test suite with `go test ./...`. A failed test stops the build.

Trivy scans the built image. A HIGH or CRITICAL vulnerability stops the build. A failed image is not pushed. The build pins the Trivy version.

## Image

The build uses Podman with `deploy/container/Containerfile`, as [Container](container.md) describes. It keeps the image local until the scan passes, so the scan and the push use the same image.

The image goes to `ghcr.io/zon/invoicer`. Every release tags it `sha-<short commit>` and the app version, such as `0.2.0`. Both tags point at one digest. A build makes no `latest` tag.

The image carries the OCI labels that [Release](release.md) describes.

## Chart

The build lints the chart at `deploy/chart` and packages it at the `version` in `Chart.yaml`. It fails when `appVersion` differs from the app version, as [Release](release.md) describes.

The package goes to `oci://ghcr.io/zon/charts` as `invoicer`. A published chart version is never replaced. The build compares the local chart with an already published package of the same version. A match skips the push. A difference fails the build, so a chart change needs a `version` bump.

## Credentials

CI uses the workflow `GITHUB_TOKEN` with package write permission. The build needs no other secret.

A tag pushed with `GITHUB_TOKEN` starts no workflow, so the release workflow calls the build directly. A tag pushed by hand starts the build through the tag trigger.

## Out of Scope

* Deploying the chart, which belongs to the lab repository
* Release numbering, which belongs to [Release](release.md)
* A standalone binary download
* Image signing and provenance
* The monthly schedule
