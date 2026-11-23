package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/zon/invoicer/internal/store"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// buildInvoicer builds the invoicer binary into a temporary directory.
func buildInvoicer(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "invoicer")

	build := exec.Command("go", "build", "-o", binary, "./cmd/invoicer")
	build.Dir = repoRoot(t)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, output)
	}
	return binary
}

// TestVersionFlagPrintsVersionAndExits verifies the built binary reports its
// version and returns without running a command.
func TestVersionFlagPrintsVersionAndExits(t *testing.T) {
	binary := buildInvoicer(t)

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

// TestGenerateFailedSendExitsNonZero verifies a failed send reports an error,
// exits non-zero, and leaves the saved invoice without a send time.
func TestGenerateFailedSendExitsNonZero(t *testing.T) {
	binary := buildInvoicer(t)
	home := t.TempDir()

	configDir := filepath.Join(home, ".invoicer")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("creating config directory: %v", err)
	}
	dbPath := filepath.Join(home, "invoices.db")
	config := fmt.Sprintf(`database: %s
customerEmail: billing@acme.com
baseUrl: https://invoices.example.com
resend:
  apiKeyFile: %s
  from: invoicer@example.com
`, dbPath, filepath.Join(home, "missing-api-key"))
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(config), 0o600); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cmd := exec.Command(binary, "january", "2025",
		"-v", "Jane Contractor",
		"-c", "Acme Corp",
		"-r", "150",
		"-H", "40",
	)
	cmd.Env = append(os.Environ(), "HOME="+home)
	output, err := cmd.CombinedOutput()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("invoicer generate: got %v, want a non-zero exit\n%s", err, output)
	}
	if !bytes.Contains(output, []byte("reading resend.apiKeyFile")) {
		t.Errorf("output: got %q, want the failed send error", output)
	}

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer s.Close()

	invoices, err := s.List()
	if err != nil {
		t.Fatalf("listing invoices: %v", err)
	}
	if len(invoices) != 1 {
		t.Fatalf("invoices: got %d, want the saved invoice", len(invoices))
	}
	if !invoices[0].EmailSentAt.IsZero() {
		t.Errorf("EmailSentAt: got %v, want no send time", invoices[0].EmailSentAt)
	}
}
