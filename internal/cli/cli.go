// Package cli defines the command-line interface for invoicer.
package cli

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

// Run executes the root command (invoice generation).
func (c *CLI) Run() error {
	// Invoice generation logic will be implemented in the invoice category.
	return nil
}
