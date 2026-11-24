# Release Specification

## Purpose
Releases give the binary, container image, chart, and git tag one traceable app version.

## Requirements

### Requirement: App Version
A release SHALL use the app version in the `VERSION` file at the repo root. The app version SHALL be `MAJOR.MINOR.PATCH`.

While the major version is 0, a breaking change SHALL bump minor and any other change SHALL bump patch. From 1.0, SemVer rules SHALL apply.

The chart build SHALL fail when the chart `appVersion` differs from the app version.

#### Scenario: Version file
- GIVEN `VERSION` holds `0.2.0`
- WHEN a release builds
- THEN the app version is `0.2.0`

#### Scenario: Breaking change before 1.0
- GIVEN a current release of `0.2.0`
- WHEN a change breaks the config format
- THEN the next release is `0.3.0`

#### Scenario: Other change before 1.0
- GIVEN a current release of `0.2.0`
- WHEN a change does not break existing behavior
- THEN the next release is `0.2.1`

#### Scenario: Mismatched chart app version
- GIVEN `VERSION` holds `0.2.0` and the chart `appVersion` is `0.1.0`
- WHEN the chart build runs
- THEN the build fails

### Requirement: Release Tag
The container build SHALL tag the built commit `v<app version>` and push the tag after it publishes the image. The tag SHALL NOT be reused.

The container build SHALL NOT create a tag that exists on the remote.

#### Scenario: Tagged build
- GIVEN `VERSION` holds `0.2.0` at a main commit
- AND the image `0.2.0` does not exist
- WHEN the container build runs
- THEN it publishes the image
- AND it tags the commit `v0.2.0`
- AND it pushes the tag

#### Scenario: Existing tag
- GIVEN the tag `v0.2.0` exists on the remote
- WHEN the container build publishes an image for `0.2.0`
- THEN it leaves the tag unchanged
- AND the build succeeds

#### Scenario: Built image
- GIVEN the image `0.2.0` exists
- WHEN a main push runs the container build
- THEN it builds nothing
- AND it creates no tag

### Requirement: Automatic Release
A push to `main` SHALL start the container build and the chart build.

Each build SHALL skip when its registry already holds its version. The container build SHALL compare its app version with the image tags. The chart build SHALL compare the chart `version` with the published chart versions.

#### Scenario: New app version on main
- GIVEN `VERSION` holds `0.2.0`
- AND no image has the `0.2.0` tag
- WHEN the push lands
- THEN the container build publishes the image
- AND the container build tags the commit `v0.2.0`

#### Scenario: Published app version on main
- GIVEN `VERSION` holds `0.2.0`
- AND an image has the `0.2.0` tag
- WHEN the push lands
- THEN the container build does nothing

#### Scenario: New chart version on main
- GIVEN the chart `version` is `0.1.2`
- AND no chart package has version `0.1.2`
- WHEN a chart-only push lands
- THEN the chart build publishes version `0.1.2`
- AND the container build does nothing when the app version image exists

#### Scenario: Published chart version on main
- GIVEN the chart `version` is `0.1.2`
- AND a chart package has version `0.1.2`
- WHEN the push lands
- THEN the chart build does nothing

### Requirement: Binary Version
The binary SHALL report its app version from `invoicer --version` and exit successfully.

A build from the release tag SHALL report the app version. A build from another commit SHALL report a version that names the commit. A build with uncommitted changes SHALL mark the version dirty.

#### Scenario: Tagged build
- GIVEN a build from tag `v0.2.0`
- WHEN `invoicer --version` runs
- THEN it prints `0.2.0`
- AND it exits successfully

#### Scenario: Commit build
- GIVEN a build from a commit after `v0.2.0`
- WHEN `invoicer --version` runs
- THEN it prints a version such as `0.2.0-3-gabc1234`
- AND the version names the commit

#### Scenario: Dirty build
- GIVEN a build with uncommitted changes
- WHEN `invoicer --version` runs
- THEN the version is marked dirty

#### Scenario: Version runs no command
- GIVEN any build
- WHEN `invoicer --version` runs
- THEN it prints the version and exits
- AND it runs no generate, web, or set command

### Requirement: Container Tags
Every published image SHALL carry a `sha-<short commit>` tag. A release image SHALL also carry the app version tag. Both tags SHALL resolve to the same digest.

The image SHALL NOT carry a `latest` tag. A pushed tag SHALL NOT be reused for another image.

A published image SHALL carry OCI labels for version, revision, source, and creation time.

#### Scenario: Release build
- GIVEN a build for app version `0.2.0`
- WHEN the image is pushed
- THEN it has the tags `0.2.0` and `sha-abc1234`
- AND both tags resolve to the same digest

#### Scenario: No latest tag
- GIVEN any image build
- WHEN the image tags are listed
- THEN no `latest` tag exists

#### Scenario: Image labels
- GIVEN a pushed image
- WHEN its OCI labels are read
- THEN the version, revision, source, and creation time are present

### Requirement: Chart Version
The chart SHALL have its own SemVer `version`. The version SHALL bump when the chart changes.

The chart `appVersion` SHALL be the app version of the image the chart defaults to. An empty `image.tag` SHALL use `appVersion`, as [Chart](chart.md#requirement-values) describes.

An app version bump SHALL set `appVersion` to the new app version and SHALL bump `version`. It SHALL use the new app version for `version` when that is higher, and otherwise bump the chart patch.

A chart-only change SHALL bump `version` and keep `appVersion`.

#### Scenario: App version bump
- GIVEN a chart `version` of 0.1.0 and `appVersion` of 0.1.0
- WHEN the app version bumps to 0.1.1
- THEN the chart `appVersion` is 0.1.1
- AND the chart `version` is 0.1.1

#### Scenario: App version bump after a chart-only fix
- GIVEN a chart `version` of 0.1.2 and `appVersion` of 0.1.0
- WHEN the app version bumps to 0.1.1
- THEN the chart `appVersion` is 0.1.1
- AND the chart `version` is 0.1.3

#### Scenario: App release
- GIVEN a release of app version `0.2.0`
- WHEN the chart is packaged
- THEN the chart `appVersion` is `0.2.0`
- AND an empty `image.tag` uses `0.2.0`

#### Scenario: Chart-only fix
- GIVEN a chart with `version` 0.1.1 and `appVersion` 0.1.0
- WHEN a chart fix ships as version 0.1.2
- THEN `appVersion` stays `0.1.0`
- AND the chart still defaults to the `0.1.0` image

#### Scenario: Chart version bump
- GIVEN a packaged chart version
- WHEN the chart changes
- THEN the next package uses a higher `version`

### Requirement: Release Artifacts
One main commit with `VERSION` SHALL build the binary, image, and chart. The binary version, image version tag, and chart `appVersion` SHALL all equal the app version. The container build SHALL tag the commit `v<app version>`.

#### Scenario: Artifacts agree
- GIVEN `VERSION` holds `0.2.0` at a main commit
- WHEN the push builds the binary, image, and chart
- THEN the binary reports `0.2.0`
- AND the image has the `0.2.0` tag
- AND the chart `appVersion` is `0.2.0`
- AND the commit is tagged `v0.2.0`
- AND every artifact comes from the main commit
