package container_test

import (
	"os"
	"path/filepath"
	"regexp"
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
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func goVersion(t *testing.T, root string) string {
	t.Helper()
	match := regexp.MustCompile(`(?m)^go (\d+\.\d+)`).FindStringSubmatch(readFile(t, filepath.Join(root, "go.mod")))
	if match == nil {
		t.Fatal("go.mod has no go version")
	}
	return match[1]
}

func TestContainerfileRunsInvoicerWithArguments(t *testing.T) {
	root := repoRoot(t)
	containerfile := readFile(t, filepath.Join(root, "deploy", "container", "Containerfile"))

	entrypoint := regexp.MustCompile(`(?m)^ENTRYPOINT \["(?:/[^"]*/)?invoicer"\]$`).FindString(containerfile)
	if entrypoint == "" {
		t.Fatal("entrypoint does not run invoicer in exec form")
	}
	if regexp.MustCompile(`(?m)^CMD\b`).MatchString(containerfile) {
		t.Fatal("image sets default arguments instead of passing them through")
	}
}

func runtimeStage(t *testing.T, containerfile string) string {
	t.Helper()
	stages := strings.Split(containerfile, "\nFROM ")
	if len(stages) < 2 {
		t.Fatal("Containerfile has no runtime stage")
	}
	return stages[len(stages)-1]
}

func TestContainerfileRunsAsUser1000(t *testing.T) {
	root := repoRoot(t)
	runtime := runtimeStage(t, readFile(t, filepath.Join(root, "deploy", "container", "Containerfile")))

	if !regexp.MustCompile(`(?m)^USER 1000:1000$`).MatchString(runtime) {
		t.Fatal("image does not run as user and group 1000")
	}
	if !regexp.MustCompile(`(?m)^ENV HOME=/home/invoicer$`).MatchString(runtime) {
		t.Fatal("image does not set HOME to /home/invoicer")
	}
	if !strings.Contains(runtime, "mkdir -p /home/invoicer/.invoicer") {
		t.Fatal("image does not create /home/invoicer/.invoicer")
	}
	if !strings.Contains(runtime, "chown -R 1000:1000 /home/invoicer") {
		t.Fatal("image does not give user 1000 ownership of /home/invoicer")
	}
}

func TestContainerfileSetsNoDatabasePath(t *testing.T) {
	root := repoRoot(t)
	runtime := runtimeStage(t, readFile(t, filepath.Join(root, "deploy", "container", "Containerfile")))

	if regexp.MustCompile(`(?mi)^ENV\b.*(?:database|db[_-]?path)`).MatchString(runtime) {
		t.Fatal("image sets a database path")
	}
}

func TestContainerfileExposesPort8080(t *testing.T) {
	root := repoRoot(t)
	runtime := runtimeStage(t, readFile(t, filepath.Join(root, "deploy", "container", "Containerfile")))

	if !regexp.MustCompile(`(?m)^EXPOSE 8080$`).MatchString(runtime) {
		t.Fatal("image does not expose port 8080")
	}
}

func TestContainerfileHoldsNoConfigDatabaseOrSecret(t *testing.T) {
	root := repoRoot(t)
	runtime := runtimeStage(t, readFile(t, filepath.Join(root, "deploy", "container", "Containerfile")))

	copies := regexp.MustCompile(`(?m)^(?:COPY|ADD)\b.*$`).FindAllString(runtime, -1)
	if len(copies) != 1 {
		t.Fatalf("runtime stage copies %d things, want only the binary", len(copies))
	}
	if !regexp.MustCompile(`^COPY --from=\S+ /[^ ]*/invoicer /usr/local/bin/invoicer$`).MatchString(copies[0]) {
		t.Fatalf("runtime stage copies %q instead of only the binary", copies[0])
	}
	for _, line := range strings.Split(runtime, "\n") {
		lower := strings.ToLower(line)
		for _, held := range []string{"config.yaml", "database", "secret", ".db"} {
			if strings.Contains(lower, held) {
				t.Fatalf("runtime stage holds %s: %s", held, line)
			}
		}
	}
}

func TestContainerfileBuildsFromRepositoryRoot(t *testing.T) {
	root := repoRoot(t)
	containerfile := readFile(t, filepath.Join(root, "deploy", "container", "Containerfile"))

	build := regexp.MustCompile(`(?m)^FROM golang:(\d+\.\d+)(?:\.\d+)?-alpine`).FindStringSubmatch(containerfile)
	if build == nil {
		t.Fatal("build stage does not use a Go Alpine image")
	}
	if build[1] != goVersion(t, root) {
		t.Fatalf("build stage uses Go %s, go.mod uses %s", build[1], goVersion(t, root))
	}
	if !strings.Contains(containerfile, "CGO_ENABLED=1") {
		t.Fatal("build does not enable CGO for SQLite")
	}
	if !strings.Contains(containerfile, "./cmd/invoicer") {
		t.Fatal("build does not compile the invoicer command")
	}
	if !regexp.MustCompile(`(?m)^FROM alpine:`).MatchString(containerfile) {
		t.Fatal("runtime stage does not use Alpine")
	}
	if !strings.Contains(containerfile, "ca-certificates") {
		t.Fatal("runtime stage does not install CA certificates")
	}
	if !regexp.MustCompile(`(?m)^COPY --from=\S+ /[^ ]*/invoicer /usr/local/bin/invoicer$`).MatchString(containerfile) {
		t.Fatal("runtime stage does not install the binary")
	}
}
