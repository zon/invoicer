# Release Specification

## Purpose
Releases give the binary, container image, and chart one traceable app version from a git tag.

## Requirements

### Requirement: Release Tag
A release SHALL use a git tag `vMAJOR.MINOR.PATCH` on `main`. The app version SHALL be the tag without the leading `v`.

While the major version is 0, a breaking change SHALL bump minor and any other change SHALL bump patch. From 1.0, SemVer rules SHALL apply.

The release SHALL fail when the chart `appVersion` differs from the app version.

#### Scenario: Tagged release
- GIVEN `main` at a release commit
- WHEN the commit is tagged `v0.2.0`
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
- GIVEN a tag `v0.2.0` and a chart `appVersion` of `0.1.0`
- WHEN the release runs
- THEN the release fails

### Requirement: Release Command
`just release` SHALL tag the head commit of `origin/main` with `v` followed by the contents of the `VERSION` file at that commit, and push the tag. It SHALL NOT read the working tree.

The command SHALL fail without tagging when `VERSION` is missing or is not `MAJOR.MINOR.PATCH`, when the chart `appVersion` differs from `VERSION`, or when the tag exists on the remote.

#### Scenario: Release from remote head
- GIVEN `VERSION` holds `0.2.0` at the head of `origin/main`
- WHEN `just release` runs
- THEN the head commit of `origin/main` is tagged `v0.2.0`
- AND the tag is pushed to `origin`

#### Scenario: Local changes ignored
- GIVEN a local `VERSION` that differs from the one at the head of `origin/main`
- WHEN `just release` runs
- THEN the tag uses the version at the head of `origin/main`

#### Scenario: Missing version file
- GIVEN no `VERSION` file at the head of `origin/main`
- WHEN `just release` runs
- THEN it fails
- AND no tag is created

#### Scenario: Invalid version
- GIVEN `VERSION` holds `0.2`
- WHEN `just release` runs
- THEN it fails
- AND no tag is created

#### Scenario: Mismatched chart app version
- GIVEN `VERSION` holds `0.2.0` and the chart `appVersion` is `0.1.0`
- WHEN `just release` runs
- THEN it fails
- AND no tag is created

#### Scenario: Reused tag
- GIVEN the tag `v0.2.0` exists on the remote
- WHEN `just release` runs with `VERSION` at `0.2.0`
- THEN it fails

### Requirement: Automatic Release
A push to `main` SHALL release the pushed commit when the tag `vVERSION` does not exist on the remote, and SHALL do nothing when it exists.

The release SHALL read `VERSION` from the pushed commit and SHALL use the [release command](#requirement-release-command).

#### Scenario: Unreleased version on main
- GIVEN `VERSION` holds `0.2.0` at the pushed `main` commit
- AND the tag `v0.2.0` does not exist on the remote
- WHEN the push lands
- THEN the commit is tagged `v0.2.0`
- AND the build runs on the tag

#### Scenario: Released version on main
- GIVEN `VERSION` holds `0.2.0` at the pushed `main` commit
- AND the tag `v0.2.0` exists on the remote
- WHEN the push lands
- THEN no tag is created
- AND no build runs

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
- GIVEN a build from tag `v0.2.0`
- WHEN the image is pushed
- THEN it has the tags `0.2.0` and `sha-abc1234`
- AND both tags resolve to the same digest

#### Scenario: Commit build
- GIVEN a build from an untagged commit
- WHEN the image is pushed
- THEN it has the `sha-abc1234` tag
- AND it has no app version tag

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
One release tag SHALL build the binary, image, and chart from the tagged commit. The binary version, image version tag, and chart `appVersion` SHALL all equal the app version.

#### Scenario: Artifacts agree
- GIVEN tag `v0.2.0`
- WHEN the release builds the binary, image, and chart
- THEN the binary reports `0.2.0`
- AND the image has the `0.2.0` tag
- AND the chart `appVersion` is `0.2.0`
- AND every artifact comes from the tagged commit
