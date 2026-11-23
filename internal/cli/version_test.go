package cli

import (
	"bytes"
	"testing"

	"github.com/alecthomas/kong"
)

// newVersionParser returns a parser for the root CLI that reports the app
// version and records the exit code a version request asks for.
func newVersionParser(t *testing.T, out *bytes.Buffer, exitCode *int) *kong.Kong {
	t.Helper()
	var cmd CLI
	parser, err := kong.New(&cmd,
		kong.Name("invoicer"),
		kong.Vars{"version": Version},
		kong.Writers(out, out),
		kong.Exit(func(code int) { *exitCode = code }),
	)
	if err != nil {
		t.Fatalf("kong.New failed: %v", err)
	}
	return parser
}

func TestCLIVersionPrintsAppVersion(t *testing.T) {
	var out bytes.Buffer
	exitCode := -1
	parser := newVersionParser(t, &out, &exitCode)

	if _, err := parser.Parse([]string{"--version"}); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exit code: got %d, want 0", exitCode)
	}
	if got, want := out.String(), Version+"\n"; got != want {
		t.Errorf("output: got %q, want %q", got, want)
	}
}

func TestVersionReportsAValueWithoutAStamp(t *testing.T) {
	if Version == "" {
		t.Fatal("an unstamped binary reports no version")
	}
}
