package ui

import (
	"fmt"
)

// RenderConfigInit renders config initialization UI
func RenderConfigInit() string {
	title := "Drip Configuration Setup"
	box := boxStyle.Copy().Width(50)
	return "\n" + box.Render(titleStyle.Render(title)) + "\n"
}

// RenderConfigShow renders the config display
func RenderConfigShow(server, token string, tokenHidden bool, tlsEnabled bool, configPath string) string {
	lines := []string{
		KeyValue("Server", server),
	}

	if token != "" {
		if tokenHidden {
			if len(token) > 10 {
				displayToken := token[:3] + "***" + token[len(token)-3:]
				lines = append(lines, KeyValue("Token", Muted(displayToken+" (hidden)")))
			} else {
				lines = append(lines, KeyValue("Token", Muted(token[:3]+"*** (hidden)")))
			}
		} else {
			lines = append(lines, KeyValue("Token", token))
		}
	} else {
		lines = append(lines, KeyValue("Token", Muted("(not set)")))
	}

	tlsStatus := "enabled"
	if !tlsEnabled {
		tlsStatus = "disabled"
	}
	lines = append(lines, KeyValue("TLS", tlsStatus))
	lines = append(lines, KeyValue("Config", Muted(configPath)))

	return Info("Current Configuration", lines...)
}

// RenderConfigSaved renders config saved message
func RenderConfigSaved(configPath string) string {
	return SuccessBox(
		"Configuration Saved",
		Muted("Config saved to: ")+configPath,
		"",
		Muted("You can now use 'drip' without --server and --token flags"),
	)
}

// RenderConfigUpdated renders config updated message
func RenderConfigUpdated(updates []string) string {
	lines := make([]string, len(updates)+1)
	for i, update := range updates {
		lines[i] = Success(update)
	}
	lines[len(updates)] = ""
	lines = append(lines, Muted("Configuration has been updated"))
	return SuccessBox("Configuration Updated", lines...)
}

// RenderConfigDeleted renders config deleted message
func RenderConfigDeleted() string {
	return SuccessBox("Configuration Deleted", Muted("Configuration file has been removed"))
}

// RenderConfigValidation renders config validation results
func RenderConfigValidation(serverValid, tokenSet, tlsEnabled bool) string {
	lines := []string{}

	if serverValid {
		lines = append(lines, Success("Server address is valid"))
	} else {
		lines = append(lines, Error("Server address is not set"))
	}

	if tokenSet {
		lines = append(lines, Success("Token is set"))
	} else {
		lines = append(lines, Warning("Token is not set (authentication may fail)"))
	}

	if tlsEnabled {
		lines = append(lines, Success("TLS is enabled"))
	} else {
		lines = append(lines, Warning("TLS is disabled (not recommended for production)"))
	}

	lines = append(lines, "")
	lines = append(lines, Muted("Configuration validation complete"))

	if serverValid && tokenSet && tlsEnabled {
		return SuccessBox("Configuration Valid", lines...)
	}
	return WarningBox("Configuration Validation", lines...)
}

// RenderDaemonStarted renders daemon started message
func RenderDaemonStarted(tunnelType string, port int, pid int, logPath string) string {
	lines := []string{
		KeyValue("Type", Highlight(tunnelType)),
		KeyValue("Port", fmt.Sprintf("%d", port)),
		KeyValue("PID", fmt.Sprintf("%d", pid)),
		"",
		Muted("Commands:"),
		Cyan("  drip list") + Muted("           Check tunnel status"),
		Cyan(fmt.Sprintf("  drip attach %s %d", tunnelType, port)) + Muted("  View logs"),
		Cyan(fmt.Sprintf("  drip stop %s %d", tunnelType, port)) + Muted("    Stop tunnel"),
		"",
		Muted("Logs: ")+mutedStyle.Render(logPath),
	}
	return SuccessBox("Tunnel Started in Background", lines...)
}
