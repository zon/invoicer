package invoice_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zon/invoicer/internal/invoice"
)

// testInvoice returns a sample invoice for use in tests.
func testInvoice() *invoice.Invoice {
	return &invoice.Invoice{
		Month:    time.January,
		Year:     2025,
		Vendor:   "Jane Contractor",
		Customer: "Acme Corp",
		Rate:     150.0,
		Weeks: []invoice.Week{
			{
				Start: time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2025, time.January, 5, 0, 0, 0, 0, time.UTC),
				Hours: 32.0,
			},
			{
				Start: time.Date(2025, time.January, 6, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2025, time.January, 12, 0, 0, 0, 0, time.UTC),
				Hours: 40.0,
			},
		},
	}
}

// --- BuildPrompt tests ---

func TestBuildPrompt_ContainsVendor(t *testing.T) {
	inv := testInvoice()
	prompt := invoice.BuildPrompt(inv, "/tmp/invoice.html")
	if !strings.Contains(prompt, "Jane Contractor") {
		t.Errorf("prompt does not contain vendor name, got: %s", prompt)
	}
}

func TestBuildPrompt_ContainsCustomer(t *testing.T) {
	inv := testInvoice()
	prompt := invoice.BuildPrompt(inv, "/tmp/invoice.html")
	if !strings.Contains(prompt, "Acme Corp") {
		t.Errorf("prompt does not contain customer name, got: %s", prompt)
	}
}

func TestBuildPrompt_ContainsMonthYear(t *testing.T) {
	inv := testInvoice()
	prompt := invoice.BuildPrompt(inv, "/tmp/invoice.html")
	if !strings.Contains(prompt, "January") {
		t.Errorf("prompt does not contain month name, got: %s", prompt)
	}
	if !strings.Contains(prompt, "2025") {
		t.Errorf("prompt does not contain year, got: %s", prompt)
	}
}

func TestBuildPrompt_ContainsRate(t *testing.T) {
	inv := testInvoice()
	prompt := invoice.BuildPrompt(inv, "/tmp/invoice.html")
	if !strings.Contains(prompt, "150.00") {
		t.Errorf("prompt does not contain hourly rate, got: %s", prompt)
	}
}

func TestBuildPrompt_ContainsOutputPath(t *testing.T) {
	inv := testInvoice()
	outputPath := "/tmp/invoice-acme-corp-2025-01.html"
	prompt := invoice.BuildPrompt(inv, outputPath)
	if !strings.Contains(prompt, outputPath) {
		t.Errorf("prompt does not contain output path %q, got: %s", outputPath, prompt)
	}
}

func TestBuildPrompt_ContainsWeekLineItems(t *testing.T) {
	inv := testInvoice()
	prompt := invoice.BuildPrompt(inv, "/tmp/invoice.html")
	// Should contain at least one weekly line item with hours.
	if !strings.Contains(prompt, "32.0") && !strings.Contains(prompt, "40.0") {
		t.Errorf("prompt does not contain weekly hours, got: %s", prompt)
	}
}

func TestBuildPrompt_ContainsTotal(t *testing.T) {
	inv := testInvoice()
	// Total = (32 + 40) * 150 = 10800.00
	prompt := invoice.BuildPrompt(inv, "/tmp/invoice.html")
	if !strings.Contains(prompt, "10800.00") {
		t.Errorf("prompt does not contain total amount, got: %s", prompt)
	}
}

func TestBuildPrompt_ContainsHTMLRequirements(t *testing.T) {
	inv := testInvoice()
	prompt := invoice.BuildPrompt(inv, "/tmp/invoice.html")
	// Must instruct opencode to write a complete HTML document.
	if !strings.Contains(strings.ToLower(prompt), "html") {
		t.Errorf("prompt does not mention HTML, got: %s", prompt)
	}
	// Must instruct to use random/unique styling.
	lp := strings.ToLower(prompt)
	if !strings.Contains(lp, "random") && !strings.Contains(lp, "unique") && !strings.Contains(lp, "creative") {
		t.Errorf("prompt does not mention random/unique/creative styling, got: %s", prompt)
	}
}

func TestBuildPrompt_InstructsWriteToolNotText(t *testing.T) {
	inv := testInvoice()
	prompt := invoice.BuildPrompt(inv, "/tmp/invoice.html")
	// Must tell opencode to write the file using the write tool, not output HTML as text.
	lp := strings.ToLower(prompt)
	if !strings.Contains(lp, "write") {
		t.Errorf("prompt does not instruct to write the file, got: %s", prompt)
	}
}

