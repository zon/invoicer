package web

import (
	"bytes"
	"strings"
	"testing"

	"github.com/signintech/gopdf"
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
