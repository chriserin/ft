package cmd

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/chriserin/ft/internal/db"
	"github.com/chriserin/ft/internal/parser"
	"github.com/chriserin/ft/internal/ui"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Scan fts/ for .ft files and register new ones",
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunSync(cmd.OutOrStdout())
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}

type tagInsertion struct {
	line int   // 1-based line number of the Scenario: line
	id   int64 // scenario ID
}

func RunSync(w io.Writer) error {
	if _, err := os.Stat("fts"); os.IsNotExist(err) {
		return fmt.Errorf("run `ft init` first")
	}

	sqlDB, err := db.Open("fts/ft.db")
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer sqlDB.Close()

	matches, err := filepath.Glob("fts/*.ft")
	if err != nil {
		return fmt.Errorf("scanning fts/: %w", err)
	}
	sort.Strings(matches)

	fileCount := 0
	scenarioCount := 0
	for _, path := range matches {
		// Register file in files table
		var fileID int64
		err := sqlDB.QueryRow(`SELECT id FROM files WHERE file_path = ?`, path).Scan(&fileID)
		isNew := false
		if err == sql.ErrNoRows {
			result, err := sqlDB.Exec(`INSERT INTO files (file_path) VALUES (?)`, path)
			if err != nil {
				return fmt.Errorf("inserting %s: %w", path, err)
			}
			fileID, _ = result.LastInsertId()
			isNew = true
		} else if err != nil {
			return fmt.Errorf("querying %s: %w", path, err)
		}

		// Read and parse file
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		doc, parseErrors := parser.Parse(path, content)
		pf := parser.Transform(doc, path, content, parseErrors)

		// If errors, write error comments to file, print err line, skip
		if len(pf.Errors) > 0 {
			if err := writeErrorsToFile(path, pf.Errors); err != nil {
				return fmt.Errorf("writing errors to %s: %w", path, err)
			}
			for _, pe := range pf.Errors {
				ui.ErrLine(w, path, pe.Message)
			}
			if isNew {
				ui.NewLine(w, path)
			} else {
				ui.TrkLine(w, path)
			}
			fileCount++
			continue
		}

		// Print file status line
		if isNew {
			ui.NewLine(w, path)
		} else {
			ui.TrkLine(w, path)
		}

		// Process scenarios
		var insertions []tagInsertion
		for _, ps := range pf.Scenarios {
			if ps.FtTag != "" && !isNew {
				// Tracked file with tagged scenario — check DB
				var existingID int64
				err := sqlDB.QueryRow(`SELECT id FROM scenarios WHERE id = ?`, ps.FtTag).Scan(&existingID)
				if err == nil {
					// Record exists, nothing to do
					continue
				}
				if err != sql.ErrNoRows {
					return fmt.Errorf("querying scenario %s: %w", ps.FtTag, err)
				}
				// Tag in file but not in DB — re-register with the existing ID
				tagID, _ := strconv.ParseInt(ps.FtTag, 10, 64)
				_, err = sqlDB.Exec(
					`INSERT INTO scenarios (id, file_id, name) VALUES (?, ?, ?)`,
					tagID, fileID, ps.Name,
				)
				if err != nil {
					return fmt.Errorf("inserting scenario @ft:%s %q: %w", ps.FtTag, ps.Name, err)
				}
				ui.ScenarioLine(w, tagID, ps.Name)
				scenarioCount++
				continue
			}

			// New scenario (or new file ignoring stale tags) — INSERT with fresh ID
			result, err := sqlDB.Exec(
				`INSERT INTO scenarios (file_id, name) VALUES (?, ?)`,
				fileID, ps.Name,
			)
			if err != nil {
				return fmt.Errorf("inserting scenario %q: %w", ps.Name, err)
			}
			id, _ := result.LastInsertId()
			insertions = append(insertions, tagInsertion{line: ps.Line, id: id})
			ui.ScenarioLine(w, id, ps.Name)
			scenarioCount++
		}

		// Write @ft tags to file
		if len(insertions) > 0 {
			if err := writeTagsToFile(path, insertions); err != nil {
				return fmt.Errorf("writing tags to %s: %w", path, err)
			}
		}
		fileCount++
	}

	ui.SummaryLine(w, fileCount, scenarioCount)
	return nil
}

var ftTagLineRe = regexp.MustCompile(`^\s*@ft:\d+\s*$`)

// writeTagsToFile writes @ft:<id> tag lines into the file above each Scenario: line.
// If an existing @ft tag line is already above the Scenario:, it is replaced.
// Modifications are applied from bottom to top to preserve line numbers.
func writeTagsToFile(path string, insertions []tagInsertion) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")

	// Sort insertions by line number descending so we modify from bottom to top
	sort.Slice(insertions, func(i, j int) bool {
		return insertions[i].line > insertions[j].line
	})

	for _, ins := range insertions {
		idx := ins.line - 1 // 0-based index of Scenario: line
		if idx < 0 || idx >= len(lines) {
			continue
		}

		// Match indentation of the Scenario: line
		scenarioLine := lines[idx]
		indent := ""
		for _, ch := range scenarioLine {
			if ch == ' ' || ch == '\t' {
				indent += string(ch)
			} else {
				break
			}
		}

		tagLine := fmt.Sprintf("%s@ft:%d", indent, ins.id)

		// Check if the line above is an existing @ft tag line — replace it
		if idx > 0 && ftTagLineRe.MatchString(lines[idx-1]) {
			lines[idx-1] = tagLine
		} else {
			// Insert new tag line above the Scenario: line
			newLines := make([]string, 0, len(lines)+1)
			newLines = append(newLines, lines[:idx]...)
			newLines = append(newLines, tagLine)
			newLines = append(newLines, lines[idx:]...)
			lines = newLines
		}
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// writeErrorsToFile prepends # ft error: comments to the top of the file.
func writeErrorsToFile(path string, errors []parser.ParseError) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var errorLines []string
	for _, pe := range errors {
		errorLines = append(errorLines, fmt.Sprintf("# ft error: %s (line %d)", pe.Message, pe.Line))
	}

	content := strings.Join(errorLines, "\n") + "\n" + string(data)

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
