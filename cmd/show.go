package cmd

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chriserin/ft/internal/db"
	"github.com/chriserin/ft/internal/parser"
	"github.com/chriserin/ft/internal/ui"
	"github.com/spf13/cobra"
)

var showHistory bool

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show a scenario by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if showHistory {
			return RunShowHistory(cmd.OutOrStdout(), args[0])
		}
		return RunShow(cmd.OutOrStdout(), args[0])
	},
}

func init() {
	showCmd.Flags().BoolVar(&showHistory, "history", false, "Show only the status history")
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
	var storedContent sql.NullString
	err = sqlDB.QueryRow(`
		SELECT s.id, s.name, f.file_path, s.content
		FROM scenarios s
		JOIN files f ON s.file_id = f.id
		WHERE s.id = ?
	`, id).Scan(&scenarioID, &scenarioName, &filePath, &storedContent)
	if err != nil {
		return fmt.Errorf("scenario %d not found", id)
	}

	fileName := filepath.Base(filePath)

	// Read and parse the file
	var scenarioContent string
	var background string
	content, readErr := os.ReadFile(filePath)
	if readErr == nil {
		doc, parseErrors := parser.Parse(filePath, content)
		pf := parser.Transform(doc, filePath, content, parseErrors)

		// Find the matching scenario by FtTag
		idStr := strconv.FormatInt(id, 10)
		for i := range pf.Scenarios {
			if pf.Scenarios[i].FtTag == idStr {
				scenarioContent = pf.Scenarios[i].Content
				break
			}
		}

		background = extractBackground(string(content))
	}

	// Fall back to stored content for removed scenarios
	if scenarioContent == "" && storedContent.Valid {
		scenarioContent = storedContent.String
	}

	if scenarioContent == "" {
		return fmt.Errorf("scenario %d not found in file %s", id, filePath)
	}

	// Query current status
	currentStatus := "no-activity"
	var statusStr string
	err = sqlDB.QueryRow(`SELECT status FROM statuses WHERE scenario_id = ? ORDER BY changed_at DESC, id DESC LIMIT 1`, id).Scan(&statusStr)
	if err == nil {
		currentStatus = statusStr
	}

	// Query history
	histRows, err := sqlDB.Query(`SELECT status, changed_at FROM statuses WHERE scenario_id = ? ORDER BY changed_at DESC, id DESC`, id)
	if err != nil {
		return fmt.Errorf("querying status history: %w", err)
	}
	defer histRows.Close()

	var history []ui.HistoryEntry
	for histRows.Next() {
		var s string
		var changedAt time.Time
		if err := histRows.Scan(&s, &changedAt); err != nil {
			return fmt.Errorf("scanning history row: %w", err)
		}
		history = append(history, ui.HistoryEntry{Status: s, ChangedAt: changedAt})
	}

	// Print header and status
	ui.ShowHeader(w, scenarioID, fileName)
	ui.ShowStatus(w, currentStatus)

	// Print history if present
	if len(history) > 0 {
		ui.ShowHistory(w, history)
	}

	// Query test links
	var testLinks []ui.TestLink
	tlRows, err := sqlDB.Query(`SELECT file_path, line_number FROM test_links WHERE scenario_id = ? ORDER BY file_path, line_number`, id)
	if err == nil {
		defer tlRows.Close()
		for tlRows.Next() {
			var tl ui.TestLink
			if err := tlRows.Scan(&tl.FilePath, &tl.LineNumber); err == nil {
				testLinks = append(testLinks, tl)
			}
		}
	}
	if len(testLinks) > 0 {
		fmt.Fprintln(w)
		ui.ShowTests(w, testLinks)
	}

	// Print Background if present
	if background != "" {
		fmt.Fprintln(w)
		ui.ShowGherkin(w, background)
	}

	// Print scenario content
	fmt.Fprintln(w)
	ui.ShowGherkin(w, scenarioContent)

	return nil
}

func RunShowHistory(w io.Writer, rawID string) error {
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
	var createdAt time.Time
	err = sqlDB.QueryRow(`SELECT id, name, created_at FROM scenarios WHERE id = ?`, id).Scan(&scenarioID, &scenarioName, &createdAt)
	if err != nil {
		return fmt.Errorf("scenario %d not found", id)
	}

	histRows, err := sqlDB.Query(`SELECT status, changed_at FROM statuses WHERE scenario_id = ? ORDER BY changed_at DESC, id DESC`, id)
	if err != nil {
		return fmt.Errorf("querying status history: %w", err)
	}
	defer histRows.Close()

	var history []ui.HistoryEntry
	for histRows.Next() {
		var s string
		var changedAt time.Time
		if err := histRows.Scan(&s, &changedAt); err != nil {
			return fmt.Errorf("scanning history row: %w", err)
		}
		history = append(history, ui.HistoryEntry{Status: s, ChangedAt: changedAt})
	}

	ui.ShowHistoryHeader(w, scenarioID, scenarioName)

	if len(history) == 0 {
		history = append(history, ui.HistoryEntry{Status: "no-activity", ChangedAt: createdAt})
	}

	ui.ShowHistoryRows(w, history)

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
