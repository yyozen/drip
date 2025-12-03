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

const (
	maxReconnectAttempts = 5
	reconnectInterval    = 3 * time.Second
)

var (
	subdomain    string
	daemonMode   bool
	daemonMarker bool
	localAddress string
)

var httpCmd = &cobra.Command{
	Use:   "http <port>",
	Short: "Start HTTP tunnel",
	Long: `Start an HTTP tunnel to expose a local HTTP server.

Example:
  drip http 3000                    Tunnel localhost:3000
  drip http 8080 --subdomain myapp  Use custom subdomain

Configuration:
  First time: Run 'drip config init' to save server and token
  Subsequent: Just run 'drip http <port>'

Note: Uses TCP over TLS 1.3 for secure communication`,
	Args: cobra.ExactArgs(1),
	RunE: runHTTP,
}

func init() {
	httpCmd.Flags().StringVarP(&subdomain, "subdomain", "n", "", "Custom subdomain (optional)")
	httpCmd.Flags().BoolVarP(&daemonMode, "daemon", "d", false, "Run in background (daemon mode)")
	httpCmd.Flags().StringVarP(&localAddress, "address", "a", "127.0.0.1", "Local address to forward to (default: 127.0.0.1)")
	httpCmd.Flags().BoolVar(&daemonMarker, "daemon-child", false, "Internal flag for daemon child process")
	httpCmd.Flags().MarkHidden("daemon-child")
	rootCmd.AddCommand(httpCmd)
}

func runHTTP(cmd *cobra.Command, args []string) error {
	port, err := strconv.Atoi(args[0])
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid port number: %s", args[0])
	}

	if daemonMode && !daemonMarker {
		daemonArgs := append([]string{"http"}, args...)
		daemonArgs = append(daemonArgs, "--daemon-child")
		if subdomain != "" {
			daemonArgs = append(daemonArgs, "--subdomain", subdomain)
		}
		if localAddress != "127.0.0.1" {
			daemonArgs = append(daemonArgs, "--address", localAddress)
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
		return StartDaemon("http", port, daemonArgs)
	}

	var serverAddr, token string

	if serverURL == "" {
		cfg, err := config.LoadClientConfig("")
		if err != nil {
			return fmt.Errorf(`configuration not found.

Please run 'drip config init' first, or use flags:
  drip http %d --server SERVER:PORT --token TOKEN`, port)
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
		TunnelType: protocol.TunnelTypeHTTP,
		LocalHost:  localAddress,
		LocalPort:  port,
		Subdomain:  subdomain,
		Insecure:   insecure,
	}

	var daemon *DaemonInfo
	if daemonMarker {
		daemon = &DaemonInfo{
			PID:        os.Getpid(),
			Type:       "http",
			Port:       port,
			Subdomain:  subdomain,
			Server:     serverAddr,
			StartTime:  time.Now(),
			Executable: os.Args[0],
		}
	}

	return runTunnelWithUI(connConfig, daemon)
}
