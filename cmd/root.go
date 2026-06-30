// Package cmd implements the `nyt` command-line interface wrapping the New York
// Times developer APIs. Each API lives in its own file and registers itself on
// rootCmd via init(), so adding an API never requires editing this file.
package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"gitea.jonn.me/jons-org/nyt_cli/internal/client"
	"gitea.jonn.me/jons-org/nyt_cli/internal/config"
	"github.com/spf13/cobra"
)

// Persistent flag values, populated by cobra before any command runs.
var (
	flagAPIKey   string
	flagJSON     bool
	flagVerbose  bool
	flagTimeout  time.Duration
	flagThrottle time.Duration
	flagRetries  int
	flagBaseURL  string
)

var cachedClient *client.Client

var rootCmd = &cobra.Command{
	Use:   "nyt",
	Short: "A command-line wrapper for the New York Times APIs",
	Long: `nyt wraps the New York Times developer APIs (https://developer.nytimes.com).

It covers Top Stories, Article Search, the Archive, Most Popular, the Books
best-seller lists, and the public RSS feeds.

Authentication:
  Every request needs your NYT app "Key", supplied (in priority order) via:
    --api-key flag, $NYT_API_KEY or $NYT_KEY, a .env file, or
    ~/.config/nyt/config.json (see "nyt config").

Output:
  Human-readable tables by default; pass --json for the raw NYT response.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command with a signal-aware context.
func Execute() {
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	watchSignals(stop)

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&flagAPIKey, "api-key", "", "NYT API key (overrides env/config)")
	pf.BoolVar(&flagJSON, "json", false, "output the raw NYT JSON response")
	pf.BoolVarP(&flagVerbose, "verbose", "v", false, "log requests to stderr (API key redacted)")
	pf.DurationVar(&flagTimeout, "timeout", 30*time.Second, "per-request timeout")
	pf.DurationVar(&flagThrottle, "throttle", 0, "minimum delay between requests (e.g. 6s to respect rate limits)")
	pf.IntVar(&flagRetries, "retries", 3, "retries on 429/5xx/transport errors")
	pf.StringVar(&flagBaseURL, "base-url", client.DefaultBaseURL, "API base URL")
	_ = pf.MarkHidden("base-url")
}

// apiClient lazily builds (and caches) an authenticated client, returning a
// clear error if no API key is configured. Commands that hit the keyed NYT APIs
// call this; commands like `version` and `config` do not, so they work without
// a key.
func apiClient() (*client.Client, error) { return buildClient(true) }

// rssClient builds a client for the public RSS feeds, which need no API key. It
// reuses the key when one is present (harmless) but does not require it.
func rssClient() (*client.Client, error) { return buildClient(false) }

func buildClient(requireKey bool) (*client.Client, error) {
	if cachedClient != nil {
		return cachedClient, nil
	}
	key, source, err := config.ResolveAPIKey(flagAPIKey)
	if err != nil {
		if requireKey {
			return nil, err
		}
		key, source = "", "none (RSS feeds need no key)"
	}
	if flagVerbose {
		fmt.Fprintf(os.Stderr, "nyt: using API key from %s\n", source)
	}
	cachedClient = client.New(key,
		client.WithBaseURL(flagBaseURL),
		client.WithTimeout(flagTimeout),
		client.WithMinInterval(flagThrottle),
		client.WithMaxRetries(flagRetries),
		client.WithVerbose(flagVerbose),
	)
	return cachedClient, nil
}

// jsonOutput reports whether --json was set.
func jsonOutput() bool { return flagJSON }

// ctxOf returns the command's context (signal-aware, set by Execute).
func ctxOf(cmd *cobra.Command) context.Context {
	if c := cmd.Context(); c != nil {
		return c
	}
	return context.Background()
}
