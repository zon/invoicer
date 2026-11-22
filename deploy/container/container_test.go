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
