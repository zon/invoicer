package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zon/invoicer/internal/access"
	"github.com/zon/invoicer/internal/config"
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

	// Access is nil when the config omits every access setting, which runs the
	// web app without sign-in for local development.
	Access *access.Options
}

// Run executes the web subcommand.
func (c *WebCmd) Run() error {
	return c.run("", os.Stdout)
}

// listenAndServe starts the HTTP server. Tests replace it to inspect the
// address and handler without binding a port.
var listenAndServe = func(server *http.Server) error {
	return server.ListenAndServe()
}

// notifyContext returns a context cancelled by an interrupt. Tests replace it
// to trigger the shutdown without sending a signal.
var notifyContext = signal.NotifyContext

// shutdownTimeout limits how long an interrupt waits for requests to finish.
const shutdownTimeout = 10 * time.Second

// run executes web with an explicit config path and output writer.
func (c *WebCmd) run(configPath string, out io.Writer) error {
	opts, err := c.resolveOptions(configPath)
	if err != nil {
		return err
	}

	ctx, stop := notifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	auth, err := accessFor(ctx, opts.Access)
	if err != nil {
		return err
	}

	s, err := store.Open(opts.Database)
	if err != nil {
		return fmt.Errorf("opening invoice database: %w", err)
	}
	defer s.Close()

	handler := web.New(s)
	if auth != nil {
		handler = web.New(s, auth)
	}

	fmt.Fprintf(out, "invoicer web ready at %s\n", localURL(opts.Address))
	return serve(ctx, &http.Server{Addr: opts.Address, Handler: handler})
}

// accessFor discovers the provider named by the access settings. It returns nil
// when the config omits access settings.
func accessFor(ctx context.Context, opts *access.Options) (*access.Access, error) {
	if opts == nil {
		return nil, nil
	}
	return access.New(ctx, *opts)
}

// localURL returns the URL for a listen address, using localhost for
// addresses that listen on every interface.
func localURL(address string) string {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "http://" + address + "/"
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port) + "/"
}

// serve runs the server until ctx is cancelled, then shuts it down gracefully.
func serve(ctx context.Context, server *http.Server) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- listenAndServe(server)
	}()

	select {
	case err := <-errCh:
		return serverError(err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutting down web server: %w", err)
	}
	return serverError(<-errCh)
}

func serverError(err error) error {
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return fmt.Errorf("starting web server: %w", err)
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

	accessOptions, err := resolveAccessOptions(cfg)
	if err != nil {
		return nil, err
	}

	return &WebOptions{Address: c.address(), Database: database, Access: accessOptions}, nil
}

// resolveAccessOptions reads the web app's access settings and the secrets
// they name. It returns nil when the config omits every access setting.
func resolveAccessOptions(cfg *config.Config) (*access.Options, error) {
	if cfg.OIDC == nil && cfg.BaseURL == "" {
		return nil, nil
	}
	if cfg.OIDC == nil {
		return nil, errors.New("oidc settings are required in the config file")
	}

	opts := &access.Options{
		BaseURL:       cfg.BaseURL,
		Issuer:        cfg.OIDC.Issuer,
		ClientID:      cfg.OIDC.ClientID,
		AdminGroup:    cfg.OIDC.AdminGroup,
		CustomerGroup: cfg.OIDC.CustomerGroup,
		GroupsClaim:   cfg.OIDC.GroupsClaim,
	}

	secret, err := readAccessFile("oidc.clientSecretFile", cfg.OIDC.ClientSecretFile)
	if err != nil {
		return nil, err
	}
	if len(secret) == 0 {
		return nil, fmt.Errorf("oidc.clientSecretFile is empty: %q", cfg.OIDC.ClientSecretFile)
	}
	opts.ClientSecret = string(secret)

	key, err := readAccessFile("oidc.cookieKeyFile", cfg.OIDC.CookieKeyFile)
	if err != nil {
		return nil, err
	}
	if len(key) < 32 {
		return nil, fmt.Errorf("oidc.cookieKeyFile must hold at least 32 bytes: %q", cfg.OIDC.CookieKeyFile)
	}
	opts.CookieKey = key

	if err := opts.Validate(); err != nil {
		return nil, err
	}
	return opts, nil
}

// readAccessFile reads a file named by an access setting.
func readAccessFile(setting, path string) ([]byte, error) {
	if path == "" {
		return nil, fmt.Errorf("%s is required", setting)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s %q: %w", setting, path, err)
	}
	return data, nil
}

func (c *WebCmd) address() string {
	if c.Address == "" {
		return defaultWebAddress
	}
	return c.Address
}
