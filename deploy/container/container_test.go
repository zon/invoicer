package container_test

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
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

func buildStage(t *testing.T, containerfile string) string {
	t.Helper()
	stages := strings.Split(containerfile, "\nFROM ")
	if len(stages) < 2 {
		t.Fatal("Containerfile has no build stage")
	}
	return stages[0]
}

func runtimeStage(t *testing.T, containerfile string) string {
	t.Helper()
	stages := strings.Split(containerfile, "\nFROM ")
	if len(stages) < 2 {
		t.Fatal("Containerfile has no runtime stage")
	}
	return stages[len(stages)-1]
}

// apkPackages returns the packages a stage installs with apk.
func apkPackages(stage string) []string {
	var packages []string
	for _, line := range strings.Split(stage, "\n") {
		if !strings.HasPrefix(line, "RUN apk add") {
			continue
		}
		for _, field := range strings.Fields(line) {
			switch {
			case field == "RUN", field == "apk", field == "add", field == "&&", field == "\\":
				continue
			case strings.HasPrefix(field, "-"):
				continue
			}
			packages = append(packages, field)
		}
	}
	return packages
}

func TestContainerfileBuildsSQLiteWithoutRuntimeToolchain(t *testing.T) {
	root := repoRoot(t)
	containerfile := readFile(t, filepath.Join(root, "deploy", "container", "Containerfile"))
	build := buildStage(t, containerfile)
	runtime := runtimeStage(t, containerfile)

	buildPackages := apkPackages(build)
	for _, tool := range []string{"gcc", "musl-dev"} {
		if !slices.Contains(buildPackages, tool) {
			t.Fatalf("build installs %v, want %s for CGO SQLite", buildPackages, tool)
		}
	}
	if packages := apkPackages(runtime); len(packages) != 1 || packages[0] != "ca-certificates" {
		t.Fatalf("runtime installs %v, want only CA certificates", packages)
	}
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

func TestContainerfileReadsMountedConfig(t *testing.T) {
	root := repoRoot(t)
	runtime := runtimeStage(t, readFile(t, filepath.Join(root, "deploy", "container", "Containerfile")))

	home := stageEnv(t, runtime, "HOME")
	configPath := filepath.Join(home, ".invoicer", "config.yaml")
	if configPath != "/home/invoicer/.invoicer/config.yaml" {
		t.Fatalf("config path: got %q, want /home/invoicer/.invoicer/config.yaml", configPath)
	}
	if !strings.Contains(runtime, "mkdir -p "+filepath.Dir(configPath)) {
		t.Fatal("image does not create the config directory")
	}
	if regexp.MustCompile(`(?mi)^ENV\b.*config(?:[_-]?path)?`).MatchString(runtime) {
		t.Fatal("image sets a config path instead of reading it from HOME")
	}
}

func TestContainerfileReadsMountedDatabase(t *testing.T) {
	root := repoRoot(t)
	runtime := runtimeStage(t, readFile(t, filepath.Join(root, "deploy", "container", "Containerfile")))

	if regexp.MustCompile(`(?mi)^ENV\b.*(?:database|db[_-]?path)`).MatchString(runtime) {
		t.Fatal("image sets a database path instead of reading it from the config")
	}
	if regexp.MustCompile(`(?m)--(?:database|db)\b`).MatchString(runtime) {
		t.Fatal("image passes a database path instead of leaving it to the config")
	}
}

func TestContainerfileLeavesWritablePathsToMounts(t *testing.T) {
	root := repoRoot(t)
	runtime := runtimeStage(t, readFile(t, filepath.Join(root, "deploy", "container", "Containerfile")))

	if regexp.MustCompile(`(?m)^VOLUME\b`).MatchString(runtime) {
		t.Fatal("image declares a volume instead of leaving data mounts to the deployment")
	}
	if regexp.MustCompile(`(?m)^WORKDIR\b`).MatchString(runtime) {
		t.Fatal("image sets a working directory that a read-only root filesystem cannot write")
	}
}

func stageEnv(t *testing.T, stage, name string) string {
	t.Helper()
	match := regexp.MustCompile(`(?m)^ENV ` + regexp.QuoteMeta(name) + `=(\S+)$`).FindStringSubmatch(stage)
	if match == nil {
		t.Fatalf("runtime stage does not set %s", name)
	}
	return match[1]
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

func TestContainerfileAcceptsAppVersion(t *testing.T) {
	root := repoRoot(t)
	build := buildStage(t, readFile(t, filepath.Join(root, "deploy", "container", "Containerfile")))

	if !regexp.MustCompile(`(?m)^ARG VERSION(?:=|$)`).MatchString(build) {
		t.Fatal("build stage does not accept a VERSION build argument")
	}
	if !regexp.MustCompile(`-X github\.com/zon/invoicer/internal/cli\.Version=\$\{?VERSION\}?`).MatchString(build) {
		t.Fatal("build does not stamp VERSION into the CLI version")
	}
}

func TestContainerfileDefaultsVersionForBuildWithoutArgument(t *testing.T) {
	root := repoRoot(t)
	build := buildStage(t, readFile(t, filepath.Join(root, "deploy", "container", "Containerfile")))

	match := regexp.MustCompile(`(?m)^ARG VERSION=(\S+)$`).FindStringSubmatch(build)
	if match == nil {
		t.Fatal("build without a VERSION argument stamps an empty version")
	}
	if match[1] == "" {
		t.Fatal("VERSION defaults to empty")
	}
}
