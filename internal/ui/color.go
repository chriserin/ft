package ui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/lipgloss"
)

var (
	newStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	trkStyle = lipgloss.NewStyle().Faint(true)
)

func NewLine(w io.Writer, path string) {
	fmt.Fprintln(w, newStyle.Render("new")+"  "+path)
}

func TrkLine(w io.Writer, path string) {
	fmt.Fprintln(w, trkStyle.Render("trk")+"  "+path)
}

func SummaryLine(w io.Writer, count int) {
	fmt.Fprintf(w, "synced %d files\n", count)
}
