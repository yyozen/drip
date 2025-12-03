package ui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors inspired by Vercel CLI
	primaryColor   = lipgloss.Color("#0070F3")
	successColor   = lipgloss.Color("#0070F3")
	warningColor   = lipgloss.Color("#F5A623")
	errorColor     = lipgloss.Color("#E00")
	mutedColor     = lipgloss.Color("#888")
	highlightColor = lipgloss.Color("#0070F3")
	cyanColor      = lipgloss.Color("#50E3C2")
	purpleColor    = lipgloss.Color("#7928CA")

	// Base styles
	baseStyle = lipgloss.NewStyle().
			PaddingLeft(1).
			PaddingRight(1)

	// Box styles - Vercel-like clean box
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#333")).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1)

	successBoxStyle = boxStyle.Copy().
			BorderForeground(successColor)

	warningBoxStyle = boxStyle.Copy().
			BorderForeground(warningColor)

	errorBoxStyle = boxStyle.Copy().
			BorderForeground(errorColor)

	// Text styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFF"))

	subtitleStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	successStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(warningColor).
			Bold(true)

	mutedStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	highlightStyle = lipgloss.NewStyle().
			Foreground(highlightColor).
			Bold(true)

	cyanStyle = lipgloss.NewStyle().
			Foreground(cyanColor)

	urlStyle = lipgloss.NewStyle().
			Foreground(highlightColor).
			Underline(true).
			Bold(true)

	labelStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Width(12)

	valueStyle = lipgloss.NewStyle().
			Bold(true)

	// Table styles
	tableHeaderStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Bold(true).
				PaddingRight(2)

	tableCellStyle = lipgloss.NewStyle().
			PaddingRight(2)

	tableRowHighlight = lipgloss.NewStyle().
				Foreground(highlightColor).
				Bold(true)
)

// Success returns a styled success message
func Success(text string) string {
	return successStyle.Render("✓ " + text)
}

// Error returns a styled error message
func Error(text string) string {
	return errorStyle.Render("✗ " + text)
}

// Warning returns a styled warning message
func Warning(text string) string {
	return warningStyle.Render("⚠ " + text)
}

// Muted returns a styled muted text
func Muted(text string) string {
	return mutedStyle.Render(text)
}

// Highlight returns a styled highlighted text
func Highlight(text string) string {
	return highlightStyle.Render(text)
}

// Cyan returns a styled cyan text
func Cyan(text string) string {
	return cyanStyle.Render(text)
}

// URL returns a styled URL
func URL(text string) string {
	return urlStyle.Render(text)
}

// Title returns a styled title
func Title(text string) string {
	return titleStyle.Render(text)
}

// Subtitle returns a styled subtitle
func Subtitle(text string) string {
	return subtitleStyle.Render(text)
}

// KeyValue returns a styled key-value pair
func KeyValue(key, value string) string {
	return labelStyle.Render(key+":") + " " + valueStyle.Render(value)
}

// Info renders an info box (Vercel-style)
func Info(title string, lines ...string) string {
	content := titleStyle.Render(title)
	if len(lines) > 0 {
		content += "\n\n"
		for i, line := range lines {
			if i > 0 {
				content += "\n"
			}
			content += line
		}
	}
	return boxStyle.Render(content)
}

// SuccessBox renders a success box
func SuccessBox(title string, lines ...string) string {
	content := successStyle.Render("✓ " + title)
	if len(lines) > 0 {
		content += "\n\n"
		for i, line := range lines {
			if i > 0 {
				content += "\n"
			}
			content += line
		}
	}
	return successBoxStyle.Render(content)
}

// WarningBox renders a warning box
func WarningBox(title string, lines ...string) string {
	content := warningStyle.Render("⚠ " + title)
	if len(lines) > 0 {
		content += "\n\n"
		for i, line := range lines {
			if i > 0 {
				content += "\n"
			}
			content += line
		}
	}
	return warningBoxStyle.Render(content)
}

// ErrorBox renders an error box
func ErrorBox(title string, lines ...string) string {
	content := errorStyle.Render("✗ " + title)
	if len(lines) > 0 {
		content += "\n\n"
		for i, line := range lines {
			if i > 0 {
				content += "\n"
			}
			content += line
		}
	}
	return errorBoxStyle.Render(content)
}
