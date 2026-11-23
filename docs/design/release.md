# Release Design

A release names one commit. The git tag supplies the app version. The binary, image, and chart derive their versions from it, so the artifacts of a release match.

## App Version

Releases use Semantic Versioning. A release tag is `vMAJOR.MINOR.PATCH` on `main`, such as `v0.2.0`. The app version is the tag without the `v`.

While the major version is 0, a breaking change bumps minor and every other change bumps patch. From 1.0, the usual SemVer rules apply.

The `VERSION` file at the repo root holds the app version, such as `0.2.0`. A version bump sets `VERSION` and the chart versions, as [Chart](#chart) describes. A release fails when `VERSION` and the chart `appVersion` differ, so the chart defaults to the released image.

## Release Command

`just release` starts a release. It fetches `origin/main` and reads `VERSION` from that commit. It tags the commit `v<version>` and pushes the tag. The tag starts the build, as [Build](build.md) describes.

The command reads the remote commit, not the working tree, so the tag matches the commit it names. Local changes and the current branch do not matter.

The command fails before it tags when `VERSION` is missing or not `MAJOR.MINOR.PATCH`, when the chart `appVersion` differs, or when the tag exists on the remote.

## Automatic Release

A push to `main` releases when `VERSION` names an untagged version. The release workflow reads `VERSION` from the pushed commit and skips when the tag exists on the remote. Otherwise it tags the commit and calls the build, so a version bump releases without a manual step.

## Binary

The build stamps the app version into the binary. `invoicer --version` prints it.

A tagged build reports the app version, such as `0.2.0`. Any other build reports a version that names the commit, such as `0.2.0-3-gabc1234`. A build with uncommitted changes marks the version dirty. A build without git metadata reports `dev`.

## Container

Every published image receives a `sha-<short commit>` tag. A release image also receives the app version tag. Both tags point at the same digest. A pushed tag is never reused for another image.

A published image carries OCI labels for version, revision, source, and creation time.

There is no `latest` tag. A deployment names an exact tag or digest, so the running image is identifiable.

## Chart

The chart has its own SemVer `version`. Helm requires a new chart version for every upgrade, so the version bumps whenever the chart changes. It usually matches the app version at an app release and may be ahead after a chart-only fix.

An app version bump sets `appVersion` to the new app version. It raises `version` to the new app version when that is higher, and otherwise bumps the chart patch.

The chart `appVersion` is the app version of the image the chart defaults to. An empty `image.tag` uses it, as [Chart](chart.md) describes. A chart-only fix keeps `appVersion` and defaults to the last released image.

A release publishes the chart package from the tagged commit. A chart-only fix ships in the next release, because only a tag builds.

## Artifacts

One release tag builds the binary, image, and chart from the tagged commit. The binary version, image version tag, and chart `appVersion` are the same app version. The chart `version` identifies the chart package.
