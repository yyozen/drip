package cli

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"drip/internal/client/tcp"
	"drip/internal/shared/protocol"
	"drip/pkg/config"

	"github.com/spf13/cobra"
)

var (
	httpsSubdomain    string
	httpsDaemonMode   bool
	httpsDaemonMarker bool
	httpsLocalAddress string
)

var httpsCmd = &cobra.Command{
	Use:   "https <port>",
	Short: "Start HTTPS tunnel",
	Long: `Start an HTTPS tunnel to expose a local HTTPS server.

Example:
  drip https 443                    Tunnel localhost:443
  drip https 8443 --subdomain myapp Use custom subdomain

Configuration:
  First time: Run 'drip config init' to save server and token
  Subsequent: Just run 'drip https <port>'

Note: Uses TCP over TLS 1.3 for secure communication`,
	Args: cobra.ExactArgs(1),
	RunE: runHTTPS,
}

func init() {
	httpsCmd.Flags().StringVarP(&httpsSubdomain, "subdomain", "n", "", "Custom subdomain (optional)")
	httpsCmd.Flags().BoolVarP(&httpsDaemonMode, "daemon", "d", false, "Run in background (daemon mode)")
	httpsCmd.Flags().StringVarP(&httpsLocalAddress, "address", "a", "127.0.0.1", "Local address to forward to (default: 127.0.0.1)")
	httpsCmd.Flags().BoolVar(&httpsDaemonMarker, "daemon-child", false, "Internal flag for daemon child process")
	httpsCmd.Flags().MarkHidden("daemon-child")
	rootCmd.AddCommand(httpsCmd)
}

func runHTTPS(cmd *cobra.Command, args []string) error {
	port, err := strconv.Atoi(args[0])
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid port number: %s", args[0])
	}

	if httpsDaemonMode && !httpsDaemonMarker {
		daemonArgs := append([]string{"https"}, args...)
		daemonArgs = append(daemonArgs, "--daemon-child")
		if httpsSubdomain != "" {
			daemonArgs = append(daemonArgs, "--subdomain", httpsSubdomain)
		}
		if httpsLocalAddress != "127.0.0.1" {
			daemonArgs = append(daemonArgs, "--address", httpsLocalAddress)
		}
		if serverURL != "" {
			daemonArgs = append(daemonArgs, "--server", serverURL)
		}
		if authToken != "" {
			daemonArgs = append(daemonArgs, "--token", authToken)
		}
		if insecure {
			daemonArgs = append(daemonArgs, "--insecure")
		}
		if verbose {
			daemonArgs = append(daemonArgs, "--verbose")
		}
		return StartDaemon("https", port, daemonArgs)
	}

	var serverAddr, token string

	if serverURL == "" {
		cfg, err := config.LoadClientConfig("")
		if err != nil {
			return fmt.Errorf(`configuration not found.

Please run 'drip config init' first, or use flags:
  drip https %d --server SERVER:PORT --token TOKEN`, port)
		}
		serverAddr = cfg.Server
		token = cfg.Token
	} else {
		serverAddr = serverURL
		token = authToken
	}

	if serverAddr == "" {
		return fmt.Errorf("server address is required")
	}

	connConfig := &tcp.ConnectorConfig{
		ServerAddr: serverAddr,
		Token:      token,
		TunnelType: protocol.TunnelTypeHTTPS,
		LocalHost:  httpsLocalAddress,
		LocalPort:  port,
		Subdomain:  httpsSubdomain,
		Insecure:   insecure,
	}

	var daemon *DaemonInfo
	if httpsDaemonMarker {
		daemon = &DaemonInfo{
			PID:        os.Getpid(),
			Type:       "https",
			Port:       port,
			Subdomain:  httpsSubdomain,
			Server:     serverAddr,
			StartTime:  time.Now(),
			Executable: os.Args[0],
		}
	}

	return runTunnelWithUI(connConfig, daemon)
}
