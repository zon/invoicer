package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	return filepath.Join(filepath.Dir(file), "..")
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, output)
	}
	return string(output)
}

// newRepo returns a directory holding a git repository with one commit.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "test@example.com")
	run(t, dir, "git", "config", "user.name", "Test")
	writeFile(t, filepath.Join(dir, "file.txt"), "first\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "first")
	return dir
}

// version runs the version script with dir as the working directory.
func version(t *testing.T, dir string) string {
	t.Helper()
	return strings.TrimSpace(run(t, dir, "sh", filepath.Join(repoRoot(t), "scripts", "version.sh")))
}

func TestVersionReportsTagWithoutLeadingV(t *testing.T) {
	repo := newRepo(t)
	run(t, repo, "git", "tag", "v0.2.0")

	if got, want := version(t, repo), "0.2.0"; got != want {
		t.Errorf("version: got %q, want %q", got, want)
	}
}

func TestVersionNamesCommitAfterTag(t *testing.T) {
	repo := newRepo(t)
	run(t, repo, "git", "tag", "v0.2.0")
	writeFile(t, filepath.Join(repo, "file.txt"), "second\n")
	run(t, repo, "git", "commit", "-am", "second")

	got := version(t, repo)
	if !strings.HasPrefix(got, "0.2.0-1-g") {
		t.Errorf("version: got %q, want a version after tag 0.2.0", got)
	}
	commit := strings.TrimSpace(run(t, repo, "git", "rev-parse", "--short", "HEAD"))
	if !strings.HasSuffix(got, commit) {
		t.Errorf("version: got %q, want it to end with commit %s", got, commit)
	}
}

func TestVersionNamesCommitWithoutTags(t *testing.T) {
	repo := newRepo(t)

	got := version(t, repo)
	commit := strings.TrimSpace(run(t, repo, "git", "rev-parse", "--short", "HEAD"))
	if got != commit {
		t.Errorf("version: got %q, want commit %q", got, commit)
	}
}

func TestVersionMarksUncommittedChangesDirty(t *testing.T) {
	repo := newRepo(t)
	run(t, repo, "git", "tag", "v0.2.0")
	writeFile(t, filepath.Join(repo, "file.txt"), "changed\n")

	if got, want := version(t, repo), "0.2.0-dirty"; got != want {
		t.Errorf("version: got %q, want %q", got, want)
	}
}

func TestVersionDefaultsToDevWithoutGitMetadata(t *testing.T) {
	if got, want := version(t, t.TempDir()), "dev"; got != want {
		t.Errorf("version: got %q, want %q", got, want)
	}
}

func TestJustBuildStampsVersionIntoBinary(t *testing.T) {
	justfile := readFile(t, filepath.Join(repoRoot(t), "justfile"))

	if !strings.Contains(justfile, "version := `sh scripts/version.sh`") {
		t.Fatal("justfile does not read the version from scripts/version.sh")
	}
	build := recipe(t, justfile, "build")
	if !strings.Contains(build, "-X github.com/zon/invoicer/internal/cli.Version={{version}}") {
		t.Errorf("build recipe does not stamp the version:\n%s", build)
	}
}

// recipe returns the body of a named justfile recipe.
func recipe(t *testing.T, justfile, name string) string {
	t.Helper()
	var lines []string
	inRecipe := false
	for _, line := range strings.Split(justfile, "\n") {
		if strings.HasPrefix(line, name+":") {
			inRecipe = true
			continue
		}
		if !inRecipe {
			continue
		}
		if line != "" && !strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, " ") {
			break
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		t.Fatalf("justfile has no %s recipe", name)
	}
	return strings.Join(lines, "\n")
}
