package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

const (
	tunnelCardWidth  = 76
	statsColumnWidth = 32
)

var (
	latencyFastColor   = lipgloss.Color("#22c55e") // green
	latencyYellowColor = lipgloss.Color("#eab308") // yellow
	latencyOrangeColor = lipgloss.Color("#f97316") // orange
	latencyRedColor    = lipgloss.Color("#ef4444") // red
)

// TunnelStatus represents the status of a tunnel
type TunnelStatus struct {
	Type         string        // "http", "https", "tcp"
	URL          string        // Public URL
	LocalAddr    string        // Local address
	Latency      time.Duration // Current latency
	BytesIn      int64         // Bytes received
	BytesOut     int64         // Bytes sent
	SpeedIn      float64       // Download speed
	SpeedOut     float64       // Upload speed
	TotalRequest int64         // Total requests
}

// RenderTunnelConnected renders the tunnel connection card
func RenderTunnelConnected(status *TunnelStatus) string {
	icon, typeStr, accent := tunnelVisuals(status.Type)

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(1, 2).
		Width(tunnelCardWidth)

	typeBadge := lipgloss.NewStyle().
		Background(accent).
		Foreground(lipgloss.Color("#f8fafc")).
		Bold(true).
		Padding(0, 1).
		Render(strings.ToUpper(typeStr) + " TUNNEL")

	headline := lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.NewStyle().Foreground(accent).Render(icon),
		lipgloss.NewStyle().Bold(true).MarginLeft(1).Render("Tunnel Connected"),
		lipgloss.NewStyle().MarginLeft(2).Render(typeBadge),
	)

	urlLine := lipgloss.JoinHorizontal(
		lipgloss.Left,
		urlStyle.Copy().Foreground(accent).Render(status.URL),
		lipgloss.NewStyle().MarginLeft(1).Foreground(mutedColor).Render("(forwarded link)"),
	)

	forwardLine := lipgloss.NewStyle().
		MarginLeft(2).
		Render(Muted("⇢ ") + valueStyle.Render(status.LocalAddr))

	hint := lipgloss.NewStyle().
		Foreground(latencyOrangeColor).
		Render("Ctrl+C to stop • reconnects automatically")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		headline,
		"",
		urlLine,
		forwardLine,
		"",
		hint,
	)

	return "\n" + card.Render(content) + "\n"
}

// RenderTunnelStats renders real-time tunnel statistics in a card
func RenderTunnelStats(status *TunnelStatus) string {
	latencyStr := formatLatency(status.Latency)
	trafficStr := fmt.Sprintf("↓ %s  ↑ %s", formatBytes(status.BytesIn), formatBytes(status.BytesOut))
	speedStr := fmt.Sprintf("↓ %s  ↑ %s", formatSpeed(status.SpeedIn), formatSpeed(status.SpeedOut))
	requestsStr := fmt.Sprintf("%d", status.TotalRequest)

	_, _, accent := tunnelVisuals(status.Type)

	header := lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.NewStyle().Foreground(accent).Render("◉"),
		lipgloss.NewStyle().Bold(true).MarginLeft(1).Render("Live Metrics"),
	)

	row1 := lipgloss.JoinHorizontal(
		lipgloss.Top,
		statColumn("Latency", latencyStr, statsColumnWidth),
		statColumn("Requests", highlightStyle.Render(requestsStr), statsColumnWidth),
	)

	row2 := lipgloss.JoinHorizontal(
		lipgloss.Top,
		statColumn("Traffic", Cyan(trafficStr), statsColumnWidth),
		statColumn("Speed", warningStyle.Render(speedStr), statsColumnWidth),
	)

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(1, 2).
		Width(tunnelCardWidth)

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		row1,
		row2,
	)

	return "\n" + card.Render(body) + "\n"
}

// RenderConnecting renders the connecting message
func RenderConnecting(serverAddr string, attempt int, maxAttempts int) string {
	if attempt == 0 {
		return Highlight("◌") + " Connecting to " + Muted(serverAddr) + "..."
	}
	return Warning(fmt.Sprintf("◌ Reconnecting to %s (attempt %d/%d)...", serverAddr, attempt, maxAttempts))
}

// RenderConnectionFailed renders connection failure message
func RenderConnectionFailed(err error) string {
	return Error(fmt.Sprintf("Connection failed: %v", err))
}

// RenderShuttingDown renders shutdown message
func RenderShuttingDown() string {
	return Warning("⏹  Shutting down...")
}

// RenderConnectionLost renders connection lost message
func RenderConnectionLost() string {
	return Error("⚠  Connection lost!")
}

// RenderRetrying renders retry message
func RenderRetrying(interval time.Duration) string {
	return Muted(fmt.Sprintf("  Retrying in %v...", interval))
}

// formatLatency formats latency with color
func formatLatency(d time.Duration) string {
	ms := d.Milliseconds()
	var style lipgloss.Style

	if ms == 0 {
		return mutedStyle.Render("measuring...")
	}

	switch {
	case ms < 50:
		style = lipgloss.NewStyle().Foreground(latencyFastColor)
	case ms < 150:
		style = lipgloss.NewStyle().Foreground(latencyYellowColor)
	case ms < 300:
		style = lipgloss.NewStyle().Foreground(latencyOrangeColor)
	default:
		style = lipgloss.NewStyle().Foreground(latencyRedColor)
	}

	return style.Render(fmt.Sprintf("%dms", ms))
}

// formatBytes formats bytes to human readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatSpeed formats speed to human readable format
func formatSpeed(bytesPerSec float64) string {
	const unit = 1024.0
	if bytesPerSec < unit {
		return fmt.Sprintf("%.0f B/s", bytesPerSec)
	}
	div, exp := unit, 0
	for n := bytesPerSec / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB/s", bytesPerSec/div, "KMGTPE"[exp])
}

func statColumn(label, value string, width int) string {
	labelView := lipgloss.NewStyle().
		Foreground(mutedColor).
		Render(strings.ToUpper(label))

	block := lipgloss.JoinHorizontal(
		lipgloss.Left,
		labelView,
		lipgloss.NewStyle().MarginLeft(1).Render(value),
	)

	if width <= 0 {
		return block
	}

	return lipgloss.NewStyle().
		Width(width).
		Render(block)
}

func tunnelVisuals(tunnelType string) (string, string, lipgloss.Color) {
	switch tunnelType {
	case "http":
		return "🚀", "HTTP", lipgloss.Color("#0070F3")
	case "https":
		return "🔒", "HTTPS", lipgloss.Color("#2D8CFF")
	case "tcp":
		return "🔌", "TCP", lipgloss.Color("#50E3C2")
	default:
		return "🌐", strings.ToUpper(tunnelType), lipgloss.Color("#0070F3")
	}
}
