package display

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	accent  = lipgloss.Color("#5ec3ff")
	success = lipgloss.Color("#5ec3ff")
	errClr  = lipgloss.Color("#ff6659")
	dim     = lipgloss.Color("#666666")

	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(accent)
	labelStyle   = lipgloss.NewStyle().Foreground(accent)
	dimStyle     = lipgloss.NewStyle().Foreground(dim)
	successStyle = lipgloss.NewStyle().Bold(true).Foreground(success)
	errorStyle   = lipgloss.NewStyle().Bold(true).Foreground(errClr)
)

// Header prints the huragok banner.
func Header() {
	fmt.Println()
	fmt.Println(titleStyle.Render("  ● HURAGOK — 3D Asset Pipeline"))
	fmt.Println()
}

// Prompt prints the user's prompt.
func Prompt(prompt string) {
	fmt.Printf("  %s  %s\n\n", labelStyle.Render("Prompt:"), prompt)
}

// StageStart prints a stage starting message and returns the start time.
func StageStart(msg string) time.Time {
	fmt.Printf("  %s %s", labelStyle.Render("▸"), msg)
	return time.Now()
}

// StageDone prints completion with elapsed time.
func StageDone(start time.Time) {
	elapsed := time.Since(start).Round(100 * time.Millisecond)
	fmt.Printf(" %s %s\n", successStyle.Render("done"), dimStyle.Render(fmt.Sprintf("(%s)", elapsed)))
}

// StageInfo prints an indented info line.
func StageInfo(msg string) {
	fmt.Printf("    %s\n", dimStyle.Render(msg))
}

// Success prints the final success message.
func Success(path string, sizeMB float64) {
	fmt.Println()
	fmt.Printf("  %s %s %s\n\n",
		successStyle.Render("✓ Output →"),
		path,
		dimStyle.Render(fmt.Sprintf("(%.1f MB)", sizeMB)),
	)
}

// Error prints an error message.
func Error(msg string) {
	fmt.Printf("\n  %s %s\n\n", errorStyle.Render("✗ Error:"), msg)
}
