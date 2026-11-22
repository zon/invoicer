package web

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"strconv"

	"github.com/signintech/gopdf"

	"github.com/zon/invoicer/internal/invoice"
)

// The font is embedded so a PDF renders from the binary alone. Its OFL license
// sits beside it in fonts/OFL.txt.
//
//go:embed fonts/NotoSans-Regular.ttf
var pdfFont []byte

const (
	pdfFontFamily = "Noto Sans"
	pdfFontSize   = 12

	pdfMargin        = 48.0
	pdfGap           = 10.0
	pdfLineHeight    = 16.0
	pdfTitleSize     = 24.0
	pdfTitleHeight   = 32.0
	pdfSectionSize   = 14.0
	pdfSectionHeight = 20.0
	pdfMarkerSize    = 24.0
	pdfMarkerHeight  = 32.0
	pdfRightColumn   = 90.0
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

// renderInvoicePDF returns inv as an A4 portrait PDF.
func renderInvoicePDF(inv *invoice.Invoice) ([]byte, error) {
	w, err := newPDFWriter()
	if err != nil {
		return nil, err
	}
	w.invoice(inv)
	if w.err != nil {
		return nil, w.err
	}

	var out bytes.Buffer
	if err := w.pdf.Write(&out); err != nil {
		return nil, fmt.Errorf("writing invoice PDF: %w", err)
	}
	return out.Bytes(), nil
}

// pdfWriter lays text out on A4 pages and adds a page when the next line does
// not fit.
type pdfWriter struct {
	pdf *gopdf.GoPdf
	y   float64
	err error
}

func newPDFWriter() (*pdfWriter, error) {
	pdf, err := newInvoicePDF()
	if err != nil {
		return nil, err
	}
	w := &pdfWriter{pdf: pdf}
	w.addPage()
	return w, nil
}

func (w *pdfWriter) invoice(inv *invoice.Invoice) {
	if inv.Paid {
		w.paidMarker()
	}
	w.write("Invoice", pdfTitleSize, pdfTitleHeight)
	w.gap()
	w.field("Vendor", inv.Vendor)
	w.field("Customer", inv.Customer)
	if inv.Project != "" {
		w.field("Project", inv.Project)
	}
	w.field("Month", inv.Month.String())
	w.field("Year", strconv.Itoa(inv.Year))
	w.field("Hourly rate", fmt.Sprintf("$%.2f", inv.Rate))
	w.field("Status", paidStatus(inv.Paid))
	w.gap()
	w.write("Weekly line items", pdfSectionSize, pdfSectionHeight)
	w.columns("Week", "Hours")
	for _, week := range inv.Weeks {
		w.columns(invoice.FormatWeekLabel(week), fmt.Sprintf("%g", week.Hours))
	}
	w.gap()
	w.write("Total: "+fmt.Sprintf("$%.2f", inv.Total()), pdfSectionSize, pdfSectionHeight)
}

func (w *pdfWriter) field(label, value string) {
	w.write(label+": "+value, pdfFontSize, pdfLineHeight)
}

func (w *pdfWriter) paidMarker() {
	if w.err != nil {
		return
	}
	if err := w.pdf.SetFont(pdfFontFamily, "", pdfMarkerSize); err != nil {
		w.err = fmt.Errorf("setting invoice PDF marker font: %w", err)
		return
	}
	width, err := w.pdf.MeasureTextWidth("PAID")
	if err != nil {
		w.err = fmt.Errorf("measuring invoice PDF marker: %w", err)
		return
	}
	w.pdf.SetXY(gopdf.PageSizeA4.W-pdfMargin-width, pdfMargin)
	if err := w.pdf.Cell(&gopdf.Rect{W: width, H: pdfMarkerHeight}, "PAID"); err != nil {
		w.err = fmt.Errorf("writing invoice PDF marker: %w", err)
	}
}

func (w *pdfWriter) gap() {
	w.y += pdfGap
}

func (w *pdfWriter) addPage() {
	w.pdf.AddPage()
	w.y = pdfMargin
}

func (w *pdfWriter) contentWidth() float64 {
	return gopdf.PageSizeA4.W - 2*pdfMargin
}

func (w *pdfWriter) ensureSpace(height float64) {
	if w.y+height > gopdf.PageSizeA4.H-pdfMargin {
		w.addPage()
	}
}

func (w *pdfWriter) write(text string, size, lineHeight float64) {
	if w.err != nil {
		return
	}
	if err := w.pdf.SetFont(pdfFontFamily, "", size); err != nil {
		w.err = fmt.Errorf("setting invoice PDF font: %w", err)
		return
	}

	lines, err := w.pdf.SplitTextWithWordWrap(text, w.contentWidth())
	if errors.Is(err, gopdf.ErrEmptyString) {
		lines = []string{""}
	} else if err != nil {
		w.err = fmt.Errorf("wrapping invoice PDF text: %w", err)
		return
	}

	for _, line := range lines {
		w.ensureSpace(lineHeight)
		w.pdf.SetXY(pdfMargin, w.y)
		if err := w.pdf.Cell(&gopdf.Rect{W: w.contentWidth(), H: lineHeight}, line); err != nil {
			w.err = fmt.Errorf("writing invoice PDF text: %w", err)
			return
		}
		w.y += lineHeight
	}
}

func (w *pdfWriter) columns(left, right string) {
	if w.err != nil {
		return
	}
	if err := w.pdf.SetFont(pdfFontFamily, "", pdfFontSize); err != nil {
		w.err = fmt.Errorf("setting invoice PDF font: %w", err)
		return
	}

	w.ensureSpace(pdfLineHeight)
	leftWidth := w.contentWidth() - pdfRightColumn

	w.pdf.SetXY(pdfMargin, w.y)
	if err := w.pdf.Cell(&gopdf.Rect{W: leftWidth, H: pdfLineHeight}, left); err != nil {
		w.err = fmt.Errorf("writing invoice PDF column: %w", err)
		return
	}
	w.pdf.SetXY(pdfMargin+leftWidth, w.y)
	align := gopdf.CellOption{Align: gopdf.Right | gopdf.Top}
	if err := w.pdf.CellWithOption(&gopdf.Rect{W: pdfRightColumn, H: pdfLineHeight}, right, align); err != nil {
		w.err = fmt.Errorf("writing invoice PDF column: %w", err)
		return
	}
	w.y += pdfLineHeight
}
