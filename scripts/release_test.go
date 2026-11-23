package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// releaseFixture is a clone whose origin/main holds the given VERSION and chart appVersion.
// An empty version leaves out the VERSION file.
func releaseFixture(t *testing.T, version, appVersion string) (clone, origin string) {
	t.Helper()
	root := t.TempDir()
	origin = filepath.Join(root, "origin.git")
	run(t, root, "git", "init", "--bare", "--initial-branch=main", origin)

	clone = filepath.Join(root, "clone")
	run(t, root, "git", "clone", origin, clone)
	run(t, clone, "git", "config", "user.email", "test@example.com")
	run(t, clone, "git", "config", "user.name", "Test")
	run(t, clone, "git", "checkout", "-b", "main")

	chart := filepath.Join(clone, "deploy", "chart")
	if err := os.MkdirAll(chart, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(chart, "Chart.yaml"),
		"apiVersion: v2\nname: invoicer\nversion: 0.1.0\nappVersion: \""+appVersion+"\"\n")
	if version != "" {
		writeFile(t, filepath.Join(clone, "VERSION"), version+"\n")
	}
	run(t, clone, "git", "add", ".")
	run(t, clone, "git", "commit", "-m", "release commit")
	run(t, clone, "git", "push", "origin", "main")
	return clone, origin
}

// runRelease runs the release script in the clone.
func runRelease(t *testing.T, clone string) (string, error) {
	t.Helper()
	cmd := exec.Command("sh", filepath.Join(repoRoot(t), "scripts", "release.sh"))
	cmd.Dir = clone
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// remoteTags lists the tags in the origin repository.
func remoteTags(t *testing.T, origin string) string {
	t.Helper()
	return strings.TrimSpace(run(t, origin, "git", "tag"))
}

func TestReleaseTagsRemoteHeadAndPushesTag(t *testing.T) {
	clone, origin := releaseFixture(t, "0.2.0", "0.2.0")

	output, err := runRelease(t, clone)

	if err != nil {
		t.Fatalf("release: %v\n%s", err, output)
	}
	if got, want := remoteTags(t, origin), "v0.2.0"; got != want {
		t.Fatalf("remote tags: got %q, want %q", got, want)
	}
	head := strings.TrimSpace(run(t, origin, "git", "rev-parse", "main"))
	tagged := strings.TrimSpace(run(t, origin, "git", "rev-parse", "v0.2.0^{commit}"))
	if tagged != head {
		t.Errorf("tag points at %s, want remote head %s", tagged, head)
	}
}

func TestReleaseIgnoresLocalChanges(t *testing.T) {
	clone, origin := releaseFixture(t, "0.2.0", "0.2.0")
	writeFile(t, filepath.Join(clone, "VERSION"), "9.9.9\n")
	run(t, clone, "git", "commit", "-am", "unpushed bump")

	output, err := runRelease(t, clone)

	if err != nil {
		t.Fatalf("release: %v\n%s", err, output)
	}
	if got, want := remoteTags(t, origin), "v0.2.0"; got != want {
		t.Fatalf("remote tags: got %q, want %q", got, want)
	}
}

func TestReleaseFailsWithoutVersionFile(t *testing.T) {
	clone, origin := releaseFixture(t, "", "0.2.0")

	output, err := runRelease(t, clone)

	if err == nil {
		t.Fatalf("release accepted a missing VERSION file:\n%s", output)
	}
	if got := remoteTags(t, origin); got != "" {
		t.Errorf("remote tags: got %q, want none", got)
	}
}

func TestReleaseFailsOnInvalidVersion(t *testing.T) {
	clone, origin := releaseFixture(t, "0.2", "0.2")

	output, err := runRelease(t, clone)

	if err == nil {
		t.Fatalf("release accepted version 0.2:\n%s", output)
	}
	if got := remoteTags(t, origin); got != "" {
		t.Errorf("remote tags: got %q, want none", got)
	}
}

func TestReleaseFailsOnChartAppVersionMismatch(t *testing.T) {
	clone, origin := releaseFixture(t, "0.2.0", "0.1.0")

	output, err := runRelease(t, clone)

	if err == nil {
		t.Fatalf("release accepted appVersion 0.1.0 for version 0.2.0:\n%s", output)
	}
	if !strings.Contains(output, "0.1.0") || !strings.Contains(output, "0.2.0") {
		t.Errorf("release does not report both versions:\n%s", output)
	}
	if got := remoteTags(t, origin); got != "" {
		t.Errorf("remote tags: got %q, want none", got)
	}
}

func TestReleaseFailsWhenTagExistsOnRemote(t *testing.T) {
	clone, origin := releaseFixture(t, "0.2.0", "0.2.0")
	run(t, clone, "git", "tag", "v0.2.0")
	run(t, clone, "git", "push", "origin", "v0.2.0")
	run(t, clone, "git", "tag", "-d", "v0.2.0")

	output, err := runRelease(t, clone)

	if err == nil {
		t.Fatalf("release accepted a tag that exists on the remote:\n%s", output)
	}
	if !strings.Contains(output, "v0.2.0") {
		t.Errorf("release does not name the tag:\n%s", output)
	}
	if got, want := remoteTags(t, origin), "v0.2.0"; got != want {
		t.Errorf("remote tags: got %q, want %q", got, want)
	}
}

func TestJustReleaseRunsReleaseScript(t *testing.T) {
	justfile := readFile(t, filepath.Join(repoRoot(t), "justfile"))

	if release := recipe(t, justfile, "release"); !strings.Contains(release, "scripts/release.sh") {
		t.Errorf("release recipe does not run scripts/release.sh:\n%s", release)
	}
}
