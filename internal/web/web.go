// Package web serves the invoice web app.
package web

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/zon/invoicer/internal/invoice"
)

//go:embed templates/*.html
var templateFiles embed.FS

// Store provides the invoices the web app reads.
type Store interface {
	List() ([]*invoice.Invoice, error)
}

// Server serves the invoice web app.
type Server struct {
	*gin.Engine
	store Store
}

// New returns a server that lists invoices from s.
func New(s Store) *Server {
	server := &Server{
		Engine: gin.New(),
		store:  s,
	}
	server.SetHTMLTemplate(template.Must(template.ParseFS(templateFiles, "templates/*.html")))
	server.GET("/", server.index)
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

type pageData struct {
	Invoices []invoiceRow
}

type invoiceRow struct {
	Customer string
	Month    string
	Year     int
	Project  string
	Total    string
	Path     string
}

func (s *Server) index(c *gin.Context) {
	invoices, err := s.store.List()
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	c.HTML(http.StatusOK, "index.html", pageData{Invoices: rows(invoices)})
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

func detailPath(year int, month time.Month) string {
	return fmt.Sprintf("/%d/%s", year, strings.ToLower(month.String()[:3]))
}
