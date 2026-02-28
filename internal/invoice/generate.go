package invoice

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// OpencodeExec is the function used to run the opencode subprocess.
// It can be overridden in tests to use a fake binary.
var OpencodeExec = func(model, dir, prompt string) ([]byte, error) {
	cmd := exec.Command("opencode", "run",
		"--model", model,
		"--format", "json",
		"--dir", dir,
		prompt,
	)
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

// GenerateHTML prompts opencode to generate an HTML invoice and writes it to outputPath.
// model is the opencode-formatted model stub (e.g. "anthropic/claude-haiku-4-5").
func GenerateHTML(inv *Invoice, model, outputPath string) error {
	prompt := BuildPrompt(inv, outputPath)

	out, err := OpencodeExec(model, filepath.Dir(outputPath), prompt)
	if err != nil {
		return fmt.Errorf("running opencode: %w", err)
	}

	// Parse JSON lines to check for errors or confirm file was written.
	if err := CheckOpencodeOutput(out, outputPath); err != nil {
		return err
	}

	return nil
}

// BuildPrompt creates the opencode prompt for generating the HTML invoice.
func BuildPrompt(inv *Invoice, outputPath string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(
		"Generate a professional HTML invoice for the following contract work. "+
			"Use creative, unique styling with random color schemes and typography. "+
			"Make it visually appealing and modern. "+
			"Write the complete HTML (with embedded CSS) to the file: %s\n\n",
		outputPath,
	))

	sb.WriteString(fmt.Sprintf("Invoice Details:\n"))
	sb.WriteString(fmt.Sprintf("- Vendor (Contractor): %s\n", inv.Vendor))
	sb.WriteString(fmt.Sprintf("- Customer (Client): %s\n", inv.Customer))
	sb.WriteString(fmt.Sprintf("- Month: %s %d\n", inv.Month.String(), inv.Year))
	sb.WriteString(fmt.Sprintf("- Hourly Rate: $%.2f\n", inv.Rate))
	sb.WriteString("\nWeekly Line Items:\n")

	for _, w := range inv.Weeks {
		weekLabel := FormatWeekLabel(w)
		subtotal := w.Hours * inv.Rate
		sb.WriteString(fmt.Sprintf("  - %s: %.1f hours @ $%.2f/hr = $%.2f\n",
			weekLabel, w.Hours, inv.Rate, subtotal))
	}

	sb.WriteString(fmt.Sprintf("\nTotal Amount: $%.2f\n", inv.Total()))
	sb.WriteString("\nRequirements:\n")
	sb.WriteString("- Complete HTML5 document with embedded CSS styling\n")
	sb.WriteString("- Unique, creative visual design with random color palette\n")
	sb.WriteString("- Professional invoice layout with all line items shown in a table\n")
	sb.WriteString("- Include invoice date and a generated invoice number\n")
	sb.WriteString("- Show totals clearly\n")
	sb.WriteString("- Write the file using the write tool - do not output the HTML in text\n")

	return sb.String()
}

// FormatWeekLabel returns a human-readable label for a week range.
func FormatWeekLabel(w Week) string {
	if w.Start.Month() == w.End.Month() {
		return fmt.Sprintf("%s %d-%d", w.Start.Month().String()[:3], w.Start.Day(), w.End.Day())
	}
	return fmt.Sprintf("%s %d - %s %d",
		w.Start.Month().String()[:3], w.Start.Day(),
		w.End.Month().String()[:3], w.End.Day())
}

// opencodeEvent represents a single JSON event line from opencode --format json.
type opencodeEvent struct {
	Type string          `json:"type"`
	Part json.RawMessage `json:"part"`
}

// toolState represents the state of a tool_use event's part.
type toolPart struct {
	Tool  string    `json:"tool"`
	State toolState `json:"state"`
}

type toolState struct {
	Status string          `json:"status"`
	Input  json.RawMessage `json:"input"`
	Output string          `json:"output"`
}

type writeInput struct {
	FilePath string `json:"filePath"`
}

// CheckOpencodeOutput parses the JSON lines from opencode and verifies the file was written.
func CheckOpencodeOutput(out []byte, expectedPath string) error {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var event opencodeEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if event.Type != "tool_use" {
			continue
		}
		var part toolPart
		if err := json.Unmarshal(event.Part, &part); err != nil {
			continue
		}
		if part.Tool != "write" {
			continue
		}
		var input writeInput
		if err := json.Unmarshal(part.State.Input, &input); err != nil {
			continue
		}
		// Check if this write was to our expected output path.
		if input.FilePath == expectedPath && part.State.Status == "completed" {
			return nil
		}
	}

	// Fallback: check if the file exists on disk.
	if _, err := os.Stat(expectedPath); err == nil {
		return nil
	}

	return fmt.Errorf("opencode did not write the HTML invoice to %s", expectedPath)
}

// ConvertToPDF converts an HTML file to PDF using an available tool.
// It tries wkhtmltopdf, then falls back to chromium/google-chrome headless.
func ConvertToPDF(htmlPath, pdfPath string) error {
	// Try wkhtmltopdf first.
	if path, err := exec.LookPath("wkhtmltopdf"); err == nil {
		cmd := exec.Command(path, htmlPath, pdfPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("wkhtmltopdf: %w", err)
		}
		return nil
	}

	// Try chromium/google-chrome headless.
	for _, browser := range []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable"} {
		if path, err := exec.LookPath(browser); err == nil {
			cmd := exec.Command(path,
				"--headless",
				"--disable-gpu",
				"--no-sandbox",
				"--print-to-pdf="+pdfPath,
				"file://"+htmlPath,
			)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("%s: %w", browser, err)
			}
			return nil
		}
	}

	return fmt.Errorf("no PDF conversion tool found (install wkhtmltopdf or chromium)")
}

// OutputFilename returns the output filename for an invoice (without extension).
func OutputFilename(inv *Invoice) string {
	return fmt.Sprintf("invoice-%s-%d-%02d",
		strings.ToLower(strings.ReplaceAll(inv.Customer, " ", "-")),
		inv.Year,
		int(inv.Month),
	)
}

// InvoiceFilePath returns the full path for the HTML invoice file.
func InvoiceFilePath(inv *Invoice, dir string) string {
	return filepath.Join(dir, OutputFilename(inv)+".html")
}

// PDFFilePath returns the full path for the PDF invoice file.
func PDFFilePath(inv *Invoice, dir string) string {
	return filepath.Join(dir, OutputFilename(inv)+".pdf")
}

// CurrentDir returns the working directory, falling back to temp dir.
func CurrentDir() string {
	if dir, err := os.Getwd(); err == nil {
		return dir
	}
	return os.TempDir()
}

// Now returns the current time. Exposed as a variable for testing.
var Now = func() time.Time {
	return time.Now()
}
