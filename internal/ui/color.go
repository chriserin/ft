package ui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/lipgloss"
)

var (
	newStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	trkStyle   = lipgloss.NewStyle().Faint(true)
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	plusStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	ftTagStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

func NewLine(w io.Writer, path string) {
	fmt.Fprintln(w, newStyle.Render("new")+"  "+path)
}

func TrkLine(w io.Writer, path string) {
	fmt.Fprintln(w, trkStyle.Render("trk")+"  "+path)
}

func ErrLine(w io.Writer, path, message string) {
	fmt.Fprintf(w, "%s  %s — %s\n", errStyle.Render("err"), path, message)
}

func ScenarioLine(w io.Writer, id int64, name string) {
	fmt.Fprintf(w, "       %s %s %s\n", plusStyle.Render("+"), ftTagStyle.Render(fmt.Sprintf("@ft:%d", id)), name)
}

func SummaryLine(w io.Writer, fileCount, scenarioCount int) {
	if scenarioCount > 0 {
		fmt.Fprintf(w, "synced %d files, %d scenarios\n", fileCount, scenarioCount)
	} else {
		fmt.Fprintf(w, "synced %d files\n", fileCount)
	}
}
