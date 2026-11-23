package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	imagePublishTestRepo    = "ghcr.io/zon/invoicer"
	imagePublishTestVersion = "0.1.0"
	imagePublishTestCommit  = "abc1234def5678"
	imagePublishTestShort   = "abc1234"
	imagePublishTestLocal   = "ghcr.io/zon/invoicer:ci"
	imagePublishTestDigest  = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	imagePublishOtherDigest = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
)

// imageRegistry describes how the fake registry answers for the commit tag and
// the app version tag. A mode of missing, present, or denied matches a tag.
type imageRegistry struct {
	shaMode       string
	shaDigest     string
	versionMode   string
	versionDigest string
}

// newImagePublishTools returns a directory holding fake skopeo and podman
// commands that record their calls and answer for each tag.
func newImagePublishTools(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	skopeo := `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$SKOPEO_LOG"

command=$1
shift

case "$command" in
	inspect)
		case "$1" in
			*":sha-"*)
				mode=${FAKE_SHA_MODE:-missing}
				digest=${FAKE_SHA_DIGEST:-}
				;;
			*)
				mode=${FAKE_VERSION_MODE:-missing}
				digest=${FAKE_VERSION_DIGEST:-}
				;;
		esac
		case "$mode" in
			present)
				printf '{"Digest":"%s"}\n' "$digest"
				;;
			denied)
				echo "Error: unauthorized: authentication required" >&2
				exit 1
				;;
			*)
				echo "Error: reading manifest: manifest unknown" >&2
				exit 1
				;;
		esac
		;;
	copy)
		;;
esac
`
	podman := `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$PODMAN_LOG"

case "$1" in
	push)
		;;
	*)
		exit 1
		;;
esac
`
	writeFile(t, filepath.Join(dir, "skopeo"), skopeo)
	writeFile(t, filepath.Join(dir, "podman"), podman)
	if err := os.Chmod(filepath.Join(dir, "skopeo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "podman"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

type imagePublisherResult struct {
	output string
	err    error
	skopeo string
	podman string
}

// runImagePublisher runs the image publisher against fake registry tools.
func runImagePublisher(t *testing.T, registry imageRegistry) imagePublisherResult {
	t.Helper()
	toolsDir := newImagePublishTools(t)
	logDir := t.TempDir()
	skopeoLog := filepath.Join(logDir, "skopeo.log")
	podmanLog := filepath.Join(logDir, "podman.log")

	cmd := exec.Command("sh", filepath.Join(repoRoot(t), "scripts", "publish-image.sh"),
		imagePublishTestRepo, imagePublishTestVersion, imagePublishTestCommit, imagePublishTestLocal)
	cmd.Env = append(os.Environ(),
		"PATH="+toolsDir+":"+os.Getenv("PATH"),
		"SKOPEO_LOG="+skopeoLog,
		"PODMAN_LOG="+podmanLog,
		"FAKE_SHA_MODE="+registry.shaMode,
		"FAKE_SHA_DIGEST="+registry.shaDigest,
		"FAKE_VERSION_MODE="+registry.versionMode,
		"FAKE_VERSION_DIGEST="+registry.versionDigest,
	)
	output, err := cmd.CombinedOutput()

	var skopeo, podman string
	if data, readErr := os.ReadFile(skopeoLog); readErr == nil {
		skopeo = string(data)
	}
	if data, readErr := os.ReadFile(podmanLog); readErr == nil {
		podman = string(data)
	}
	return imagePublisherResult{output: string(output), err: err, skopeo: skopeo, podman: podman}
}

// imageToolCall returns the logged call for a command, or an empty string.
func imageToolCall(log, command string) string {
	for _, line := range strings.Split(log, "\n") {
		if strings.HasPrefix(line, command+" ") {
			return line
		}
	}
	return ""
}

func TestPublishImageScriptTagsNewCommit(t *testing.T) {
	result := runImagePublisher(t, imageRegistry{})

	if result.err != nil {
		t.Fatalf("publish image: %v\n%s", result.err, result.output)
	}
	push := imageToolCall(result.podman, "push")
	wantPush := "push " + imagePublishTestLocal + " docker://" + imagePublishTestRepo + ":sha-" + imagePublishTestShort
	if push != wantPush {
		t.Fatalf("image push: got %q, want %q", push, wantPush)
	}
	copy := imageToolCall(result.skopeo, "copy")
	wantCopy := "copy docker://" + imagePublishTestRepo + ":sha-" + imagePublishTestShort + " docker://" + imagePublishTestRepo + ":" + imagePublishTestVersion
	if copy != wantCopy {
		t.Fatalf("version tag copy: got %q, want %q", copy, wantCopy)
	}
}

func TestPublishImageScriptTagsPublishedCommit(t *testing.T) {
	result := runImagePublisher(t, imageRegistry{shaMode: "present", shaDigest: imagePublishTestDigest})

	if result.err != nil {
		t.Fatalf("publish image: %v\n%s", result.err, result.output)
	}
	if push := imageToolCall(result.podman, "push"); push != "" {
		t.Fatalf("image pushed again instead of tagging the published commit: %q", push)
	}
	copy := imageToolCall(result.skopeo, "copy")
	wantCopy := "copy docker://" + imagePublishTestRepo + ":sha-" + imagePublishTestShort + " docker://" + imagePublishTestRepo + ":" + imagePublishTestVersion
	if copy != wantCopy {
		t.Fatalf("version tag copy: got %q, want %q", copy, wantCopy)
	}
}

func TestPublishImageScriptSkipsPublishedVersion(t *testing.T) {
	result := runImagePublisher(t, imageRegistry{
		shaMode:       "present",
		shaDigest:     imagePublishTestDigest,
		versionMode:   "present",
		versionDigest: imagePublishTestDigest,
	})

	if result.err != nil {
		t.Fatalf("publish image: %v\n%s", result.err, result.output)
	}
	if push := imageToolCall(result.podman, "push"); push != "" {
		t.Fatalf("image pushed although the version tag is already published: %q", push)
	}
	if copy := imageToolCall(result.skopeo, "copy"); copy != "" {
		t.Fatalf("version tag copied although it is already published: %q", copy)
	}
}

func TestPublishImageScriptStopsOnDifferentVersionDigest(t *testing.T) {
	result := runImagePublisher(t, imageRegistry{
		shaMode:       "present",
		shaDigest:     imagePublishTestDigest,
		versionMode:   "present",
		versionDigest: imagePublishOtherDigest,
	})

	if result.err == nil {
		t.Fatalf("publish image reused the version tag at a different digest:\n%s", result.output)
	}
	if !strings.Contains(result.output, imagePublishTestVersion) {
		t.Fatalf("publish image does not report the version tag:\n%s", result.output)
	}
	if !strings.Contains(result.output, imagePublishTestDigest) || !strings.Contains(result.output, imagePublishOtherDigest) {
		t.Fatalf("publish image does not report both digests:\n%s", result.output)
	}
	if push := imageToolCall(result.podman, "push"); push != "" {
		t.Fatalf("image pushed although the version tag points to another image: %q", push)
	}
	if copy := imageToolCall(result.skopeo, "copy"); copy != "" {
		t.Fatalf("version tag copied although it points to another image: %q", copy)
	}
}

func TestPublishImageScriptStopsWhenVersionPublishedWithoutCommit(t *testing.T) {
	result := runImagePublisher(t, imageRegistry{
		versionMode:   "present",
		versionDigest: imagePublishTestDigest,
	})

	if result.err == nil {
		t.Fatalf("publish image reused the version tag without a published commit image:\n%s", result.output)
	}
	if push := imageToolCall(result.podman, "push"); push != "" {
		t.Fatalf("image pushed although the version tag is already published: %q", push)
	}
	if copy := imageToolCall(result.skopeo, "copy"); copy != "" {
		t.Fatalf("version tag copied although it is already published: %q", copy)
	}
}

func TestPublishImageScriptStopsOnFailedCheck(t *testing.T) {
	result := runImagePublisher(t, imageRegistry{shaMode: "denied"})

	if result.err == nil {
		t.Fatalf("publish image ignored a failed registry check:\n%s", result.output)
	}
	if push := imageToolCall(result.podman, "push"); push != "" {
		t.Fatalf("image pushed after a failed registry check: %q", push)
	}
	if copy := imageToolCall(result.skopeo, "copy"); copy != "" {
		t.Fatalf("version tag copied after a failed registry check: %q", copy)
	}
}

func TestPublishImageScriptStopsOnFailedVersionCheck(t *testing.T) {
	result := runImagePublisher(t, imageRegistry{
		shaMode:     "present",
		shaDigest:   imagePublishTestDigest,
		versionMode: "denied",
	})

	if result.err == nil {
		t.Fatalf("publish image ignored a failed version check:\n%s", result.output)
	}
	if push := imageToolCall(result.podman, "push"); push != "" {
		t.Fatalf("image pushed after a failed version check: %q", push)
	}
	if copy := imageToolCall(result.skopeo, "copy"); copy != "" {
		t.Fatalf("version tag copied after a failed version check: %q", copy)
	}
}
