// Package cli defines the command-line interface for invoicer.
package cli

import (
	"fmt"

	"github.com/zon/invoicer/internal/config"
	"github.com/zon/invoicer/internal/invoice"
)

// CLI is the root command for invoicer.
type CLI struct {
	// Month is the month to invoice for (text or numeric). Defaults to previous month.
	Month string `arg:"" optional:"" help:"Month to invoice for (e.g. 'january', 'jan', or '1'). Defaults to previous month."`

	// Year is the year of the month to invoice for. Defaults to the year closest to the given month.
	Year int `arg:"" optional:"" help:"Year of the month to invoice for. Defaults to the year closest to the given month."`

	// Vendor is the name of the contractor sending the invoice.
	Vendor string `short:"v" help:"Name of the contractor sending the invoice. Required without config."`

	// Customer is the name of the client receiving the invoice.
	Customer string `short:"c" help:"Name of the client receiving the invoice. Required without config."`

	// Rate is the hourly rate for the contractor.
	Rate float64 `short:"r" help:"Hourly rate in dollars. Required without config."`

	// Hours is the number of hours per week worked.
	Hours float64 `short:"H" help:"Hours per week worked. Required without config."`

	// PDF controls whether the HTML invoice is converted to a PDF.
	PDF bool `short:"p" help:"Convert the HTML invoice to a PDF file. Defaults to false."`

	// Model is the opencode-formatted model stub to use for generation.
	Model string `short:"m" default:"anthropic/claude-haiku-4-5" help:"opencode-formatted model stub to use for invoice generation. Defaults to anthropic/claude-haiku-4-5."`

	// Set is the 'set config' subcommand.
	Set SetCmd `cmd:"" name:"set" help:"Subcommands for managing invoicer configuration."`
}

// resolveOptions merges config file values with CLI-provided values.
// CLI values take precedence over config file values.
// configPath may be empty to use the default path.
func (c *CLI) resolveOptions(configPath string) (*ResolvedOptions, error) {
	if configPath == "" {
		var err error
		configPath, err = config.DefaultPath()
		if err != nil {
			return nil, fmt.Errorf("determining config path: %w", err)
		}
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	opts := &ResolvedOptions{
		Month: c.Month,
		Year:  c.Year,
		PDF:   c.PDF,
	}

	// Merge string fields: CLI takes precedence, fall back to config.
	opts.Vendor = c.Vendor
	if opts.Vendor == "" {
		opts.Vendor = cfg.Vendor
	}

	opts.Customer = c.Customer
	if opts.Customer == "" {
		opts.Customer = cfg.Customer
	}

	// Merge numeric fields: CLI takes precedence (non-zero), fall back to config.
	opts.Rate = c.Rate
	if opts.Rate == 0 {
		opts.Rate = cfg.Rate
	}

	opts.Hours = c.Hours
	if opts.Hours == 0 {
		opts.Hours = cfg.Hours
	}

	// Merge PDF: CLI flag (-p) sets to true; if false (not set), use config value.
	if !c.PDF && cfg.PDF != nil {
		opts.PDF = *cfg.PDF
	}

	// Merge model: CLI default is "anthropic/claude-haiku-4-5"; config may override
	// but CLI explicit value takes precedence. Since we can't distinguish CLI default
	// from user-specified, we prefer the CLI value (which may be the default).
	opts.Model = c.Model
	if opts.Model == "" && cfg.Model != "" {
		opts.Model = cfg.Model
	}

	return opts, nil
}

// ResolvedOptions holds the final merged values after CLI and config are combined.
type ResolvedOptions struct {
	Month    string
	Year     int
	Vendor   string
	Customer string
	Rate     float64
	Hours    float64
	PDF      bool
	Model    string
}

// Run executes the root command (invoice generation).
func (c *CLI) Run() error {
	opts, err := c.resolveOptions("")
	if err != nil {
		return err
	}

	// Validate required options.
	if opts.Vendor == "" {
		return fmt.Errorf("vendor is required (use --vendor or set in config)")
	}
	if opts.Customer == "" {
		return fmt.Errorf("customer is required (use --customer or set in config)")
	}
	if opts.Rate == 0 {
		return fmt.Errorf("rate is required (use --rate or set in config)")
	}
	if opts.Hours == 0 {
		return fmt.Errorf("hours is required (use --hours or set in config)")
	}

	// Resolve month and year.
	month, year, err := invoice.ResolveMonthYear(opts.Month, opts.Year, invoice.Now())
	if err != nil {
		return fmt.Errorf("resolving month/year: %w", err)
	}

	// Build invoice.
	weeks := invoice.WeeksForMonth(year, month, opts.Hours)
	inv := &invoice.Invoice{
		Month:    month,
		Year:     year,
		Vendor:   opts.Vendor,
		Customer: opts.Customer,
		Rate:     opts.Rate,
		Weeks:    weeks,
	}

	// Determine output paths.
	dir := invoice.CurrentDir()
	htmlPath := invoice.InvoiceFilePath(inv, dir)

	// Generate HTML invoice via opencode.
	fmt.Printf("Generating invoice for %s %d...\n", month.String(), year)
	if err := invoice.GenerateHTML(inv, opts.Model, htmlPath); err != nil {
		return fmt.Errorf("generating HTML invoice: %w", err)
	}
	fmt.Printf("HTML invoice written to: %s\n", htmlPath)

	// Convert to PDF if requested.
	if opts.PDF {
		pdfPath := invoice.PDFFilePath(inv, dir)
		fmt.Printf("Converting to PDF...\n")
		if err := invoice.ConvertToPDF(htmlPath, pdfPath); err != nil {
			return fmt.Errorf("converting to PDF: %w", err)
		}
		fmt.Printf("PDF invoice written to: %s\n", pdfPath)
	}

	return nil
}
