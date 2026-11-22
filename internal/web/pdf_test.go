package web

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/signintech/gopdf"

	"github.com/zon/invoicer/internal/invoice"
)

func TestInvoiceFontCoversText(t *testing.T) {
	var font gopdf.SubsetFontObj
	if err := font.SetTTFData(pdfFont); err != nil {
		t.Fatalf("reading embedded font: %v", err)
	}

	text := "Invoice — “quoted” ‘text’ … € £ ¥ © ® naïve Über Привет Ελένη"
	for _, r := range text {
		if _, err := font.CharCodeToGlyphIndex(r); err != nil {
			t.Errorf("embedded font has no glyph for %q (U+%04X): %v", r, r, err)
		}
	}
}

func TestNewInvoicePDFUsesEmbeddedFont(t *testing.T) {
	t.Chdir(t.TempDir())

	pdf, err := newInvoicePDF()
	if err != nil {
		t.Fatalf("newInvoicePDF: %v", err)
	}
	pdf.AddPage()
	if err := pdf.Cell(&gopdf.Rect{W: 400, H: 20}, "Invoice — Привет — Ελένη — naïve"); err != nil {
		t.Fatalf("writing text: %v", err)
	}

	var out bytes.Buffer
	if err := pdf.Write(&out); err != nil {
		t.Fatalf("writing PDF: %v", err)
	}
	if !bytes.HasPrefix(out.Bytes(), []byte("%PDF-")) {
		t.Error("output is not a PDF")
	}
	if !strings.Contains(out.String(), "/FontFile2") {
		t.Error("PDF does not embed a TrueType font")
	}
}

func TestInvoicePDFShowsStoredInvoice(t *testing.T) {
	data, err := renderInvoicePDF(testInvoice())
	if err != nil {
		t.Fatalf("renderInvoicePDF: %v", err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Fatal("output is not a PDF")
	}

	text, _ := readInvoicePDF(t, data)
	for _, want := range []string{
		"Jane Doe",
		"Acme Corp",
		"Website",
		"October",
		"2027",
		"$100.00",
		"Status: Unpaid",
		"Weekly line items",
		"Oct 4-10",
		"Oct 11-17",
		"Oct 18-24",
		"Oct 25-31",
		"Total: $10000.00",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("PDF does not show %q", want)
		}
	}
	for _, hours := range []string{"10", "20", "30", "40"} {
		if !hasPDFLine(text, hours) {
			t.Errorf("PDF does not show the week with %s hours", hours)
		}
	}
}

func TestInvoicePDFUsesA4PortraitPages(t *testing.T) {
	data, err := renderInvoicePDF(testInvoice())
	if err != nil {
		t.Fatalf("renderInvoicePDF: %v", err)
	}

	match := pdfMediaBox.FindSubmatch(data)
	if match == nil {
		t.Fatal("PDF has no page size")
	}
	width := pdfNumber(t, match[1])
	height := pdfNumber(t, match[2])
	if math.Abs(width-gopdf.PageSizeA4.W) > 0.01 || math.Abs(height-gopdf.PageSizeA4.H) > 0.01 {
		t.Errorf("page size: got %gx%g, want %gx%g", width, height, gopdf.PageSizeA4.W, gopdf.PageSizeA4.H)
	}
	if width >= height {
		t.Errorf("page is not portrait: %gx%g", width, height)
	}
}

func TestInvoicePDFOmitsUnsetProject(t *testing.T) {
	inv := testInvoice()
	inv.Project = ""

	data, err := renderInvoicePDF(inv)
	if err != nil {
		t.Fatalf("renderInvoicePDF: %v", err)
	}
	if text, _ := readInvoicePDF(t, data); strings.Contains(text, "Project") {
		t.Error("PDF shows a project that is not set")
	}
}

func TestInvoicePDFShowsPaidMarker(t *testing.T) {
	t.Run("paid", func(t *testing.T) {
		inv := testInvoice()
		inv.Paid = true

		data, err := renderInvoicePDF(inv)
		if err != nil {
			t.Fatalf("renderInvoicePDF: %v", err)
		}
		text, _ := readInvoicePDF(t, data)
		if !hasPDFLine(text, "Status: Paid") {
			t.Error("PDF does not show the paid status")
		}
		if !strings.Contains(text, "PAID") {
			t.Error("PDF has no paid marker")
		}
	})

	t.Run("unpaid", func(t *testing.T) {
		data, err := renderInvoicePDF(testInvoice())
		if err != nil {
			t.Fatalf("renderInvoicePDF: %v", err)
		}
		text, _ := readInvoicePDF(t, data)
		if !hasPDFLine(text, "Status: Unpaid") {
			t.Error("PDF does not show the unpaid status")
		}
		if strings.Contains(text, "PAID") {
			t.Error("unpaid PDF has a paid marker")
		}
	})
}

