package scripts_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// chartAppVersionFixture returns a chart directory with the given appVersion.
func chartAppVersionFixture(t *testing.T, appVersion string) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Chart.yaml"),
		"apiVersion: v2\nname: invoicer\nversion: 0.2.0\nappVersion: \""+appVersion+"\"\n")
	return dir
}

// runChartAppVersionCheck runs the chart app version check.
func runChartAppVersionCheck(t *testing.T, chartDir, appVersion string) (string, error) {
	t.Helper()
	cmd := exec.Command("sh",
		filepath.Join(repoRoot(t), "scripts", "check-chart-app-version.sh"), chartDir, appVersion)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func TestCheckChartAppVersionAcceptsMatch(t *testing.T) {
	output, err := runChartAppVersionCheck(t, chartAppVersionFixture(t, "0.2.0"), "0.2.0")

	if err != nil {
		t.Fatalf("check chart app version: %v\n%s", err, output)
	}
}

func TestCheckChartAppVersionFailsOnMismatch(t *testing.T) {
	output, err := runChartAppVersionCheck(t, chartAppVersionFixture(t, "0.1.0"), "0.2.0")

	if err == nil {
		t.Fatalf("check accepted appVersion 0.1.0 for app version 0.2.0:\n%s", output)
	}
	if !strings.Contains(output, "0.1.0") || !strings.Contains(output, "0.2.0") {
		t.Fatalf("check does not report both versions:\n%s", output)
	}
}

func TestCheckChartAppVersionFailsWithoutAppVersion(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Chart.yaml"), "apiVersion: v2\nname: invoicer\nversion: 0.2.0\n")

	output, err := runChartAppVersionCheck(t, dir, "0.2.0")

	if err == nil {
		t.Fatalf("check accepted a chart without an appVersion:\n%s", output)
	}
}
