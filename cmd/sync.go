package cmd

import (
	"database/sql"
	"fmt"
	"go/scanner"
	"go/token"
	"io"
	"io/fs"
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

type dbScenario struct {
	ID      int64
	Name    string
	Content sql.NullString
}

type scenarioAction struct {
	kind string // "new", "modified", "removed", "unchanged"
	id   int64
	name string
}

func loadDBScenarios(sqlDB *sql.DB, fileID int64) map[int64]dbScenario {
	rows, err := sqlDB.Query(`SELECT id, name, content FROM scenarios WHERE file_id = ?`, fileID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	result := make(map[int64]dbScenario)
	for rows.Next() {
		var s dbScenario
		if err := rows.Scan(&s.ID, &s.Name, &s.Content); err != nil {
			continue
		}
		result[s.ID] = s
	}
	return result
}

func scenarioHasStatusHistory(sqlDB *sql.DB, scenarioID int64) bool {
	var count int
	err := sqlDB.QueryRow(`SELECT COUNT(*) FROM statuses WHERE scenario_id = ?`, scenarioID).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

func stepsOf(content string) string {
	_, after, found := strings.Cut(content, "\n")
	if !found {
		return ""
	}
	return after
}

func scenarioLatestStatus(sqlDB *sql.DB, scenarioID int64) string {
	var status string
	err := sqlDB.QueryRow(`SELECT status FROM statuses WHERE scenario_id = ? ORDER BY id DESC LIMIT 1`, scenarioID).Scan(&status)
	if err != nil {
		return ""
	}
	return status
}

func scenarioLatestStatusIsRemoved(sqlDB *sql.DB, scenarioID int64) bool {
	return scenarioLatestStatus(sqlDB, scenarioID) == "removed"
}

func reconcileTrackedFile(sqlDB *sql.DB, fileID int64, pf *parser.ParsedFile) ([]scenarioAction, []tagInsertion) {
	remaining := loadDBScenarios(sqlDB, fileID)
	var actions []scenarioAction
	var insertions []tagInsertion

	for _, ps := range pf.Scenarios {
		matched := false

		if ps.FtTag != "" {
			tagID, err := strconv.ParseInt(ps.FtTag, 10, 64)
			if err == nil {
				if dbS, ok := remaining[tagID]; ok {
					// Matched by tag
					delete(remaining, tagID)
					matched = true

					wasRemoved := scenarioLatestStatusIsRemoved(sqlDB, tagID)
					if wasRemoved {
						sqlDB.Exec(`INSERT INTO statuses (scenario_id, status) VALUES (?, 'restored')`, tagID)
					}

					nameChanged := dbS.Name != ps.Name
					contentChanged := dbS.Content.Valid && stepsOf(dbS.Content.String) != stepsOf(ps.Content)
					firstPopulation := !dbS.Content.Valid

					if wasRemoved {
						sqlDB.Exec(`UPDATE scenarios SET name = ?, content = ?, updated_at = datetime('now') WHERE id = ?`, ps.Name, ps.Content, tagID)
						actions = append(actions, scenarioAction{kind: "new", id: tagID, name: ps.Name})
					} else if nameChanged || contentChanged {
						sqlDB.Exec(`UPDATE scenarios SET name = ?, content = ?, updated_at = datetime('now') WHERE id = ?`, ps.Name, ps.Content, tagID)
						if contentChanged && scenarioHasStatusHistory(sqlDB, tagID) && scenarioLatestStatus(sqlDB, tagID) != "modified" {
							sqlDB.Exec(`INSERT INTO statuses (scenario_id, status) VALUES (?, 'modified')`, tagID)
						}
						actions = append(actions, scenarioAction{kind: "modified", id: tagID, name: ps.Name})
					} else if firstPopulation {
						// Silently populate content without marking as modified
						sqlDB.Exec(`UPDATE scenarios SET content = ?, updated_at = datetime('now') WHERE id = ?`, ps.Content, tagID)
						actions = append(actions, scenarioAction{kind: "unchanged", id: tagID, name: ps.Name})
					} else {
						actions = append(actions, scenarioAction{kind: "unchanged", id: tagID, name: ps.Name})
					}
				}
				// If tag ID not in remaining, fall through to name match
			}
		}

		if !matched {
			// Try name match in remaining
			nameMatched := false
			for dbID, dbS := range remaining {
				if dbS.Name == ps.Name {
					// Matched by name
					delete(remaining, dbID)
					nameMatched = true
					sqlDB.Exec(`UPDATE scenarios SET name = ?, content = ?, updated_at = datetime('now') WHERE id = ?`, ps.Name, ps.Content, dbID)
					insertions = append(insertions, tagInsertion{line: ps.Line, id: dbID})
					actions = append(actions, scenarioAction{kind: "modified", id: dbID, name: ps.Name})
					break
				}
			}

			if !nameMatched {
				// New scenario
				result, err := sqlDB.Exec(`INSERT INTO scenarios (file_id, name, content) VALUES (?, ?, ?)`, fileID, ps.Name, ps.Content)
				if err == nil {
					id, _ := result.LastInsertId()
					insertions = append(insertions, tagInsertion{line: ps.Line, id: id})
					actions = append(actions, scenarioAction{kind: "new", id: id, name: ps.Name})
				}
			}
		}
	}

	// Remaining entries are removed scenarios
	for dbID, dbS := range remaining {
		if scenarioLatestStatusIsRemoved(sqlDB, dbID) {
			continue
		}
		actions = append(actions, scenarioAction{kind: "removed", id: dbID, name: dbS.Name})
		if scenarioHasStatusHistory(sqlDB, dbID) {
			sqlDB.Exec(`INSERT INTO statuses (scenario_id, status) VALUES (?, 'removed')`, dbID)
		} else {
			sqlDB.Exec(`DELETE FROM scenarios WHERE id = ?`, dbID)
		}
	}

	return actions, insertions
}

func handleDeletedFile(sqlDB *sql.DB, fileID int64) ([]scenarioAction, error) {
	remaining := loadDBScenarios(sqlDB, fileID)
	var actions []scenarioAction

	for dbID, dbS := range remaining {
		if scenarioLatestStatusIsRemoved(sqlDB, dbID) {
			continue
		}
		actions = append(actions, scenarioAction{kind: "removed", id: dbID, name: dbS.Name})
		if scenarioHasStatusHistory(sqlDB, dbID) {
			sqlDB.Exec(`INSERT INTO statuses (scenario_id, status) VALUES (?, 'removed')`, dbID)
		} else {
			sqlDB.Exec(`DELETE FROM scenarios WHERE id = ?`, dbID)
		}
	}

	return actions, nil
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

	diskPaths := make(map[string]bool)
	fileCount := 0
	scenarioCount := 0
	for _, path := range matches {
		diskPaths[path] = true

		// Register file in files table (filter deleted = FALSE)
		var fileID int64
		var isNew bool
		err := sqlDB.QueryRow(`SELECT id FROM files WHERE file_path = ? AND deleted = FALSE`, path).Scan(&fileID)
		if err == sql.ErrNoRows {
			// Check if there's a deleted record to undelete
			var deletedID int64
			err2 := sqlDB.QueryRow(`SELECT id FROM files WHERE file_path = ? AND deleted = TRUE`, path).Scan(&deletedID)
			if err2 == nil {
				// Undelete
				sqlDB.Exec(`UPDATE files SET deleted = FALSE, updated_at = datetime('now') WHERE id = ?`, deletedID)
				fileID = deletedID
				isNew = false
			} else {
				result, err := sqlDB.Exec(`INSERT INTO files (file_path) VALUES (?)`, path)
				if err != nil {
					return fmt.Errorf("inserting %s: %w", path, err)
				}
				fileID, _ = result.LastInsertId()
				isNew = true
			}
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

		if isNew {
			// New file path
			ui.NewLine(w, path)

			var insertions []tagInsertion
			for _, ps := range pf.Scenarios {
				result, err := sqlDB.Exec(
					`INSERT INTO scenarios (file_id, name, content) VALUES (?, ?, ?)`,
					fileID, ps.Name, ps.Content,
				)
				if err != nil {
					return fmt.Errorf("inserting scenario %q: %w", ps.Name, err)
				}
				id, _ := result.LastInsertId()
				insertions = append(insertions, tagInsertion{line: ps.Line, id: id})
				ui.ScenarioLine(w, id, ps.Name)
				scenarioCount++
			}

			if len(insertions) > 0 {
				if err := writeTagsToFile(path, insertions); err != nil {
					return fmt.Errorf("writing tags to %s: %w", path, err)
				}
			}
		} else {
			// Tracked file path
			actions, insertions := reconcileTrackedFile(sqlDB, fileID, pf)

			// Determine mod/trk
			hasActivity := false
			for _, a := range actions {
				if a.kind == "new" || a.kind == "modified" || a.kind == "removed" {
					hasActivity = true
					break
				}
			}

			if hasActivity {
				ui.ModLine(w, path)
			} else {
				ui.TrkLine(w, path)
			}

			// Print scenario lines
			for _, a := range actions {
				switch a.kind {
				case "new":
					ui.ScenarioLine(w, a.id, a.name)
					scenarioCount++
				case "modified":
					ui.ModifiedScenarioLine(w, a.id, a.name)
					scenarioCount++
				case "removed":
					ui.RemovedScenarioLine(w, a.id, a.name)
					scenarioCount++
				}
			}

			if len(insertions) > 0 {
				if err := writeTagsToFile(path, insertions); err != nil {
					return fmt.Errorf("writing tags to %s: %w", path, err)
				}
			}
		}
		fileCount++
	}

	// Handle deleted files
	rows, err := sqlDB.Query(`SELECT id, file_path FROM files WHERE deleted = FALSE`)
	if err != nil {
		return fmt.Errorf("querying files: %w", err)
	}
	defer rows.Close()

	type fileRecord struct {
		id   int64
		path string
	}
	var allFiles []fileRecord
	for rows.Next() {
		var f fileRecord
		if err := rows.Scan(&f.id, &f.path); err != nil {
			continue
		}
		allFiles = append(allFiles, f)
	}
	rows.Close()

	for _, f := range allFiles {
		if !diskPaths[f.path] {
			actions, err := handleDeletedFile(sqlDB, f.id)
			if err != nil {
				return fmt.Errorf("handling deleted file %s: %w", f.path, err)
			}

			ui.DelLine(w, f.path)
			fileCount++
			for _, a := range actions {
				ui.RemovedScenarioLine(w, a.id, a.name)
				scenarioCount++
			}

			sqlDB.Exec(`UPDATE files SET deleted = TRUE, updated_at = datetime('now') WHERE id = ?`, f.id)
		}
	}

	syncTestLinks(sqlDB)

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

var testLinkTagRe = regexp.MustCompile(`@ft:(\d+)`)

type testLink struct {
	scenarioID int64
	filePath   string
	lineNumber int
}

func scanTestLinksInFile(path string, src []byte) []testLink {
	fset := token.NewFileSet()
	var s scanner.Scanner
	s.Init(fset.AddFile(path, fset.Base(), len(src)), src, nil, scanner.ScanComments)

	// Collect comment positions that contain @ft:N
	type commentTag struct {
		line int
		id   int64
	}
	var tags []commentTag

	// Also collect func Test line numbers
	funcLines := make(map[int]bool)

	var prevTok token.Token
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.COMMENT {
			m := testLinkTagRe.FindStringSubmatch(lit)
			if m != nil {
				id, err := strconv.ParseInt(m[1], 10, 64)
				if err == nil {
					tags = append(tags, commentTag{line: fset.Position(pos).Line, id: id})
				}
			}
		}
		// Detect "func Test..." — IDENT "Test*" preceded by FUNC keyword
		if tok == token.IDENT && strings.HasPrefix(lit, "Test") && prevTok == token.FUNC {
			funcLines[fset.Position(pos).Line] = true
		}
		prevTok = tok
	}

	// Keep only tags where the next non-blank source line is a func Test
	srcLines := strings.Split(string(src), "\n")
	var links []testLink
	for _, tag := range tags {
		isAboveTest := false
		for j := tag.line; j < len(srcLines); j++ { // tag.line is 1-based, srcLines is 0-based, so j=tag.line is the next line
			trimmed := strings.TrimSpace(srcLines[j])
			if trimmed == "" {
				continue
			}
			if funcLines[j+1] { // j is 0-based index, funcLines keys are 1-based
				isAboveTest = true
			}
			break
		}
		if isAboveTest {
			links = append(links, testLink{scenarioID: tag.id, filePath: path, lineNumber: tag.line})
		}
	}
	return links
}

func syncTestLinks(sqlDB *sql.DB) {
	var links []testLink

	filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "fts" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		links = append(links, scanTestLinksInFile(path, data)...)
		return nil
	})

	// Load all valid scenario IDs in one query
	validIDs := make(map[int64]bool)
	rows, err := sqlDB.Query(`SELECT id FROM scenarios`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			if rows.Scan(&id) == nil {
				validIDs[id] = true
			}
		}
	}

	// Full reconciliation in a single transaction
	tx, err := sqlDB.Begin()
	if err != nil {
		return
	}
	tx.Exec(`DELETE FROM test_links`)
	stmt, err := tx.Prepare(`INSERT INTO test_links (scenario_id, file_path, line_number) VALUES (?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return
	}
	defer stmt.Close()
	for _, l := range links {
		if validIDs[l.scenarioID] {
			stmt.Exec(l.scenarioID, l.filePath, l.lineNumber)
		}
	}
	tx.Commit()
}
