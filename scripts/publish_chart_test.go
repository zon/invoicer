package scripts_test

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const chartRegistry = "oci://ghcr.io/zon/charts"

// newFakeHelm returns a directory holding a fake helm that records its calls.
func newFakeHelm(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	script := `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$HELM_LOG"

command=$1
shift

destination=
while [ $# -gt 0 ]; do
	case "$1" in
		--destination)
			destination=$2
			shift 2
			;;
		--version)
			shift 2
			;;
		*)
			shift
			;;
	esac
done

case "$command" in
	package)
		mkdir -p "$destination"
		cp "$FAKE_PACKAGE" "$destination/$FAKE_PACKAGE_NAME"
		;;
	pull)
		case "${FAKE_PULL_MODE:-}" in
			missing)
				echo "Error: failed to pull: 404 Not Found" >&2
				exit 1
				;;
			denied)
				echo "Error: failed to pull: 401 Unauthorized" >&2
				exit 1
				;;
		esac
		mkdir -p "$destination"
		tar -xzf "$FAKE_PULL" -C "$destination"
		;;
	push)
		;;
esac
`
	path := filepath.Join(dir, "helm")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// chartFixture returns a chart directory with the given version.
func chartFixture(t *testing.T, version string) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Chart.yaml"), "apiVersion: v2\nname: invoicer\nversion: "+version+"\n")
	return dir
}

// chartArchive writes a chart package holding the given file contents.
func chartArchive(t *testing.T, files map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "chart.tgz")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := gzip.NewWriter(file)
	writer := tar.NewWriter(archive)

	for name, content := range files {
		header := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func chartFiles(version, values string) map[string]string {
	return map[string]string{
		"invoicer/Chart.yaml":  "apiVersion: v2\nname: invoicer\nversion: " + version + "\n",
		"invoicer/values.yaml": values,
	}
}

type publishChartResult struct {
	output string
	err    error
	log    string
}

// runPublishChart runs the chart publisher against a fake helm.
func runPublishChart(t *testing.T, version, pullMode, localValues, publishedValues string) publishChartResult {
	t.Helper()
	helmDir := newFakeHelm(t)
	logPath := filepath.Join(t.TempDir(), "helm.log")
	packagePath := chartArchive(t, chartFiles(version, localValues))
	pullPath := chartArchive(t, chartFiles(version, publishedValues))
	chartDir := chartFixture(t, version)

	cmd := exec.Command("sh", filepath.Join(repoRoot(t), "scripts", "publish-chart.sh"), chartDir, chartRegistry)
	cmd.Env = append(os.Environ(),
		"PATH="+helmDir+":"+os.Getenv("PATH"),
		"HELM_LOG="+logPath,
		"FAKE_PACKAGE="+packagePath,
		"FAKE_PULL="+pullPath,
		"FAKE_PULL_MODE="+pullMode,
		"FAKE_PACKAGE_NAME=invoicer-"+version+".tgz",
	)
	output, err := cmd.CombinedOutput()

	var log string
	if data, readErr := os.ReadFile(logPath); readErr == nil {
		log = string(data)
	}
	return publishChartResult{output: string(output), err: err, log: log}
}

// helmCall returns the logged call for a helm command, or an empty string.
func helmCall(log, command string) string {
	for _, line := range strings.Split(log, "\n") {
		if strings.HasPrefix(line, command+" ") {
			return line
		}
	}
	return ""
}

func TestPublishChartPushesNewVersion(t *testing.T) {
	result := runPublishChart(t, "1.2.3", "missing", "local chart", "")

	if result.err != nil {
		t.Fatalf("publish chart: %v\n%s", result.err, result.output)
	}
	pull := helmCall(result.log, "pull")
	if !strings.Contains(pull, chartRegistry+"/invoicer") {
		t.Fatalf("chart pull does not use the registry and chart name: %q", pull)
	}
	if !strings.Contains(pull, "--version 1.2.3") {
		t.Fatalf("chart pull does not use the Chart.yaml version: %q", pull)
	}
	if !strings.Contains(pull, "--untar") {
		t.Fatalf("chart pull does not unpack the published chart for comparison: %q", pull)
	}
	push := helmCall(result.log, "push")
	if push == "" {
		t.Fatalf("workflow does not push a new chart version:\n%s", result.log)
	}
	if !strings.Contains(push, "invoicer-1.2.3.tgz") {
		t.Fatalf("chart push does not use the packaged chart: %q", push)
	}
	if !strings.HasSuffix(strings.TrimSpace(push), " "+chartRegistry) {
		t.Fatalf("chart push does not use the chart registry: %q", push)
	}
}

func TestPublishChartSkipsPublishedVersionWithSameContent(t *testing.T) {
	result := runPublishChart(t, "1.2.3", "", "same chart", "same chart")

	if result.err != nil {
		t.Fatalf("publish chart: %v\n%s", result.err, result.output)
	}
	if push := helmCall(result.log, "push"); push != "" {
		t.Fatalf("chart pushed an already published version: %q", push)
	}
}

func TestPublishChartFailsOnDifferentContent(t *testing.T) {
	result := runPublishChart(t, "1.2.3", "", "local chart", "published chart")

	if result.err == nil {
		t.Fatalf("publish chart replaced a published version:\n%s", result.output)
	}
	if push := helmCall(result.log, "push"); push != "" {
		t.Fatalf("chart pushed a changed version: %q", push)
	}
}

func TestPublishChartFailsWhenPublishedVersionCannotBeChecked(t *testing.T) {
	result := runPublishChart(t, "1.2.3", "denied", "local chart", "")

	if result.err == nil {
		t.Fatalf("publish chart ignored a failed registry check:\n%s", result.output)
	}
	if push := helmCall(result.log, "push"); push != "" {
		t.Fatalf("chart pushed after a failed registry check: %q", push)
	}
}
