# Build Design

CI builds and publishes the container image and the Helm chart in separate workflows. Both run on GitHub Actions after a push to `main`. Each workflow checks its registry for the version it would build and skips when that version exists.

## Trigger

A push to `main` starts both workflows. A pull request runs no workflow. CI builds only on a push to `main`, so the two workflows publish from the same commit.

## Version

Each workflow reads the app version from `VERSION`. The chart workflow reads its own `version` from `Chart.yaml`.

The container build stamps the app version into the binary, so `invoicer --version` reports it. It creates the git tag `v<app version>` after it publishes the image. The tag names a commit whose image exists.

The container build reads the short commit for the image tags.

## Checks

Each build runs the test suite with `go test ./...`. A failed test stops that build.

The container build scans the built image with Trivy. A HIGH or CRITICAL vulnerability stops the build. A failed image is not pushed. The build pins the Trivy version.

## Container

The check job reads `VERSION` and looks for that image tag in the registry. The build runs only when the registry does not have it.

The container build uses Podman with `deploy/container/Containerfile`, as [Container](container.md) describes. It keeps the image local until the scan passes, so the scan and the push use the same image.

The image goes to `ghcr.io/zon/invoicer`. Every build tags it `sha-<short commit>` and the app version, such as `0.2.0`. Both tags point at one digest. A build makes no `latest` tag.

The image carries the OCI labels that [Release](release.md) describes.

After the push, the build tags the commit `v<app version>` and pushes the tag. It skips a tag that already exists.

## Chart

The check job reads the chart `version` from `Chart.yaml` and looks for it in the chart registry. The build runs only when the registry does not have it.

The chart build lints the chart at `deploy/chart` and packages it at the `version` in `Chart.yaml`. It fails when `appVersion` differs from the app version, as [Release](release.md) describes.

The package goes to `oci://ghcr.io/zon/charts` as `invoicer`. A published chart version is never replaced. The build compares the local chart with an already published package of the same version. A match skips the push. A difference fails the build, so a chart change needs a `version` bump.

## Credentials

CI uses the workflow `GITHUB_TOKEN` with package write permission. The container workflow also needs contents write permission to push the release tag. The build needs no other secret.
