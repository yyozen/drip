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

var tcpCmd = &cobra.Command{
	Use:   "tcp <port>",
	Short: "Start TCP tunnel",
	Long: `Start a TCP tunnel to expose any TCP service.

Example:
  drip tcp 5432                     Tunnel PostgreSQL
  drip tcp 3306                     Tunnel MySQL
  drip tcp 22                       Tunnel SSH
  drip tcp 6379 --subdomain myredis Tunnel Redis with custom subdomain

Supported Services:
  - Databases: PostgreSQL (5432), MySQL (3306), Redis (6379), MongoDB (27017)
  - SSH: Port 22
  - Any TCP service

Configuration:
  First time: Run 'drip config init' to save server and token
  Subsequent: Just run 'drip tcp <port>'

Note: Uses TCP over TLS 1.3 for secure communication`,
	Args: cobra.ExactArgs(1),
	RunE: runTCP,
}

func init() {
	tcpCmd.Flags().StringVarP(&subdomain, "subdomain", "n", "", "Custom subdomain (optional)")
	tcpCmd.Flags().BoolVarP(&daemonMode, "daemon", "d", false, "Run in background (daemon mode)")
	tcpCmd.Flags().StringVarP(&localAddress, "address", "a", "127.0.0.1", "Local address to forward to (default: 127.0.0.1)")
	tcpCmd.Flags().BoolVar(&daemonMarker, "daemon-child", false, "Internal flag for daemon child process")
	tcpCmd.Flags().MarkHidden("daemon-child")
	rootCmd.AddCommand(tcpCmd)
}

func runTCP(cmd *cobra.Command, args []string) error {
	port, err := strconv.Atoi(args[0])
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid port number: %s", args[0])
	}

	if daemonMode && !daemonMarker {
		daemonArgs := append([]string{"tcp"}, args...)
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
		return StartDaemon("tcp", port, daemonArgs)
	}

	var serverAddr, token string

	if serverURL == "" {
		cfg, err := config.LoadClientConfig("")
		if err != nil {
			return fmt.Errorf(`configuration not found.

Please run 'drip config init' first, or use flags:
  drip tcp %d --server SERVER:PORT --token TOKEN`, port)
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
		TunnelType: protocol.TunnelTypeTCP,
		LocalHost:  localAddress,
		LocalPort:  port,
		Subdomain:  subdomain,
		Insecure:   insecure,
	}

	var daemon *DaemonInfo
	if daemonMarker {
		daemon = &DaemonInfo{
			PID:        os.Getpid(),
			Type:       "tcp",
			Port:       port,
			Subdomain:  subdomain,
			Server:     serverAddr,
			StartTime:  time.Now(),
			Executable: os.Args[0],
		}
	}

	return runTunnelWithUI(connConfig, daemon)
}