// --- CheckOpencodeOutput tests ---

// makeToolUseEvent creates a JSON line representing an opencode tool_use event.
func makeToolUseEvent(tool, filePath, status string) string {
	type writeInput struct {
		FilePath string `json:"filePath"`
	}
	input, _ := json.Marshal(writeInput{FilePath: filePath})
	type toolState struct {
		Status string          `json:"status"`
		Input  json.RawMessage `json:"input"`
		Output string          `json:"output"`
	}
	type toolPart struct {
		Tool  string    `json:"tool"`
		State toolState `json:"state"`
	}
	type event struct {
		Type string          `json:"type"`
		Part json.RawMessage `json:"part"`
	}
	partJSON, _ := json.Marshal(toolPart{
		Tool: tool,
		State: toolState{
			Status: status,
			Input:  input,
		},
	})
	ev, _ := json.Marshal(event{Type: "tool_use", Part: partJSON})
	return string(ev)
}

func TestCheckOpencodeOutput_SuccessWithWriteEvent(t *testing.T) {
	expectedPath := "/tmp/invoice.html"
	line := makeToolUseEvent("write", expectedPath, "completed")
	err := invoice.CheckOpencodeOutput([]byte(line), expectedPath)
	if err != nil {
		t.Errorf("expected success, got error: %v", err)
	}
}

func TestCheckOpencodeOutput_FailsWhenWrongPath(t *testing.T) {
	line := makeToolUseEvent("write", "/tmp/other.html", "completed")
	err := invoice.CheckOpencodeOutput([]byte(line), "/tmp/invoice.html")
	// No file on disk either, so should fail.
	if err == nil {
		t.Error("expected error for wrong path, got nil")
	}
}

func TestCheckOpencodeOutput_FailsWhenStatusNotCompleted(t *testing.T) {
	expectedPath := "/tmp/invoice.html"
	line := makeToolUseEvent("write", expectedPath, "error")
	err := invoice.CheckOpencodeOutput([]byte(line), expectedPath)
	// No file on disk, so should fail.
	if err == nil {
		t.Error("expected error for non-completed status, got nil")
	}
}

func TestCheckOpencodeOutput_FallbackToFileExistence(t *testing.T) {
	// Create a temp file to simulate the output file existing on disk.
	tmpFile, err := os.CreateTemp("", "invoice-*.html")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// No write event in output, but file exists on disk — should succeed.
	err = invoice.CheckOpencodeOutput([]byte(""), tmpFile.Name())
	if err != nil {
		t.Errorf("expected success via file existence fallback, got: %v", err)
	}
}

func TestCheckOpencodeOutput_FailsWhenNoEventAndNoFile(t *testing.T) {
	err := invoice.CheckOpencodeOutput([]byte(""), "/tmp/nonexistent-invoice-xyz.html")
	if err == nil {
		t.Error("expected error when no event and no file, got nil")
	}
}

func TestCheckOpencodeOutput_IgnoresNonWriteTools(t *testing.T) {
	expectedPath := "/tmp/invoice.html"
	// A tool_use event for a different tool (e.g. "bash") should be ignored.
	line := makeToolUseEvent("bash", expectedPath, "completed")
	err := invoice.CheckOpencodeOutput([]byte(line), expectedPath)
	// No file on disk, so should fail.
	if err == nil {
		t.Error("expected error when only non-write tool events present, got nil")
	}
}

func TestCheckOpencodeOutput_IgnoresInvalidJSON(t *testing.T) {
	expectedPath := "/tmp/invoice.html"
	// Mix of invalid JSON lines and a valid write event.
	validLine := makeToolUseEvent("write", expectedPath, "completed")
	output := fmt.Sprintf("not json\n%s\n{invalid}", validLine)
	err := invoice.CheckOpencodeOutput([]byte(output), expectedPath)
	if err != nil {
		t.Errorf("expected success despite invalid JSON lines, got: %v", err)
	}
}

// --- GenerateHTML tests ---

