package cmd

import (
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

type scenarioAction struct {
	kind string // "new", "modified", "removed", "unchanged"
	id   int64
	name string
}

func stepsOf(content string) string {
	_, after, found := strings.Cut(content, "\n")
	if !found {
		return ""
	}
	return after
}

// insertOrAdoptScenario inserts a scenario belonging to a file not yet
// tracked in the DB. If the scenario already carries an @ft:<id> tag and
// that id isn't claimed by an existing scenario, it's adopted as-is — this
// is what allows a rebuild (fts/ft.db deleted, then `ft sync`) to recreate
// scenarios with their original ids instead of reassigning fresh ones. If
// the id is already claimed by something else, or there's no tag at all,
// a fresh id is assigned exactly as before.
func insertOrAdoptScenario(store *db.Store, fileID int64, ps parser.ParsedScenario) (id int64, adopted bool, err error) {
	if ps.FtTag != "" {
		if tagID, parseErr := strconv.ParseInt(ps.FtTag, 10, 64); parseErr == nil && !store.ScenarioExists(tagID) {
			if err := store.InsertScenarioWithID(tagID, fileID, ps.Name, ps.Content); err != nil {
				return 0, false, err
			}
			return tagID, true, nil
		}
	}

	id, err = store.InsertScenario(fileID, ps.Name, ps.Content)
	return id, false, err
}

func reconcileTrackedFile(store *db.Store, fileID int64, pf *parser.ParsedFile) ([]scenarioAction, []tagInsertion, error) {
	remaining, err := store.ScenariosByFile(fileID)
	if err != nil {
		return nil, nil, err
	}
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

					wasRemoved := store.LatestInsertedStatusIsRemoved(tagID)
					if wasRemoved {
						store.InsertStatus(tagID, "restored")
					}

					nameChanged := dbS.Name != ps.Name
					contentChanged := dbS.Content.Valid && stepsOf(dbS.Content.String) != stepsOf(ps.Content)
					firstPopulation := !dbS.Content.Valid

					if wasRemoved {
						store.UpdateScenarioNameContent(tagID, ps.Name, ps.Content)
						actions = append(actions, scenarioAction{kind: "new", id: tagID, name: ps.Name})
					} else if nameChanged || contentChanged {
						store.UpdateScenarioNameContent(tagID, ps.Name, ps.Content)
						latestStatus, _ := store.LatestInsertedStatus(tagID)
						if contentChanged && store.HasStatusHistory(tagID) && latestStatus != "modified" {
							store.InsertStatus(tagID, "modified")
						}
						actions = append(actions, scenarioAction{kind: "modified", id: tagID, name: ps.Name})
					} else if firstPopulation {
						// Silently populate content without marking as modified
						store.UpdateScenarioContent(tagID, ps.Content)
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
					store.UpdateScenarioNameContent(dbID, ps.Name, ps.Content)
					insertions = append(insertions, tagInsertion{line: ps.Line, id: dbID})
					actions = append(actions, scenarioAction{kind: "modified", id: dbID, name: ps.Name})
					break
				}
			}

			if !nameMatched {
				// New scenario
				id, err := store.InsertScenario(fileID, ps.Name, ps.Content)
				if err == nil {
					insertions = append(insertions, tagInsertion{line: ps.Line, id: id})
					actions = append(actions, scenarioAction{kind: "new", id: id, name: ps.Name})
				}
			}
		}
	}

	// Remaining entries are removed scenarios
	for dbID, dbS := range remaining {
		if store.LatestInsertedStatusIsRemoved(dbID) {
			continue
		}
		actions = append(actions, scenarioAction{kind: "removed", id: dbID, name: dbS.Name})
		if store.HasStatusHistory(dbID) {
			store.InsertStatus(dbID, "removed")
		} else {
			store.DeleteScenario(dbID)
		}
	}

	return actions, insertions, nil
}

func handleDeletedFile(store *db.Store, fileID int64) ([]scenarioAction, error) {
	remaining, err := store.ScenariosByFile(fileID)
	if err != nil {
		return nil, err
	}
	var actions []scenarioAction

	for dbID, dbS := range remaining {
		if store.LatestInsertedStatusIsRemoved(dbID) {
			continue
		}
		actions = append(actions, scenarioAction{kind: "removed", id: dbID, name: dbS.Name})
		if store.HasStatusHistory(dbID) {
			store.InsertStatus(dbID, "removed")
		} else {
			store.DeleteScenario(dbID)
		}
	}

	return actions, nil
}

