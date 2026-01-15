package cli

import (
	"fmt"
	"os"

	"drip/internal/shared/constants"
	"github.com/spf13/cobra"
)

var (
	serverConfigFull bool
)

var serverConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage server configuration",
	Long:  "Display and manage Drip server configuration",
}

var serverConfigShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show server configuration",
	Long:  "Display the current Drip server configuration from environment variables and flags",
	RunE:  runServerConfigShow,
}

func init() {
	serverConfigCmd.AddCommand(serverConfigShowCmd)
	serverConfigShowCmd.Flags().BoolVar(&serverConfigFull, "full", false, "Show full tokens (not masked)")

	serverCmd.AddCommand(serverConfigCmd)
}

func runServerConfigShow(_ *cobra.Command, _ []string) error {
	// Read configuration from environment variables and defaults
	port := getEnvInt("DRIP_PORT", 8443)
	publicPort := getEnvInt("DRIP_PUBLIC_PORT", 0)
	domain := getEnvString("DRIP_DOMAIN", constants.DefaultDomain)
	token := getEnvString("DRIP_TOKEN", "")
	metricsToken := getEnvString("DRIP_METRICS_TOKEN", "")
	tlsCert := getEnvString("DRIP_TLS_CERT", "")
	tlsKey := getEnvString("DRIP_TLS_KEY", "")
	tcpPortMin := getEnvInt("DRIP_TCP_PORT_MIN", constants.DefaultTCPPortMin)
	tcpPortMax := getEnvInt("DRIP_TCP_PORT_MAX", constants.DefaultTCPPortMax)
	pprofPort := getEnvInt("DRIP_PPROF_PORT", 0)

	if publicPort == 0 {
		publicPort = port
	}

	// Mask tokens if not showing full
	displayToken := maskToken(token, serverConfigFull)
	displayMetricsToken := maskToken(metricsToken, serverConfigFull)

	// Print configuration
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║              Drip Server Configuration                      ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Server settings
	fmt.Println("📡 Server Settings:")
	fmt.Printf("  Domain:           %s\n", colorValue(domain))
	fmt.Printf("  Port:             %s\n", colorValue(fmt.Sprintf("%d", port)))
	if publicPort != port {
		fmt.Printf("  Public Port:      %s\n", colorValue(fmt.Sprintf("%d", publicPort)))
	}
	fmt.Println()

	// Authentication
	fmt.Println("🔐 Authentication:")
	if token != "" {
		fmt.Printf("  Auth Token:       %s\n", colorValue(displayToken))
	} else {
		fmt.Printf("  Auth Token:       %s\n", colorWarning("(not set)"))
	}
	if metricsToken != "" {
		fmt.Printf("  Metrics Token:    %s\n", colorValue(displayMetricsToken))
	} else {
		fmt.Printf("  Metrics Token:    %s\n", colorWarning("(not set)"))
	}
	fmt.Println()

	// TLS Configuration
	fmt.Println("🔒 TLS Configuration:")
	if tlsCert != "" {
		certStatus := checkFileExists(tlsCert)
		fmt.Printf("  Certificate:      %s %s\n", colorValue(tlsCert), certStatus)
	} else {
		fmt.Printf("  Certificate:      %s\n", colorError("(not set)"))
	}
	if tlsKey != "" {
		keyStatus := checkFileExists(tlsKey)
		fmt.Printf("  Private Key:      %s %s\n", colorValue(tlsKey), keyStatus)
	} else {
		fmt.Printf("  Private Key:      %s\n", colorError("(not set)"))
	}
	fmt.Println()

	// TCP Tunnel Ports
	fmt.Println("🌐 TCP Tunnel Port Range:")
	fmt.Printf("  Min Port:         %s\n", colorValue(fmt.Sprintf("%d", tcpPortMin)))
	fmt.Printf("  Max Port:         %s\n", colorValue(fmt.Sprintf("%d", tcpPortMax)))
	fmt.Printf("  Available Ports:  %s\n", colorValue(fmt.Sprintf("%d", tcpPortMax-tcpPortMin+1)))
	fmt.Println()

	// Performance
	if pprofPort > 0 {
		fmt.Println("⚡ Performance:")
		fmt.Printf("  Pprof Port:       %s\n", colorValue(fmt.Sprintf("%d", pprofPort)))
		fmt.Println()
	}

	// Configuration sources
	fmt.Println("Configuration Sources:")
	fmt.Println("  Command-line flags (highest priority)")
	fmt.Println("  Environment variables (DRIP_*)")
	fmt.Println("  Config file: /etc/drip/config.yaml or ~/.drip/server.yaml")
	fmt.Println()

	// Endpoints
	fmt.Println("🔗 Server Endpoints:")
	fmt.Printf("  Main:             https://%s:%d\n", domain, publicPort)
	fmt.Printf("  Health:           https://%s:%d/health\n", domain, publicPort)
	fmt.Printf("  Stats:            https://%s:%d/stats\n", domain, publicPort)
	fmt.Printf("  Metrics:          https://%s:%d/metrics\n", domain, publicPort)
	fmt.Println()

	if !serverConfigFull && (token != "" || metricsToken != "") {
		fmt.Println("💡 Tip: Use --full flag to show complete tokens")
		fmt.Println()
	}

	return nil
}

func maskToken(token string, showFull bool) string {
	if token == "" {
		return ""
	}
	if showFull {
		return token
	}

	tokenLen := len(token)
	if tokenLen <= 8 {
		return "***"
	}
	// Show first 4 and last 4 characters
	return token[:4] + "***" + token[tokenLen-4:]
}

func checkFileExists(path string) string {
	if _, err := os.Stat(path); err == nil {
		return colorSuccess("✓")
	}
	return colorError("✗ (not found)")
}

func colorValue(s string) string {
	return fmt.Sprintf("\033[36m%s\033[0m", s) // Cyan
}

func colorSuccess(s string) string {
	return fmt.Sprintf("\033[32m%s\033[0m", s) // Green
}

func colorWarning(s string) string {
	return fmt.Sprintf("\033[33m%s\033[0m", s) // Yellow
}

func colorError(s string) string {
	return fmt.Sprintf("\033[31m%s\033[0m", s) // Red
}