func TestGenerateHTML_CallsOpencodeWithCorrectArgs(t *testing.T) {
	inv := testInvoice()
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "invoice-acme-corp-2025-01.html")

	var capturedModel, capturedDir, capturedPrompt string

	// Override OpencodeExec to capture arguments and write a fake file.
	origExec := invoice.OpencodeExec
	defer func() { invoice.OpencodeExec = origExec }()

	invoice.OpencodeExec = func(model, dir, prompt string) ([]byte, error) {
		capturedModel = model
		capturedDir = dir
		capturedPrompt = prompt
		// Write a fake HTML file so CheckOpencodeOutput succeeds via fallback.
		if err := os.WriteFile(outputPath, []byte("<html>fake</html>"), 0644); err != nil {
			return nil, err
		}
		return []byte(""), nil
	}

	if err := invoice.GenerateHTML(inv, "anthropic/claude-haiku-4-5", outputPath); err != nil {
		t.Fatalf("GenerateHTML() error: %v", err)
	}

	if capturedModel != "anthropic/claude-haiku-4-5" {
		t.Errorf("model = %q, want %q", capturedModel, "anthropic/claude-haiku-4-5")
	}
	if capturedDir != tmpDir {
		t.Errorf("dir = %q, want %q", capturedDir, tmpDir)
	}
	if !strings.Contains(capturedPrompt, "Acme Corp") {
		t.Errorf("prompt does not contain customer, got: %s", capturedPrompt)
	}
}

func TestGenerateHTML_WritesHTMLFile(t *testing.T) {
	inv := testInvoice()
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "invoice-acme-corp-2025-01.html")

	origExec := invoice.OpencodeExec
	defer func() { invoice.OpencodeExec = origExec }()

	// Fake opencode writes the expected HTML file.
	invoice.OpencodeExec = func(model, dir, prompt string) ([]byte, error) {
		content := "<html><body>Invoice</body></html>"
		if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
			return nil, err
		}
		return []byte(""), nil
	}

	if err := invoice.GenerateHTML(inv, "anthropic/claude-haiku-4-5", outputPath); err != nil {
		t.Fatalf("GenerateHTML() error: %v", err)
	}

	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("expected HTML file to exist after GenerateHTML, but it does not")
	}
}

func TestGenerateHTML_ReturnsErrorWhenOpencodeFailsAndNoFile(t *testing.T) {
	inv := testInvoice()
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "invoice.html")

	origExec := invoice.OpencodeExec
	defer func() { invoice.OpencodeExec = origExec }()

	// Fake opencode returns empty output without writing a file.
	invoice.OpencodeExec = func(model, dir, prompt string) ([]byte, error) {
		return []byte(""), nil
	}

	err := invoice.GenerateHTML(inv, "anthropic/claude-haiku-4-5", outputPath)
	if err == nil {
		t.Error("expected error when opencode writes no file, got nil")
	}
}

func TestGenerateHTML_UsesWriteEventToConfirmSuccess(t *testing.T) {
	inv := testInvoice()
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "invoice-acme-corp-2025-01.html")

	origExec := invoice.OpencodeExec
	defer func() { invoice.OpencodeExec = origExec }()

	// Fake opencode that returns a valid write event AND writes the file.
	invoice.OpencodeExec = func(model, dir, prompt string) ([]byte, error) {
		if err := os.WriteFile(outputPath, []byte("<html>fake</html>"), 0644); err != nil {
			return nil, err
		}
		event := makeToolUseEvent("write", outputPath, "completed")
		return []byte(event), nil
	}

	if err := invoice.GenerateHTML(inv, "anthropic/claude-haiku-4-5", outputPath); err != nil {
		t.Fatalf("GenerateHTML() error: %v", err)
	}
}

// --- ConvertToPDF tests ---

func TestConvertToPDF_UsesAvailableTool(t *testing.T) {
	// Create a fake "wkhtmltopdf" script in a temp dir and put it on PATH.
	tmpDir := t.TempDir()

	fakeWkhtmltopdf := filepath.Join(tmpDir, "wkhtmltopdf")
	// Write a shell script that creates the output PDF file.
	script := "#!/bin/sh\ntouch \"$2\"\n"
	if err := os.WriteFile(fakeWkhtmltopdf, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	// Prepend tmpDir to PATH.
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	htmlPath := filepath.Join(tmpDir, "invoice.html")
	pdfPath := filepath.Join(tmpDir, "invoice.pdf")
	// Create a dummy HTML file.
	if err := os.WriteFile(htmlPath, []byte("<html></html>"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := invoice.ConvertToPDF(htmlPath, pdfPath); err != nil {
		t.Fatalf("ConvertToPDF() error: %v", err)
	}

	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		t.Error("expected PDF file to be created, but it does not exist")
	}
}

func TestConvertToPDF_ReturnsErrorWhenNoToolFound(t *testing.T) {
	// Use an empty PATH so no PDF tool is found.
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", "")
	defer os.Setenv("PATH", origPath)

	err := invoice.ConvertToPDF("/tmp/invoice.html", "/tmp/invoice.pdf")
	if err == nil {
		t.Error("expected error when no PDF tool is available, got nil")
	}
}
