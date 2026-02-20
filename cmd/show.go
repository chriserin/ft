package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chriserin/ft/internal/db"
	"github.com/chriserin/ft/internal/parser"
	"github.com/chriserin/ft/internal/ui"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show a scenario by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunShow(cmd.OutOrStdout(), args[0])
	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}

func RunShow(w io.Writer, rawID string) error {
	// Strip @ft: prefix if present
	rawID = strings.TrimPrefix(rawID, "@ft:")
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid scenario ID: %s", rawID)
	}

	if _, err := os.Stat("fts"); os.IsNotExist(err) {
		return fmt.Errorf("run `ft init` first")
	}

	sqlDB, err := db.Open("fts/ft.db")
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer sqlDB.Close()

	var scenarioID int64
	var scenarioName string
	var filePath string
	err = sqlDB.QueryRow(`
		SELECT s.id, s.name, f.file_path
		FROM scenarios s
		JOIN files f ON s.file_id = f.id
		WHERE s.id = ?
	`, id).Scan(&scenarioID, &scenarioName, &filePath)
	if err != nil {
		return fmt.Errorf("scenario %d not found", id)
	}

	fileName := filepath.Base(filePath)

	// Read and parse the file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", filePath, err)
	}

	doc, parseErrors := parser.Parse(filePath, content)
	pf := parser.Transform(doc, filePath, content, parseErrors)

	// Find the matching scenario by FtTag
	idStr := strconv.FormatInt(id, 10)
	var matched *parser.ParsedScenario
	for i := range pf.Scenarios {
		if pf.Scenarios[i].FtTag == idStr {
			matched = &pf.Scenarios[i]
			break
		}
	}
	if matched == nil {
		return fmt.Errorf("scenario %d not found in file %s", id, filePath)
	}

	// Extract Background content from raw file lines
	background := extractBackground(string(content))

	// Print header and status
	ui.ShowHeader(w, scenarioID, fileName)
	ui.ShowStatus(w, "no-activity")

	// Print Background if present
	if background != "" {
		fmt.Fprintln(w)
		ui.ShowGherkin(w, background)
	}

	// Print scenario content
	fmt.Fprintln(w)
	ui.ShowGherkin(w, matched.Content)

	return nil
}

// extractBackground finds the Background: section in raw file content
// and returns it as a string, collecting lines until the next keyword or tag.
func extractBackground(content string) string {
	lines := strings.Split(content, "\n")
	inBackground := false
	var bgLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Background:") {
			inBackground = true
			bgLines = append(bgLines, line)
			continue
		}
		if inBackground {
			// Stop at next keyword, tag, or scenario
			if strings.HasPrefix(trimmed, "Scenario:") ||
				strings.HasPrefix(trimmed, "Scenario Outline:") ||
				strings.HasPrefix(trimmed, "@") ||
				strings.HasPrefix(trimmed, "Rule:") ||
				strings.HasPrefix(trimmed, "Examples:") {
				break
			}
			bgLines = append(bgLines, line)
		}
	}

	if len(bgLines) == 0 {
		return ""
	}

	// Trim trailing blank lines
	for len(bgLines) > 0 && strings.TrimSpace(bgLines[len(bgLines)-1]) == "" {
		bgLines = bgLines[:len(bgLines)-1]
	}

	return strings.Join(bgLines, "\n")
}
