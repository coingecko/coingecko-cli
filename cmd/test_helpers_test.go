package cmd

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/coingecko/coingecko-cli/internal/api"
	"github.com/coingecko/coingecko-cli/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// withTestClient overrides newAPIClient so that command handlers use a client
// pointed at the given httptest.Server. Restored via t.Cleanup.
func withTestClient(t *testing.T, srv *httptest.Server, tier string) {
	t.Helper()

	// Override loadConfig to return a test config without touching real config files.
	origLoad := loadConfig
	loadConfig = func() (*config.Config, error) {
		return &config.Config{APIKey: "test-key", Tier: tier}, nil
	}
	t.Cleanup(func() { loadConfig = origLoad })

	// Override newAPIClient to point at the test server.
	origClient := newAPIClient
	newAPIClient = func(cfg *config.Config) *api.Client {
		c := api.NewClient(cfg)
		c.SetBaseURL(srv.URL)
		return c
	}
	t.Cleanup(func() { newAPIClient = origClient })
}

// withTestClientDemo is shorthand for withTestClient with demo tier.
func withTestClientDemo(t *testing.T, srv *httptest.Server) {
	t.Helper()
	withTestClient(t, srv, "demo")
}

// withTestClientPaid is shorthand for withTestClient with paid tier.
func withTestClientPaid(t *testing.T, srv *httptest.Server) {
	t.Helper()
	withTestClient(t, srv, "paid")
}

// executeCommand runs rootCmd with the given args and captures stdout/stderr.
// It resets rootCmd args via t.Cleanup.
func executeCommand(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	// Capture stdout.
	oldStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	// Capture stderr.
	oldStderr := os.Stderr
	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr

	// Reset all flags to defaults before each run to prevent state leakage.
	resetAllFlags(rootCmd)

	rootCmd.SetArgs(args)
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		rootCmd.SilenceUsage = false
		rootCmd.SilenceErrors = false
	})

	// Drain pipes concurrently to avoid deadlock if output exceeds pipe buffer.
	var bufOut, bufErr bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, _ = io.Copy(&bufOut, rOut) }()
	go func() { defer wg.Done(); _, _ = io.Copy(&bufErr, rErr) }()

	// Run the command — Cobra writes to os.Stdout/os.Stderr.
	cmdErr := rootCmd.Execute()

	// Close write ends so readers finish, then wait.
	_ = wOut.Close()
	_ = wErr.Close()
	wg.Wait()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	return bufOut.String(), bufErr.String(), cmdErr
}

// newTestServer creates an httptest.Server from a handler func.
func newTestServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

// captureStdout runs fn and returns everything it wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

// resetAllFlags resets all flags on a command and its children to their default values.
func resetAllFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
	for _, c := range cmd.Commands() {
		resetAllFlags(c)
	}
}
