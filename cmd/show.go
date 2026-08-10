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

	store, err := db.OpenProjectStore()
	if err != nil {
		return err
	}
	defer store.Close()

	detail, err := store.ScenarioDetail(id)
	if err != nil {
		return fmt.Errorf("scenario %d not found", id)
	}
	scenarioID := detail.ID
	filePath := detail.FilePath
	storedContent := detail.Content

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
	if statusStr, err := store.CurrentStatus(id); err == nil {
		currentStatus = statusStr
	}

	// Query history
	statusHistory, err := store.StatusHistory(id)
	if err != nil {
		return fmt.Errorf("querying status history: %w", err)
	}

	var history []ui.HistoryEntry
	for _, e := range statusHistory {
		history = append(history, ui.HistoryEntry{Status: e.Status, ChangedAt: e.ChangedAt})
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
	dbTestLinks, err := store.TestLinks(id)
	if err == nil {
		for _, tl := range dbTestLinks {
			testLinks = append(testLinks, ui.TestLink{FilePath: tl.FilePath, LineNumber: tl.LineNumber})
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

	store, err := db.OpenProjectStore()
	if err != nil {
		return err
	}
	defer store.Close()

	scenarioName, createdAt, err := store.ScenarioBasic(id)
	if err != nil {
		return fmt.Errorf("scenario %d not found", id)
	}

	statusHistory, err := store.StatusHistory(id)
	if err != nil {
		return fmt.Errorf("querying status history: %w", err)
	}

	var history []ui.HistoryEntry
	for _, e := range statusHistory {
		history = append(history, ui.HistoryEntry{Status: e.Status, ChangedAt: e.ChangedAt})
	}

	ui.ShowHistoryHeader(w, id, scenarioName)

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
