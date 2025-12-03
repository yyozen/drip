package cli

import (
	"fmt"

	"drip/internal/client/cli/ui"
	"github.com/spf13/cobra"
)

var (
	// Version information
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"

	// Global flags
	serverURL string
	authToken string
	verbose   bool
	insecure  bool
)

var rootCmd = &cobra.Command{
	Use:   "drip",
	Short: "Drip - Fast and secure tunnels to localhost",
	Long: `Drip - High-performance tunneling service with TCP over TLS 1.3

Expose your local services to the internet securely and easily.

Configuration:
  First time: Run 'drip config init' to set up server and token
  Subsequent: Just run 'drip http <port>' or 'drip tcp <port>'

Examples:
  drip config init                  # Set up configuration
  drip http 3000                    # HTTP tunnel
  drip tcp 5432                     # PostgreSQL tunnel
  drip http 8080 --subdomain myapp  # Custom subdomain

Features:
  ✓ TCP over TLS 1.3 (secure and fast)
  ✓ HTTP and TCP tunnel support
  ✓ Auto-save configuration
  ✓ Custom subdomains
  ✓ Authentication via token`,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&serverURL, "server", "s", "", "Server address (e.g., tunnel.example.com:443)")
	rootCmd.PersistentFlags().StringVarP(&authToken, "token", "t", "", "Authentication token")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVarP(&insecure, "insecure", "k", false, "Skip TLS verification (testing only, NOT recommended)")

	rootCmd.AddCommand(versionCmd)
	// http and tcp commands are added in their respective init() functions
	// config command is added in config.go init() function
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(ui.Info(
			"Drip Client",
			"",
			ui.KeyValue("Version", Version),
			ui.KeyValue("Git Commit", GitCommit),
			ui.KeyValue("Build Time", BuildTime),
		))
	},
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

// SetVersion sets the version information
func SetVersion(version, commit, buildTime string) {
	Version = version
	GitCommit = commit
	BuildTime = buildTime
}
