package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"drip/internal/client/cli/ui"
	"github.com/spf13/cobra"
)

var attachCmd = &cobra.Command{
	Use:   "attach [type] [port]",
	Short: "Attach to a running background tunnel",
	Long: `Attach to a running background tunnel to view its logs in real-time.

Examples:
  drip attach              List running tunnels and select one
  drip attach http 3000    Attach to HTTP tunnel on port 3000
  drip attach tcp 5432     Attach to TCP tunnel on port 5432

Press Ctrl+C to detach (tunnel will continue running).`,
	Aliases: []string{"logs", "tail"},
	Args:    cobra.MaximumNArgs(2),
	RunE:    runAttach,
}

func init() {
	rootCmd.AddCommand(attachCmd)
}

func runAttach(cmd *cobra.Command, args []string) error {
	CleanupStaleDaemons()

	daemons, err := ListAllDaemons()
	if err != nil {
		return fmt.Errorf("failed to list daemons: %w", err)
	}

	if len(daemons) == 0 {
		fmt.Println(ui.Info(
			"No Running Tunnels",
			"",
			ui.Muted("Start a tunnel in background with:"),
			ui.Cyan("  drip http 3000 -d"),
			ui.Cyan("  drip tcp 5432 -d"),
		))
		return nil
	}

	var selectedDaemon *DaemonInfo

	if len(args) == 2 {
		tunnelType := args[0]
		if tunnelType != "http" && tunnelType != "tcp" {
			return fmt.Errorf("invalid tunnel type: %s (must be 'http' or 'tcp')", tunnelType)
		}

		port, err := strconv.Atoi(args[1])
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid port number: %s", args[1])
		}

		for _, d := range daemons {
			if d.Type == tunnelType && d.Port == port {
				if !IsProcessRunning(d.PID) {
					RemoveDaemonInfo(d.Type, d.Port)
					return fmt.Errorf("tunnel is not running (cleaned up stale entry)")
				}
				selectedDaemon = d
				break
			}
		}

		if selectedDaemon == nil {
			return fmt.Errorf("no %s tunnel running on port %d", tunnelType, port)
		}
	} else if len(args) == 0 {
		selectedDaemon, err = selectDaemonInteractive(daemons)
		if err != nil {
			return err
		}
		if selectedDaemon == nil {
			return nil
		}
	} else {
		return fmt.Errorf("usage: drip attach [type port]")
	}

	return attachToDaemon(selectedDaemon)
}

func selectDaemonInteractive(daemons []*DaemonInfo) (*DaemonInfo, error) {
	var runningDaemons []*DaemonInfo
	for _, d := range daemons {
		if IsProcessRunning(d.PID) {
			runningDaemons = append(runningDaemons, d)
		} else {
			RemoveDaemonInfo(d.Type, d.Port)
		}
	}

	if len(runningDaemons) == 0 {
		fmt.Println(ui.Muted("No running tunnels."))
		return nil, nil
	}

	table := ui.NewTable([]string{"#", "TYPE", "PORT", "URL", "UPTIME"}).
		WithTitle("Select a tunnel to attach")

	for i, d := range runningDaemons {
		uptime := time.Since(d.StartTime)

		var typeStr string
		if d.Type == "http" {
			typeStr = ui.Success("HTTP")
		} else {
			typeStr = ui.Highlight("TCP")
		}

		table.AddRow([]string{
			ui.Highlight(fmt.Sprintf("%d", i+1)),
			typeStr,
			fmt.Sprintf("%d", d.Port),
			ui.URL(d.URL),
			FormatDuration(uptime),
		})
	}

	fmt.Print(table.Render())

	fmt.Printf("Enter number (1-%d) or 'q' to quit: ", len(runningDaemons))

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "q" || input == "Q" {
		return nil, nil
	}

	selection, err := strconv.Atoi(input)
	if err != nil || selection < 1 || selection > len(runningDaemons) {
		return nil, fmt.Errorf("invalid selection: %s", input)
	}

	return runningDaemons[selection-1], nil
}

func attachToDaemon(daemon *DaemonInfo) error {
	logPath := filepath.Join(getDaemonDir(), fmt.Sprintf("%s_%d.log", daemon.Type, daemon.Port))

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return fmt.Errorf("log file not found: %s", logPath)
	}

	uptime := time.Since(daemon.StartTime)

	fmt.Println(ui.Info(
		fmt.Sprintf("Attached to %s tunnel on port %d", strings.ToUpper(daemon.Type), daemon.Port),
		"",
		ui.KeyValue("URL", ui.URL(daemon.URL)),
		ui.KeyValue("PID", fmt.Sprintf("%d", daemon.PID)),
		ui.KeyValue("Uptime", FormatDuration(uptime)),
		ui.KeyValue("Log", truncatePath(logPath, 48)),
		"",
		ui.Warning("Press Ctrl+C to detach (tunnel will continue running)"),
	))

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	tailCmd := exec.Command("tail", "-f", logPath)
	tailCmd.Stdout = os.Stdout
	tailCmd.Stderr = os.Stderr

	if err := tailCmd.Start(); err != nil {
		return fmt.Errorf("failed to start tail: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- tailCmd.Wait()
	}()

	select {
	case <-sigCh:
		if tailCmd.Process != nil {
			tailCmd.Process.Kill()
		}
		fmt.Println()
		fmt.Println(ui.Warning("Detached from tunnel (tunnel is still running)"))
		fmt.Println(ui.Muted(fmt.Sprintf("Use '%s' to reattach", ui.Cyan(fmt.Sprintf("drip attach %s %d", daemon.Type, daemon.Port)))))
		fmt.Println(ui.Muted(fmt.Sprintf("Use '%s' to stop the tunnel", ui.Cyan(fmt.Sprintf("drip stop %s %d", daemon.Type, daemon.Port)))))
		return nil
	case err := <-done:
		if err != nil {
			return fmt.Errorf("tail process exited: %w", err)
		}
		return nil
	}
}

func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	filename := filepath.Base(path)
	if len(filename) >= maxLen-3 {
		return "..." + filename[len(filename)-(maxLen-3):]
	}
	dirLen := maxLen - len(filename) - 3
	return path[:dirLen] + "..." + filename
}
