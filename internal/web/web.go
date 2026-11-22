// Package web serves the invoice web app.
package web

import (
	"embed"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/zon/invoicer/internal/invoice"
	"github.com/zon/invoicer/internal/store"
)

//go:embed templates/*.html
var templateFiles embed.FS

//go:embed static/style.css
var styleCSS []byte

// Store provides the invoices the web app reads.
type Store interface {
	List() ([]*invoice.Invoice, error)
	GetByMonth(year int, month time.Month) (*invoice.Invoice, error)
}

// Server serves the invoice web app.
type Server struct {
	*gin.Engine
	store Store
}

// New returns a server that lists invoices from s.
func New(s Store) *Server {
	// Gin defaults to debug mode, which logs every route on startup. Use
	// release mode unless GIN_MODE asks for a different mode.
	if os.Getenv(gin.EnvGinMode) == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	server := &Server{
		Engine: gin.New(),
		store:  s,
	}
	server.SetHTMLTemplate(template.Must(template.ParseFS(templateFiles, "templates/*.html")))
	server.GET("/style.css", server.stylesheet)
	server.GET("/", server.index)
	server.GET("/:year/:month", server.show)
	return server
}

// NewServer returns a server that lists invoices from s.
func NewServer(s Store) *Server {
	return New(s)
}

// Handler returns the HTTP handler for the server.
func (s *Server) Handler() http.Handler {
	return s.Engine
}

// Router returns the HTTP router for the server.
func (s *Server) Router() http.Handler {
	return s.Engine
}

type indexData struct {
	Invoices []invoiceRow
}

type detailData struct {
	Invoice *invoiceDetail
}

type invoiceRow struct {
	Customer string
	Month    string
	Year     int
	Project  string
	Total    string
	Path     string
}

type invoiceDetail struct {
	Vendor   string
	Customer string
	Project  string
	Month    string
	Year     int
	Rate     string
	Total    string
	Weeks    []weekRow
}

type weekRow struct {
	Label string
	Hours string
}

func (s *Server) stylesheet(c *gin.Context) {
	c.Data(http.StatusOK, "text/css; charset=utf-8", styleCSS)
}

func (s *Server) index(c *gin.Context) {
	invoices, err := s.store.List()
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	c.HTML(http.StatusOK, "index.html", indexData{
		Invoices: rows(invoices),
	})
}

func (s *Server) show(c *gin.Context) {
	year, err := strconv.Atoi(c.Param("year"))
	if err != nil {
		s.notFound(c)
		return
	}

	month, err := invoice.ParseMonth(c.Param("month"))
	if err != nil {
		s.notFound(c)
		return
	}

	inv, err := s.store.GetByMonth(year, month)
	if errors.Is(err, store.ErrNotFound) {
		s.notFound(c)
		return
	}
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	c.HTML(http.StatusOK, "detail.html", detailData{
		Invoice: detail(inv),
	})
}

func (s *Server) notFound(c *gin.Context) {
	c.Status(http.StatusNotFound)
}

func rows(invoices []*invoice.Invoice) []invoiceRow {
	sorted := append([]*invoice.Invoice(nil), invoices...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Year != sorted[j].Year {
			return sorted[i].Year > sorted[j].Year
		}
		return sorted[i].Month > sorted[j].Month
	})

	result := make([]invoiceRow, 0, len(sorted))
	for _, inv := range sorted {
		result = append(result, invoiceRow{
			Customer: inv.Customer,
			Month:    inv.Month.String(),
			Year:     inv.Year,
			Project:  inv.Project,
			Total:    fmt.Sprintf("$%.2f", inv.Total()),
			Path:     detailPath(inv.Year, inv.Month),
		})
	}
	return result
}

func detail(inv *invoice.Invoice) *invoiceDetail {
	if inv == nil {
		return nil
	}
	return &invoiceDetail{
		Vendor:   inv.Vendor,
		Customer: inv.Customer,
		Project:  inv.Project,
		Month:    inv.Month.String(),
		Year:     inv.Year,
		Rate:     fmt.Sprintf("$%.2f", inv.Rate),
		Total:    fmt.Sprintf("$%.2f", inv.Total()),
		Weeks:    weeks(inv.Weeks),
	}
}

func weeks(items []invoice.Week) []weekRow {
	result := make([]weekRow, 0, len(items))
	for _, w := range items {
		result = append(result, weekRow{
			Label: invoice.FormatWeekLabel(w),
			Hours: fmt.Sprintf("%g", w.Hours),
		})
	}
	return result
}

func detailPath(year int, month time.Month) string {
	return fmt.Sprintf("/%d/%s", year, strings.ToLower(month.String()[:3]))
}
