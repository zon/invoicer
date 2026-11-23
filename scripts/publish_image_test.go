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
)

// newImagePublishTools returns a directory holding fake skopeo and podman commands
// that record their calls.
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
		case "${FAKE_INSPECT_MODE:-missing}" in
			present)
				printf '%s\n' '{"Digest":"sha256:0000000000000000000000000000000000000000000000000000000000000000"}'
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
func runImagePublisher(t *testing.T, inspectMode string) imagePublisherResult {
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
		"FAKE_INSPECT_MODE="+inspectMode,
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
	result := runImagePublisher(t, "missing")

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
	result := runImagePublisher(t, "present")

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

func TestPublishImageScriptStopsOnFailedCheck(t *testing.T) {
	result := runImagePublisher(t, "denied")

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
