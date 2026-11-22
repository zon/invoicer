package cli

import (
	"fmt"
	"net/http"

	"github.com/zon/invoicer/internal/store"
	"github.com/zon/invoicer/internal/web"
)

// WebCmd starts the invoice web server.
type WebCmd struct {
	// Address is the address the web server listens on.
	Address string `help:"Address the web server listens on." default:":8080"`

	// Database is the path to the invoice database.
	Database string `help:"Path to the invoice database. Defaults to ~/.invoicer/invoices.db."`
}

// WebOptions holds the web server settings after CLI and config are combined.
type WebOptions struct {
	Address  string
	Database string
}

// Run executes the web subcommand.
func (c *WebCmd) Run() error {
	return c.run("")
}

// listenAndServe starts the HTTP server. Tests replace it to inspect the
// address and handler without binding a port.
var listenAndServe = http.ListenAndServe

// run executes web with an explicit config path.
func (c *WebCmd) run(configPath string) error {
	opts, err := c.resolveOptions(configPath)
	if err != nil {
		return err
	}

	s, err := store.Open(opts.Database)
	if err != nil {
		return fmt.Errorf("opening invoice database: %w", err)
	}
	defer s.Close()

	return listenAndServe(opts.Address, web.New(s))
}

// resolveOptions merges config file values with CLI-provided values.
func (c *WebCmd) resolveOptions(configPath string) (*WebOptions, error) {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return nil, err
	}

	database, err := databasePath(c.Database, cfg)
	if err != nil {
		return nil, err
	}

	return &WebOptions{Address: c.address(), Database: database}, nil
}

func (c *WebCmd) address() string {
	if c.Address == "" {
		return defaultWebAddress
	}
	return c.Address
}