func TestInvoicePDFWrapsLongValues(t *testing.T) {
	inv := testInvoice()
	inv.Project = strings.Repeat("Long project name ", 40) + strings.Repeat("W", 300)

	data, err := renderInvoicePDF(inv)
	if err != nil {
		t.Fatalf("renderInvoicePDF: %v", err)
	}
	text, lines := readInvoicePDF(t, data)
	if !strings.Contains(text, "Project:") {
		t.Error("PDF does not show the project")
	}
	if got := strings.Count(text, "Long"); got != 40 {
		t.Errorf("PDF shows %d of the project's 40 words", got)
	}
	if !strings.Contains(text, strings.Repeat("W", 20)) {
		t.Error("PDF does not wrap an unbroken long value")
	}

	measurer, err := newInvoicePDF()
	if err != nil {
		t.Fatalf("newInvoicePDF: %v", err)
	}
	for _, line := range lines {
		if line.Y < 0 || line.Y > gopdf.PageSizeA4.H {
			t.Errorf("line %q is vertically outside the page", line.Text)
		}
		if err := measurer.SetFont(pdfFontFamily, "", line.Size); err != nil {
			t.Fatalf("setting measuring font: %v", err)
		}
		width, err := measurer.MeasureTextWidth(line.Text)
		if err != nil {
			t.Fatalf("measuring %q: %v", line.Text, err)
		}
		if line.X < 0 || line.X+width > gopdf.PageSizeA4.W+0.01 {
			t.Errorf("line %q runs outside the page", line.Text)
		}
	}
}

func TestInvoicePDFFlowsOntoMorePages(t *testing.T) {
	inv := testInvoice()
	inv.Weeks = nil
	start := time.Date(2027, time.January, 4, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 60; i++ {
		weekStart := start.AddDate(0, 0, 7*i)
		inv.Weeks = append(inv.Weeks, invoice.Week{
			Start: weekStart,
			End:   weekStart.AddDate(0, 0, 6),
			Hours: float64(i + 1),
		})
	}

	data, err := renderInvoicePDF(inv)
	if err != nil {
		t.Fatalf("renderInvoicePDF: %v", err)
	}
	if pages := pdfPages(data); pages < 2 {
		t.Fatalf("PDF has %d page(s), want more than one", pages)
	}

	text, _ := readInvoicePDF(t, data)
	if !hasPDFLine(text, "60") {
		t.Error("PDF does not show the last week's hours")
	}
	if last := inv.Weeks[len(inv.Weeks)-1]; !strings.Contains(text, invoice.FormatWeekLabel(last)) {
		t.Errorf("PDF does not show the last week %q", invoice.FormatWeekLabel(last))
	}
}

func testInvoice() *invoice.Invoice {
	return &invoice.Invoice{
		Month:    time.October,
		Year:     2027,
		Vendor:   "Jane Doe",
		Customer: "Acme Corp",
		Project:  "Website",
		Rate:     100,
		Weeks: []invoice.Week{
			week(2027, time.October, 4, 10),
			week(2027, time.October, 11, 20),
			week(2027, time.October, 18, 30),
			week(2027, time.October, 25, 40),
		},
	}
}

func week(year int, month time.Month, day int, hours float64) invoice.Week {
	start := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	return invoice.Week{Start: start, End: start.AddDate(0, 0, 6), Hours: hours}
}

type pdfTextLine struct {
	X    float64
	Y    float64
	Size float64
	Text string
}

var (
	pdfTextRun  = regexp.MustCompile(`(?s)BT\s+([0-9.]+) ([0-9.]+) TD\s+/F\d+ ([0-9.]+) Tf\s+.*?\[(.*?)\] TJ`)
	pdfKerning  = regexp.MustCompile(`>[^<]*<`)
	pdfGlyph    = regexp.MustCompile(`[0-9A-Fa-f]{4}`)
	pdfBFRange  = regexp.MustCompile(`<([0-9A-Fa-f]{4})><([0-9A-Fa-f]{4})><([0-9A-Fa-f]{4,8})>`)
	pdfPage     = regexp.MustCompile(`(?m)^\s*/Type /Page\s*$`)
	pdfMediaBox = regexp.MustCompile(`/MediaBox \[ 0 0 ([0-9.]+) ([0-9.]+) \]`)
)

// readInvoicePDF returns the text and text lines of the invoice PDF. gopdf
// writes glyph identifiers and a ToUnicode map, so the test reads both.
func readInvoicePDF(t *testing.T, data []byte) (string, []pdfTextLine) {
	t.Helper()

	glyphs := pdfUnicodeMap(t, data)
	var text strings.Builder
	var lines []pdfTextLine

	for _, stream := range pdfStreams(data) {
		for _, match := range pdfTextRun.FindAllSubmatch(stream, -1) {
			line := pdfTextLine{
				X:    pdfNumber(t, match[1]),
				Y:    pdfNumber(t, match[2]),
				Size: pdfNumber(t, match[3]),
				Text: pdfDecode(t, match[4], glyphs),
			}
			lines = append(lines, line)
			text.WriteString(line.Text)
			text.WriteByte('\n')
		}
	}
	if len(lines) == 0 {
		t.Fatal("PDF has no text")
	}
	return text.String(), lines
}

func hasPDFLine(text, want string) bool {
	for _, line := range strings.Split(text, "\n") {
		if line == want {
			return true
		}
	}
	return false
}

func pdfPages(data []byte) int {
	return len(pdfPage.FindAll(data, -1))
}

func pdfStreams(data []byte) [][]byte {
	var streams [][]byte
	for {
		start := bytes.Index(data, []byte("stream"))
		if start < 0 {
			break
		}
		data = bytes.TrimPrefix(data[start+len("stream"):], []byte("\r"))
		data = bytes.TrimPrefix(data, []byte("\n"))

		end := bytes.Index(data, []byte("endstream"))
		if end < 0 {
			break
		}
		raw := bytes.TrimSuffix(data[:end], []byte("\n"))
		raw = bytes.TrimSuffix(raw, []byte("\r"))
		data = data[end+len("endstream"):]

		if decoded, ok := inflate(raw); ok {
			raw = decoded
		}
		streams = append(streams, raw)
	}
	return streams
}

func inflate(data []byte) ([]byte, bool) {
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, false
	}
	defer reader.Close()

	decoded, err := io.ReadAll(reader)
	if err != nil {
		return nil, false
	}
	return decoded, true
}

