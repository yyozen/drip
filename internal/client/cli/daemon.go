package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"drip/internal/client/cli/ui"
	json "github.com/goccy/go-json"
)

// DaemonInfo stores information about a running daemon process
type DaemonInfo struct {
	PID        int       `json:"pid"`
	Type       string    `json:"type"`       // "http" or "tcp"
	Port       int       `json:"port"`       // Local port being tunneled
	Subdomain  string    `json:"subdomain"`  // Subdomain if specified
	Server     string    `json:"server"`     // Server address
	URL        string    `json:"url"`        // Tunnel URL
	StartTime  time.Time `json:"start_time"` // When the daemon started
	Executable string    `json:"executable"` // Path to the executable
}

// getDaemonDir returns the directory for storing daemon info
func getDaemonDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".drip"
	}
	return filepath.Join(home, ".drip", "daemons")
}

// getDaemonFilePath returns the path to a daemon info file
func getDaemonFilePath(tunnelType string, port int) string {
	return filepath.Join(getDaemonDir(), fmt.Sprintf("%s_%d.json", tunnelType, port))
}

// SaveDaemonInfo saves daemon information to a file
func SaveDaemonInfo(info *DaemonInfo) error {
	dir := getDaemonDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create daemon directory: %w", err)
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal daemon info: %w", err)
	}

	path := getDaemonFilePath(info.Type, info.Port)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write daemon info: %w", err)
	}

	return nil
}

// LoadDaemonInfo loads daemon information from a file
func LoadDaemonInfo(tunnelType string, port int) (*DaemonInfo, error) {
	path := getDaemonFilePath(tunnelType, port)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read daemon info: %w", err)
	}

	var info DaemonInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to parse daemon info: %w", err)
	}

	return &info, nil
}

// RemoveDaemonInfo removes a daemon info file
func RemoveDaemonInfo(tunnelType string, port int) error {
	path := getDaemonFilePath(tunnelType, port)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove daemon info: %w", err)
	}
	return nil
}

// ListAllDaemons returns all daemon info files
func ListAllDaemons() ([]*DaemonInfo, error) {
	dir := getDaemonDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read daemon directory: %w", err)
	}

	var daemons []*DaemonInfo
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		var info DaemonInfo
		if err := json.Unmarshal(data, &info); err != nil {
			continue
		}

		daemons = append(daemons, &info)
	}

	return daemons, nil
}

// IsProcessRunning checks if a process with the given PID is running
func IsProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	return isProcessRunningOS(process)
}

// KillProcess kills a process by PID
func KillProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("process not found: %w", err)
	}

	if err := killProcessOS(process); err != nil {
		return fmt.Errorf("failed to kill process: %w", err)
	}

	return nil
}

// StartDaemon starts the current process as a daemon
func StartDaemon(tunnelType string, port int, args []string) error {
	// Get the executable path
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Build command arguments (remove -D/--daemon flag to prevent recursion)
	var cleanArgs []string
	skipNext := false
	for i, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}
		// Skip -D or --daemon flags (but NOT --daemon-child)
		if arg == "-D" || arg == "--daemon" {
			continue
		}
		// Handle -d (short form) - skip it
		if arg == "-d" {
			continue
		}
		// Skip if next arg would be a value for a removed flag (not applicable for boolean)
		_ = i
		cleanArgs = append(cleanArgs, arg)
	}

	cmd := exec.Command(executable, cleanArgs...)

	setupDaemonCmd(cmd)

	logDir := getDaemonDir()
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return fmt.Errorf("failed to create daemon directory: %w", err)
	}
	logPath := filepath.Join(logDir, fmt.Sprintf("%s_%d.log", tunnelType, port))
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}

	devNull, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
	if err != nil {
		logFile.Close()
		return fmt.Errorf("failed to open /dev/null: %w", err)
	}
	cmd.Stdin = devNull
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		logFile.Close()
		devNull.Close()
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Don't wait for the process - let it run in background
	// The child process will save its own daemon info after connecting

	fmt.Println(ui.RenderDaemonStarted(tunnelType, port, cmd.Process.Pid, logPath))

	return nil
}

// CleanupStaleDaemons removes daemon info for processes that are no longer running
func CleanupStaleDaemons() error {
	daemons, err := ListAllDaemons()
	if err != nil {
		return err
	}

	for _, info := range daemons {
		if !IsProcessRunning(info.PID) {
			RemoveDaemonInfo(info.Type, info.Port)
		}
	}

	return nil
}

// FormatDuration formats a duration in a human-readable way
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}

// ParsePortFromArgs extracts the port number from command arguments
func ParsePortFromArgs(args []string) (int, error) {
	for _, arg := range args {
		if len(arg) > 0 && arg[0] == '-' {
			continue
		}
		port, err := strconv.Atoi(arg)
		if err == nil && port > 0 && port <= 65535 {
			return port, nil
		}
	}
	return 0, fmt.Errorf("port number not found in arguments")
}
