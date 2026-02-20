package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	newStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	trkStyle     = lipgloss.NewStyle().Faint(true)
	errStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	plusStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	ftTagStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	idStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	keywordStyle = lipgloss.NewStyle().Bold(true)
	stepStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	fileStyle    = lipgloss.NewStyle().Faint(true)
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

func ListRow(w io.Writer, id int64, fileName, scenarioName, status string, idWidth, fileWidth, nameWidth int) {
	tag := fmt.Sprintf("@ft:%d", id)
	fmt.Fprintf(w, "%s  %-*s  %-*s  %s\n",
		idStyle.Render(fmt.Sprintf("%-*s", idWidth, tag)),
		fileWidth, fileName,
		nameWidth, scenarioName,
		trkStyle.Render(status),
	)
}

func ShowHeader(w io.Writer, id int64, fileName string) {
	fmt.Fprintf(w, "%s  %s\n", idStyle.Render(fmt.Sprintf("@ft:%d", id)), fileStyle.Render(fileName))
}

func ShowStatus(w io.Writer, status string) {
	fmt.Fprintf(w, "Status: %s\n", trkStyle.Render(status))
}

// sectionKeywords are Gherkin keywords that start a section.
var sectionKeywords = []string{"Background:", "Scenario:"}

// stepKeywords are Gherkin step keywords.
var stepKeywords = []string{"Given ", "When ", "Then ", "And ", "But "}

// ShowGherkin prints Gherkin content with syntax coloring.
func ShowGherkin(w io.Writer, content string) {
	lines := strings.Split(content, "\n")
	inDocString := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track doc string delimiters
		if strings.HasPrefix(trimmed, `"""`) || strings.HasPrefix(trimmed, "```") {
			fmt.Fprintln(w, trkStyle.Render(line))
			inDocString = !inDocString
			continue
		}
		if inDocString {
			fmt.Fprintln(w, trkStyle.Render(line))
			continue
		}

		rendered := false
		// Check section keywords
		for _, kw := range sectionKeywords {
			if strings.HasPrefix(trimmed, kw) {
				idx := strings.Index(line, kw)
				fmt.Fprintln(w, line[:idx]+keywordStyle.Render(kw)+line[idx+len(kw):])
				rendered = true
				break
			}
		}
		if rendered {
			continue
		}
		// Check step keywords
		for _, kw := range stepKeywords {
			if strings.HasPrefix(trimmed, kw) {
				idx := strings.Index(line, kw)
				kwTrimmed := strings.TrimRight(kw, " ")
				fmt.Fprintln(w, line[:idx]+stepStyle.Render(kwTrimmed)+line[idx+len(kwTrimmed):])
				rendered = true
				break
			}
		}
		if rendered {
			continue
		}
		fmt.Fprintln(w, line)
	}
}

func SummaryLine(w io.Writer, fileCount, scenarioCount int) {
	if scenarioCount > 0 {
		fmt.Fprintf(w, "synced %d files, %d scenarios\n", fileCount, scenarioCount)
	} else {
		fmt.Fprintf(w, "synced %d files\n", fileCount)
	}
}