func pdfUnicodeMap(t *testing.T, data []byte) map[int]rune {
	t.Helper()

	glyphs := make(map[int]rune)
	for _, stream := range pdfStreams(data) {
		for _, match := range pdfBFRange.FindAllSubmatch(stream, -1) {
			first := int(pdfHex(t, match[1]))
			last := int(pdfHex(t, match[2]))
			r := pdfRune(t, match[3])
			for code := first; code <= last; code++ {
				glyphs[code] = r
			}
		}
	}
	if len(glyphs) == 0 {
		t.Fatal("PDF has no ToUnicode map")
	}
	return glyphs
}

func pdfDecode(t *testing.T, body []byte, glyphs map[int]rune) string {
	t.Helper()

	body = pdfKerning.ReplaceAll(body, nil)
	var text strings.Builder
	for _, code := range pdfGlyph.FindAll(body, -1) {
		glyph := int(pdfHex(t, code))
		r, ok := glyphs[glyph]
		if !ok {
			t.Fatalf("PDF uses glyph %d with no Unicode map entry", glyph)
		}
		text.WriteRune(r)
	}
	return text.String()
}

func pdfNumber(t *testing.T, value []byte) float64 {
	t.Helper()

	number, err := strconv.ParseFloat(string(value), 64)
	if err != nil {
		t.Fatalf("parsing PDF number %q: %v", value, err)
	}
	return number
}

func pdfHex(t *testing.T, value []byte) uint64 {
	t.Helper()

	number, err := strconv.ParseUint(string(value), 16, 64)
	if err != nil {
		t.Fatalf("parsing PDF hex %q: %v", value, err)
	}
	return number
}

func pdfRune(t *testing.T, value []byte) rune {
	t.Helper()

	encoded, err := hex.DecodeString(string(value))
	if err != nil {
		t.Fatalf("decoding PDF unicode %q: %v", value, err)
	}
	units := make([]uint16, 0, len(encoded)/2)
	for i := 0; i+1 < len(encoded); i += 2 {
		units = append(units, uint16(encoded[i])<<8|uint16(encoded[i+1]))
	}
	runes := utf16.Decode(units)
	if len(runes) == 0 {
		t.Fatalf("PDF unicode %q holds no rune", value)
	}
	return runes[0]
}
