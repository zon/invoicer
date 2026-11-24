# Release Design

A release names one commit on `main`. The `VERSION` file supplies the app version. The container build tags the commit `v<app version>` after it publishes the image. The image and chart take their versions from `VERSION`, so the artifacts of a release match.

## App Version

Releases use Semantic Versioning. The app version is `MAJOR.MINOR.PATCH`, such as `0.2.0`.

While the major version is 0, a breaking change bumps minor and every other change bumps patch. From 1.0, the usual SemVer rules apply.

The `VERSION` file at the repo root holds the app version. A version bump sets `VERSION` and the chart versions, as [Chart](#chart) describes. The chart build fails when `VERSION` and the chart `appVersion` differ, so the chart defaults to the release image.

## Release Tag

The container build tags the commit it builds `v<app version>` and pushes the tag after it publishes the image. The tag names a commit whose image exists. A pushed tag is never reused.

A main push whose app version image already exists creates no tag. The first successful container build of that version did.

## Automatic Release

A push to `main` starts the container and chart builds, as [Build](build.md) describes. Each build skips when its registry already holds its version, so a version bump releases without a manual step.

## Binary

CI stamps the app version into the binary, so `invoicer --version` prints it.

A local build from the release tag prints the app version, such as `0.2.0`. A local build from another commit prints a version that names the commit, such as `0.2.0-3-gabc1234`. A build with uncommitted changes marks the version dirty. A build without git metadata prints `dev`.

## Container

Every published image receives a `sha-<short commit>` tag. A release image also receives the app version tag. Both tags point at the same digest. A pushed tag is never reused for another image.

A published image carries OCI labels for version, revision, source, and creation time.

There is no `latest` tag. A deployment names an exact tag or digest, so the running image is identifiable.

## Chart

The chart has its own SemVer `version`. Helm requires a new chart version for every upgrade, so the version bumps whenever the chart changes. It usually matches the app version at an app release and may be ahead after a chart-only fix.

An app version bump sets `appVersion` to the new app version. It raises `version` to the new app version when that is higher, and otherwise bumps the chart patch.

The chart `appVersion` is the app version of the image the chart defaults to. An empty `image.tag` uses it, as [Chart](chart.md) describes. A chart-only fix keeps `appVersion` and defaults to the last released image.

A chart build publishes the chart package from the main commit. A chart-only fix publishes the new chart version on its own, when the container build finds its image already published.

## Artifacts

One main commit with `VERSION` builds the binary, image, and chart. The binary version, image version tag, and chart `appVersion` are the same app version. The chart `version` identifies the chart package. The container build tags the commit `v<app version>`.
