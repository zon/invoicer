package web

import (
	_ "embed"
	"fmt"

	"github.com/signintech/gopdf"
)

// The font is embedded so a PDF renders from the binary alone. Its OFL license
// sits beside it in fonts/OFL.txt.
//
//go:embed fonts/NotoSans-Regular.ttf
var pdfFont []byte

const (
	pdfFontFamily = "Noto Sans"
	pdfFontSize   = 12
)

// newInvoicePDF returns a PDF that uses the embedded font.
func newInvoicePDF() (*gopdf.GoPdf, error) {
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	if err := pdf.AddTTFFontData(pdfFontFamily, pdfFont); err != nil {
		return nil, fmt.Errorf("adding invoice PDF font: %w", err)
	}
	if err := pdf.SetFont(pdfFontFamily, "", pdfFontSize); err != nil {
		return nil, fmt.Errorf("setting invoice PDF font: %w", err)
	}
	return pdf, nil
}
