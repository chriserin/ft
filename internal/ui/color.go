package ui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	newStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	trkStyle     = lipgloss.NewStyle().Faint(true)
	errStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	modStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	delStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	plusStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	tildeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	minusStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
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

func ModLine(w io.Writer, path string) {
	fmt.Fprintln(w, modStyle.Render("mod")+"  "+path)
}

func DelLine(w io.Writer, path string) {
	fmt.Fprintln(w, delStyle.Render("del")+"  "+path)
}

func ScenarioLine(w io.Writer, id int64, name string) {
	fmt.Fprintf(w, "       %s %s %s\n", plusStyle.Render("+"), ftTagStyle.Render(fmt.Sprintf("@ft:%d", id)), name)
}

func ModifiedScenarioLine(w io.Writer, id int64, name string) {
	fmt.Fprintf(w, "       %s %s %s\n", tildeStyle.Render("~"), ftTagStyle.Render(fmt.Sprintf("@ft:%d", id)), name)
}

func RemovedScenarioLine(w io.Writer, id int64, name string) {
	fmt.Fprintf(w, "       %s %s %s\n", minusStyle.Render("-"), ftTagStyle.Render(fmt.Sprintf("@ft:%d", id)), name)
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

type HistoryEntry struct {
	Status    string
	ChangedAt time.Time
}

func StatusConfirm(w io.Writer, id int64, prevStatus, status string) {
	tag := idStyle.Render(fmt.Sprintf("@ft:%d", id))
	if prevStatus == "" {
		fmt.Fprintf(w, "%s → %s\n", tag, status)
	} else {
		fmt.Fprintf(w, "%s %s → %s\n", tag, prevStatus, status)
	}
}

func ShowHistory(w io.Writer, entries []HistoryEntry) {
	// Compute max status width for alignment
	maxWidth := 0
	for _, e := range entries {
		if len(e.Status) > maxWidth {
			maxWidth = len(e.Status)
		}
	}

	fmt.Fprintln(w, "History:")
	for _, e := range entries {
		ts := e.ChangedAt.Format("Jan 2, 2006 3:04pm")
		padded := fmt.Sprintf("%-*s", maxWidth, e.Status)
		fmt.Fprintf(w, "  %s  %s\n", trkStyle.Render(padded), trkStyle.Render(ts))
	}
}

func SummaryLine(w io.Writer, fileCount, scenarioCount int) {
	if scenarioCount > 0 {
		fmt.Fprintf(w, "synced %d files, %d scenarios\n", fileCount, scenarioCount)
	} else {
		fmt.Fprintf(w, "synced %d files\n", fileCount)
	}
}
