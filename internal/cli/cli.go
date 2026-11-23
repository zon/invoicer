// Package cli defines the command-line interface for invoicer.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/zon/invoicer/internal/config"
	"github.com/zon/invoicer/internal/invoice"
	"github.com/zon/invoicer/internal/store"
)

const defaultWebAddress = ":8080"

// Version is the app version reported by --version. The build stamps it with
// -ldflags. A binary without a stamp reports "dev".
var Version = "dev"

// CLI is the root command for invoicer.
type CLI struct {
	// Version prints the app version and exits.
	Version kong.VersionFlag `help:"Print the app version and exit."`

	// Generate is the default subcommand for generating an invoice.
	Generate GenerateCmd `cmd:"" default:"withargs" help:"Generate an invoice for a given month."`

	// Set is the 'set' subcommand group for managing configuration.
	Set SetCmd `cmd:"" name:"set" help:"Subcommands for managing invoicer configuration."`

	// Remove is the subcommand for deleting a stored invoice.
	Remove RemoveCmd `cmd:"" help:"Remove a stored invoice by its identifier."`

	// Web is the subcommand for serving the invoice web app.
	Web WebCmd `cmd:"" help:"Start the invoice web server."`
}

// GenerateCmd is the default subcommand for generating an invoice.
type GenerateCmd struct {
	// Month is the month to invoice for (text or numeric). Defaults to previous month.
	Month string `arg:"" optional:"" help:"Month to invoice for (e.g. 'january', 'jan', or '1'). Defaults to previous month."`

	// Year is the year of the month to invoice for. Defaults to the year closest to the given month.
	Year int `arg:"" optional:"" help:"Year of the month to invoice for. Defaults to the year closest to the given month."`

	// Vendor is the name of the contractor sending the invoice.
	Vendor string `short:"v" help:"Name of the contractor sending the invoice. Required without config."`

	// Customer is the name of the customer receiving the invoice.
	Customer string `short:"c" help:"Name of the customer receiving the invoice. Required without config."`

	// Project is an optional project name to annotate the invoice with.
	Project string `help:"Name of the project to annotate the invoice with."`

	// Database is the path to the invoice database.
	Database string `help:"Path to the invoice database. Defaults to ~/.invoicer/invoices.db."`

	// Rate is the hourly rate for the contractor.
	Rate float64 `short:"r" help:"Hourly rate in dollars. Required without config."`

	// Hours is the number of hours per week worked.
	Hours float64 `short:"H" help:"Hours per week worked. Required without config."`
}

// resolveOptions merges config file values with CLI-provided values.
// CLI values take precedence over config file values.
// configPath may be empty to use the default path.
func (c *GenerateCmd) resolveOptions(configPath string) (*ResolvedOptions, error) {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return nil, err
	}

	opts := &ResolvedOptions{
		Month: c.Month,
		Year:  c.Year,
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

	opts.Project = c.Project
	if opts.Project == "" {
		opts.Project = cfg.Project
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

	// Merge database: CLI value takes precedence, then config, then the default.
	opts.Database, err = databasePath(c.Database, cfg)
	if err != nil {
		return nil, err
	}

	return opts, nil
}

// loadConfig loads the config at path, or the default config path when empty.
func loadConfig(configPath string) (*config.Config, error) {
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
	return cfg, nil
}

// databasePath resolves the database path from the flag, then config, then the default.
func databasePath(flag string, cfg *config.Config) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if cfg.Database != "" {
		return cfg.Database, nil
	}
	path, err := config.DefaultDatabasePath()
	if err != nil {
		return "", fmt.Errorf("determining default database path: %w", err)
	}
	return path, nil
}

// ResolvedOptions holds the final merged values after CLI and config are combined.
type ResolvedOptions struct {
	Month    string
	Year     int
	Vendor   string
	Customer string
	Project  string
	Rate     float64
	Hours    float64
	Database string
}

// Run executes the generate subcommand (invoice generation).
func (c *GenerateCmd) Run() error {
	return c.run("", os.Stdout)
}

// run executes generate with an explicit config path and output writer.
func (c *GenerateCmd) run(configPath string, out io.Writer) error {
	opts, err := c.resolveOptions(configPath)
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

	inv := &invoice.Invoice{
		Month:    month,
		Year:     year,
		Vendor:   opts.Vendor,
		Customer: opts.Customer,
		Project:  opts.Project,
		Rate:     opts.Rate,
		Weeks:    invoice.WeeksForMonth(year, month, opts.Hours),
	}

	s, err := store.Open(opts.Database)
	if err != nil {
		return fmt.Errorf("opening invoice database: %w", err)
	}
	defer s.Close()

	id, created, err := s.Save(inv)
	if errors.Is(err, store.ErrPaid) {
		return err
	}
	if err != nil {
		return fmt.Errorf("saving invoice: %w", err)
	}

	action := "Edited"
	if created {
		action = "Created"
	}
	fmt.Fprintf(out, "%s invoice for %s %d: %s\n", action, month.String(), year, strings.ToUpper(id))
	return nil
}
