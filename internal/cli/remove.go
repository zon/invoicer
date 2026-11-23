package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/zon/invoicer/internal/store"
)

// RemoveCmd deletes a stored invoice.
type RemoveCmd struct {
	// ID is the identifier of the invoice to remove.
	ID string `arg:"" help:"Identifier of the invoice to remove."`

	// Database is the path to the invoice database.
	Database string `help:"Path to the invoice database. Defaults to ~/.invoicer/invoices.db."`
}

// Run executes the remove subcommand.
func (c *RemoveCmd) Run() error {
	return c.run("", os.Stdout)
}

// run executes remove with an explicit config path and output writer.
func (c *RemoveCmd) run(configPath string, out io.Writer) error {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return err
	}

	database, err := databasePath(c.Database, cfg)
	if err != nil {
		return err
	}

	s, err := store.Open(database)
	if err != nil {
		return fmt.Errorf("opening invoice database: %w", err)
	}
	defer s.Close()

	inv, err := s.Remove(c.ID)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Removed invoice for %s %d: %s\n", inv.Month.String(), inv.Year, strings.ToUpper(inv.ID))
	return nil
}
