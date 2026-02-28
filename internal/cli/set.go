package cli

import (
	"fmt"

	"github.com/zon/invoicer/internal/config"
)

// SetCmd groups subcommands under "set".
type SetCmd struct {
	Config SetConfigCmd `cmd:"" name:"config" help:"Write configuration options to ~/.invoicer/config.yaml."`
}

// SetConfigCmd is the 'set config' subcommand.
// It accepts the same options as the main command and writes them to ~/.invoicer/config.yaml.
// Only explicitly provided options are written; others are left unchanged.
type SetConfigCmd struct {
	// Vendor is the name of the contractor sending the invoice.
	Vendor string `help:"Name of the contractor sending the invoice."`

	// Customer is the name of the client receiving the invoice.
	Customer string `help:"Name of the client receiving the invoice."`

	// Rate is the hourly rate for the contractor.
	Rate float64 `help:"Hourly rate in dollars."`

	// Hours is the number of hours per week worked.
	Hours float64 `help:"Hours per week worked."`

	// PDF controls whether the HTML invoice is converted to a PDF.
	PDF *bool `help:"Convert the HTML invoice to a PDF file."`

	// Model is the opencode-formatted model stub to use for generation.
	Model string `help:"opencode-formatted model stub to use for invoice generation."`
}

// Run executes the 'set config' subcommand, writing specified options to ~/.invoicer/config.yaml.
// Only options that are explicitly provided are updated; others remain unchanged.
func (s *SetConfigCmd) Run() error {
	return RunSetConfig(s, "")
}

// RunSetConfig writes the given SetConfigCmd options to the config file at path.
// If path is empty, the default path (~/.invoicer/config.yaml) is used.
// This function is exported for testability.
func RunSetConfig(s *SetConfigCmd, path string) error {
	if path == "" {
		var err error
		path, err = config.DefaultPath()
		if err != nil {
			return fmt.Errorf("determining config path: %w", err)
		}
	}

	updates := &config.Config{
		Vendor:   s.Vendor,
		Customer: s.Customer,
		Rate:     s.Rate,
		Hours:    s.Hours,
		PDF:      s.PDF,
		Model:    s.Model,
	}

	if err := config.Save(path, updates); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	return nil
}
