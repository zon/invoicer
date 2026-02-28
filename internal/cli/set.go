package cli

// SetCmd groups subcommands under "set".
type SetCmd struct {
	Config SetConfigCmd `cmd:"" name:"config" help:"Write configuration options to ~/.invoicer/config.yaml."`
}

// SetConfigCmd is the 'set config' subcommand.
// It accepts the same options as the main command and writes them to ~/.invoicer/config.yaml.
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

// Run executes the 'set config' subcommand, writing options to ~/.invoicer/config.yaml.
func (s *SetConfigCmd) Run() error {
	// Config writing logic will be implemented in the config category.
	return nil
}