func RunSync(w io.Writer) error {
	store, err := db.OpenProjectStore()
	if err != nil {
		return err
	}
	defer store.Close()

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
		activeID, found, err := store.FindActiveFileID(path)
		if err != nil {
			return fmt.Errorf("querying %s: %w", path, err)
		}
		if found {
			fileID = activeID
		} else {
			// Check if there's a deleted record to undelete
			deletedID, foundDeleted, err := store.FindDeletedFileID(path)
			if err != nil {
				return fmt.Errorf("querying %s: %w", path, err)
			}
			if foundDeleted {
				// Undelete
				store.UndeleteFile(deletedID)
				fileID = deletedID
				isNew = false
			} else {
				fileID, err = store.InsertFile(path)
				if err != nil {
					return fmt.Errorf("inserting %s: %w", path, err)
				}
				isNew = true
			}
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
				id, adopted, err := insertOrAdoptScenario(store, fileID, ps)
				if err != nil {
					return fmt.Errorf("inserting scenario %q: %w", ps.Name, err)
				}
				if !adopted {
					insertions = append(insertions, tagInsertion{line: ps.Line, id: id})
				}
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
			actions, insertions, err := reconcileTrackedFile(store, fileID, pf)
			if err != nil {
				return fmt.Errorf("reconciling %s: %w", path, err)
			}

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
	allFiles, err := store.ActiveFiles()
	if err != nil {
		return fmt.Errorf("querying files: %w", err)
	}

	for _, f := range allFiles {
		if !diskPaths[f.FilePath] {
			actions, err := handleDeletedFile(store, f.ID)
			if err != nil {
				return fmt.Errorf("handling deleted file %s: %w", f.FilePath, err)
			}

			ui.DelLine(w, f.FilePath)
			fileCount++
			for _, a := range actions {
				ui.RemovedScenarioLine(w, a.id, a.name)
				scenarioCount++
			}

			store.MarkFileDeleted(f.ID)
		}
	}

	if err := reconcileStatusesFile(store); err != nil {
		return fmt.Errorf("reconciling statuses file: %w", err)
	}

	if err := syncTestLinks(store); err != nil {
		return fmt.Errorf("syncing test links: %w", err)
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
		var indent strings.Builder
		for _, ch := range scenarioLine {
			if ch == ' ' || ch == '\t' {
				indent.WriteString(string(ch))
			} else {
				break
			}
		}

		tagLine := fmt.Sprintf("%s@ft:%d", indent.String(), ins.id)

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

// reconcileStatusesFile keeps fts/statuses.csv and the statuses table in
// sync in both directions, but only for the one-time "these two have never
// met" cases — ordinary operation always writes both together via
// Store.InsertStatus, so neither direction ever fires once a project is
// caught up.
//
//   - DB empty, file has rows: fts/ft.db was deleted and recreated (a
//     rebuild). Replay the file into the table. Entries whose scenario id has
//     no matching row (removed from its file before the DB was lost) are
//     skipped rather than reconstructed — see design/STATUSES_FILE.md.
//   - DB has rows, file is empty: an existing project is adopting this
//     feature for the first time — its history predates statuses.csv.
//     Backfill the file from the table instead.
//
// A brand-new project has both empty, so this is a no-op on an ordinary
// first sync too.
func reconcileStatusesFile(store *db.Store) error {
	dbCount, err := store.CountStatuses()
	if err != nil {
		return err
	}

	fileRows, err := db.ReadStatusesFile()
	if err != nil {
		return err
	}

	switch {
	case dbCount == 0 && len(fileRows) > 0:
		for _, row := range fileRows {
			if !store.ScenarioExists(row.ScenarioID) {
				continue
			}
			if err := store.InsertStatusAt(row.ScenarioID, row.Status, row.ChangedAt); err != nil {
				return err
			}
		}
	case dbCount > 0 && len(fileRows) == 0:
		allRows, err := store.AllStatusesOrdered()
		if err != nil {
			return err
		}
		if err := db.WriteStatusesFile(allRows); err != nil {
			return err
		}
	}
	return nil
}

func syncTestLinks(store *db.Store) error {
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
	validIDs, err := store.AllScenarioIDs()
	if err != nil {
		return err
	}

	var records []db.TestLinkRecord
	for _, l := range links {
		if validIDs[l.scenarioID] {
			records = append(records, db.TestLinkRecord{
				ScenarioID: l.scenarioID,
				FilePath:   l.filePath,
				LineNumber: l.lineNumber,
			})
		}
	}

	store.ReplaceTestLinks(records)
	return nil
}
