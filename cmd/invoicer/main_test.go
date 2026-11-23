package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// TestVersionFlagPrintsVersionAndExits verifies the built binary reports its
// version and returns without running a command.
func TestVersionFlagPrintsVersionAndExits(t *testing.T) {
	root := repoRoot(t)
	binary := filepath.Join(t.TempDir(), "invoicer")

	build := exec.Command("go", "build", "-o", binary, "./cmd/invoicer")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, output)
	}

	home := t.TempDir()
	cmd := exec.Command(binary, "--version")
	cmd.Env = append(os.Environ(), "HOME="+home)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("invoicer --version: %v", err)
	}
	if got, want := string(output), "dev\n"; got != want {
		t.Errorf("invoicer --version: got %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(home, ".invoicer")); !os.IsNotExist(err) {
		t.Error("invoicer --version wrote config or database files")
	}
}
