package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"drip/internal/client/cli/ui"
	"github.com/spf13/cobra"
)

var (
	interactiveMode bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all running background tunnels",
	Long: `List all running background tunnels.

Example:
  drip list                     Show all running tunnels
  drip list -i                  Interactive mode (select to attach/stop)

This command shows:
  - Tunnel type (HTTP/TCP)
  - Local port being tunneled
  - Public URL
  - Process ID (PID)
  - Uptime

In interactive mode, you can select a tunnel to:
  - Attach: View real-time logs
  - Stop: Terminate the tunnel`,
	Aliases: []string{"ls", "ps", "status"},
	RunE:    runList,
}

func init() {
	listCmd.Flags().BoolVarP(&interactiveMode, "interactive", "i", false, "Interactive mode for attach/stop")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	CleanupStaleDaemons()

	daemons, err := ListAllDaemons()
	if err != nil {
		return fmt.Errorf("failed to list daemons: %w", err)
	}

	if len(daemons) == 0 {
		fmt.Println()
		fmt.Println(ui.Info(
			"No Running Tunnels",
			"",
			ui.Muted("Start a tunnel in background with:"),
			"",
			ui.Cyan("  drip http 3000 -d"),
			ui.Cyan("  drip tcp 5432 -d"),
		))
		return nil
	}

	table := ui.NewTable([]string{"#", "TYPE", "PORT", "URL", "PID", "UPTIME"}).
		WithTitle("Running Tunnels")

	idx := 1
	for _, d := range daemons {
		if !IsProcessRunning(d.PID) {
			RemoveDaemonInfo(d.Type, d.Port)
			continue
		}

		uptime := time.Since(d.StartTime)

		var typeStr string
		if d.Type == "http" {
			typeStr = ui.Highlight("HTTP")
		} else if d.Type == "https" {
			typeStr = ui.Highlight("HTTPS")
		} else {
			typeStr = ui.Cyan("TCP")
		}

		table.AddRow([]string{
			ui.Highlight(fmt.Sprintf("%d", idx)),
			typeStr,
			fmt.Sprintf("%d", d.Port),
			ui.URL(d.URL),
			ui.Muted(fmt.Sprintf("%d", d.PID)),
			FormatDuration(uptime),
		})
		idx++
	}

	fmt.Print(table.Render())

	if interactiveMode || shouldPromptForAction() {
		return runInteractiveList(daemons)
	}

	fmt.Println(ui.Muted("Commands:"))
	fmt.Println(ui.RenderList([]string{
		ui.Cyan("drip list -i") + ui.Muted("               Interactive mode"),
		ui.Cyan("drip attach http 3000") + ui.Muted("      Attach to tunnel (view logs)"),
		ui.Cyan("drip stop http 3000") + ui.Muted("        Stop tunnel"),
		ui.Cyan("drip stop all") + ui.Muted("              Stop all tunnels"),
	}))

	return nil
}

func shouldPromptForAction() bool {
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) == 0 {
		return false
	}
	return false
}

func runInteractiveList(daemons []*DaemonInfo) error {
	var runningDaemons []*DaemonInfo
	for _, d := range daemons {
		if IsProcessRunning(d.PID) {
			runningDaemons = append(runningDaemons, d)
		} else {
			RemoveDaemonInfo(d.Type, d.Port)
		}
	}

	if len(runningDaemons) == 0 {
		return nil
	}

	fmt.Println()
	fmt.Print(ui.Muted("Select a tunnel (number) or 'q' to quit: "))
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "" || input == "q" || input == "Q" {
		return nil
	}

	selection, err := strconv.Atoi(input)
	if err != nil || selection < 1 || selection > len(runningDaemons) {
		return fmt.Errorf("invalid selection: %s", input)
	}

	selectedDaemon := runningDaemons[selection-1]

	fmt.Println()
	fmt.Println(ui.Info(
		fmt.Sprintf("Selected: %s tunnel on port %d", strings.ToUpper(selectedDaemon.Type), selectedDaemon.Port),
		"",
		ui.Muted("What would you like to do?"),
		"",
		ui.Cyan("  1.") + " Attach (view logs)",
		ui.Cyan("  2.") + " Stop tunnel",
		ui.Muted("  q.") + " Cancel",
	))

	fmt.Print(ui.Muted("Choose an action: "))

	actionInput, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	actionInput = strings.TrimSpace(actionInput)
	switch actionInput {
	case "1":
		return attachToDaemon(selectedDaemon)
	case "2":
		return stopDaemon(selectedDaemon.Type, selectedDaemon.Port)
	case "q", "Q", "":
		return nil
	default:
		return fmt.Errorf("invalid action: %s", actionInput)
	}
}
