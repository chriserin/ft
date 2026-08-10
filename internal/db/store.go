package db

import (
	"database/sql"
	"time"
)

// sqliteTimeLayout matches the textual format SQLite's own datetime('now')
// produces, so timestamps generated in Go and stored via explicit parameters
// round-trip identically to ones written via the column's SQL default.
const sqliteTimeLayout = "2006-01-02 15:04:05"

// Store wraps a *sql.DB and exposes the application's data-access operations,
// keeping raw SQL out of the business logic in cmd.
type Store struct {
	db *sql.DB
}

// NewStore wraps an existing *sql.DB in a Store.
func NewStore(sqlDB *sql.DB) *Store {
	return &Store{db: sqlDB}
}

// OpenStore opens the database at path and returns it wrapped in a Store.
func OpenStore(path string) (*Store, error) {
	sqlDB, err := Open(path)
	if err != nil {
		return nil, err
	}
	return NewStore(sqlDB), nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

type ScenarioListRow struct {
	ID       int64
	FilePath string
	Name     string
	Status   string
}

type ScenarioDetail struct {
	ID       int64
	Name     string
	FilePath string
	Content  sql.NullString
}

type StatusEntry struct {
	Status    string
	ChangedAt time.Time
}

type StatusCount struct {
	Status string
	Count  int
}

type TestLink struct {
	FilePath   string
	LineNumber int
}

type ScenarioRecord struct {
	ID      int64
	Name    string
	Content sql.NullString
}

type FileRecord struct {
	ID       int64
	FilePath string
}

type TestLinkRecord struct {
	ScenarioID int64
	FilePath   string
	LineNumber int
}

// IsTested reports whether a scenario has any linked tests.
func (s *Store) IsTested(scenarioID int64) bool {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM test_links WHERE scenario_id = ?`, scenarioID).Scan(&count)
	return err == nil && count > 0
}

// ListScenarios returns all scenarios joined with their file path and current status.
func (s *Store) ListScenarios() ([]ScenarioListRow, error) {
	rows, err := s.db.Query(`
		SELECT s.id, f.file_path, s.name,
			COALESCE(
				(SELECT status FROM statuses WHERE scenario_id = s.id ORDER BY changed_at DESC, id DESC LIMIT 1),
				'no-activity'
			) AS current_status
		FROM scenarios s
		JOIN files f ON s.file_id = f.id
		ORDER BY f.file_path, s.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ScenarioListRow
	for rows.Next() {
		var r ScenarioListRow
		if err := rows.Scan(&r.ID, &r.FilePath, &r.Name, &r.Status); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// ScenarioDetail fetches a scenario's core fields along with its file path.
func (s *Store) ScenarioDetail(id int64) (ScenarioDetail, error) {
	var d ScenarioDetail
	err := s.db.QueryRow(`
		SELECT s.id, s.name, f.file_path, s.content
		FROM scenarios s
		JOIN files f ON s.file_id = f.id
		WHERE s.id = ?
	`, id).Scan(&d.ID, &d.Name, &d.FilePath, &d.Content)
	return d, err
}

// ScenarioBasic fetches a scenario's id, name, and created_at.
func (s *Store) ScenarioBasic(id int64) (name string, createdAt time.Time, err error) {
	err = s.db.QueryRow(`SELECT name, created_at FROM scenarios WHERE id = ?`, id).Scan(&name, &createdAt)
	return
}

// ScenarioExists reports whether a scenario with the given ID exists.
func (s *Store) ScenarioExists(id int64) bool {
	var existingID int64
	err := s.db.QueryRow(`SELECT id FROM scenarios WHERE id = ?`, id).Scan(&existingID)
	return err == nil
}

// CurrentStatus returns the most recently changed status for a scenario.
func (s *Store) CurrentStatus(scenarioID int64) (string, error) {
	var status string
	err := s.db.QueryRow(`SELECT status FROM statuses WHERE scenario_id = ? ORDER BY changed_at DESC, id DESC LIMIT 1`, scenarioID).Scan(&status)
	return status, err
}

// LatestInsertedStatus returns the most recently inserted status for a scenario.
func (s *Store) LatestInsertedStatus(scenarioID int64) (string, error) {
	var status string
	err := s.db.QueryRow(`SELECT status FROM statuses WHERE scenario_id = ? ORDER BY id DESC LIMIT 1`, scenarioID).Scan(&status)
	return status, err
}

// LatestInsertedStatusIsRemoved reports whether a scenario's most recently inserted status is "removed".
func (s *Store) LatestInsertedStatusIsRemoved(scenarioID int64) bool {
	status, err := s.LatestInsertedStatus(scenarioID)
	return err == nil && status == "removed"
}

// HasStatusHistory reports whether a scenario has any status entries.
func (s *Store) HasStatusHistory(scenarioID int64) bool {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM statuses WHERE scenario_id = ?`, scenarioID).Scan(&count)
	return err == nil && count > 0
}

// InsertStatus records a new status for a scenario and appends the same
// event to the statuses file (see design/STATUSES_FILE.md).
func (s *Store) InsertStatus(scenarioID int64, status string) error {
	changedAt := time.Now().UTC().Format(sqliteTimeLayout)
	if err := s.InsertStatusAt(scenarioID, status, changedAt); err != nil {
		return err
	}
	return appendStatusRow(scenarioID, status, changedAt)
}

// InsertStatusAt records a status with an explicit changed_at, without
// touching the statuses file. Used when replaying the statuses file itself
// during a rebuild, so replay doesn't re-append what it just read.
func (s *Store) InsertStatusAt(scenarioID int64, status, changedAt string) error {
	_, err := s.db.Exec(`INSERT INTO statuses (scenario_id, status, changed_at) VALUES (?, ?, ?)`, scenarioID, status, changedAt)
	return err
}

// CountStatuses returns the total number of status records in the database.
func (s *Store) CountStatuses() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM statuses`).Scan(&count)
	return count, err
}

// AllStatusesOrdered returns every status record across all scenarios, in
// chronological order. Used to backfill the statuses file the first time
// it's created for a project that already has status history in the DB
// (e.g. an existing project adopting this feature).
//
// changed_at is scanned into time.Time, not string: the sqlite driver
// normalizes DATETIME columns to RFC3339 when scanning directly into a
// string, which would produce timestamps in a different format than
// InsertStatus writes (sqliteTimeLayout). Reformatting explicitly here keeps
// backfilled rows visually consistent with rows written going forward.
func (s *Store) AllStatusesOrdered() ([]StatusRow, error) {
	rows, err := s.db.Query(`SELECT scenario_id, status, changed_at FROM statuses ORDER BY changed_at ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []StatusRow
	for rows.Next() {
		var scenarioID int64
		var status string
		var changedAt time.Time
		if err := rows.Scan(&scenarioID, &status, &changedAt); err != nil {
			return nil, err
		}
		result = append(result, StatusRow{
			ScenarioID: scenarioID,
			Status:     status,
			ChangedAt:  changedAt.UTC().Format(sqliteTimeLayout),
		})
	}
	return result, rows.Err()
}

// StatusHistory returns all status entries for a scenario, most recent first.
func (s *Store) StatusHistory(scenarioID int64) ([]StatusEntry, error) {
	rows, err := s.db.Query(`SELECT status, changed_at FROM statuses WHERE scenario_id = ? ORDER BY changed_at DESC, id DESC`, scenarioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []StatusEntry
	for rows.Next() {
		var e StatusEntry
		if err := rows.Scan(&e.Status, &e.ChangedAt); err != nil {
			return nil, err
		}
		history = append(history, e)
	}
	return history, rows.Err()
}

// TestLinks returns the test links for a scenario.
func (s *Store) TestLinks(scenarioID int64) ([]TestLink, error) {
	rows, err := s.db.Query(`SELECT file_path, line_number FROM test_links WHERE scenario_id = ? ORDER BY file_path, line_number`, scenarioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []TestLink
	for rows.Next() {
		var l TestLink
		if err := rows.Scan(&l.FilePath, &l.LineNumber); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// CountScenarios returns the total number of scenarios.
func (s *Store) CountScenarios() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM scenarios`).Scan(&count)
	return count, err
}

// StatusCounts returns the count of scenarios grouped by current status.
func (s *Store) StatusCounts() ([]StatusCount, error) {
	rows, err := s.db.Query(`
		SELECT COALESCE(
			(SELECT status FROM statuses WHERE scenario_id = s.id ORDER BY changed_at DESC, id DESC LIMIT 1),
			'no-activity'
		) AS current_status, COUNT(*) AS cnt
		FROM scenarios s
		GROUP BY current_status
		ORDER BY CASE WHEN current_status = 'no-activity' THEN 1 ELSE 0 END, cnt DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var counts []StatusCount
	for rows.Next() {
		var c StatusCount
		if err := rows.Scan(&c.Status, &c.Count); err != nil {
			return nil, err
		}
		counts = append(counts, c)
	}
	return counts, rows.Err()
}

// ScenariosByFile returns the scenarios (id, name, content) belonging to a file.
func (s *Store) ScenariosByFile(fileID int64) (map[int64]ScenarioRecord, error) {
	rows, err := s.db.Query(`SELECT id, name, content FROM scenarios WHERE file_id = ?`, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]ScenarioRecord)
	for rows.Next() {
		var r ScenarioRecord
		if err := rows.Scan(&r.ID, &r.Name, &r.Content); err != nil {
			return nil, err
		}
		result[r.ID] = r
	}
	return result, rows.Err()
}

// UpdateScenarioNameContent updates a scenario's name and content.
func (s *Store) UpdateScenarioNameContent(id int64, name, content string) {
	s.db.Exec(`UPDATE scenarios SET name = ?, content = ?, updated_at = datetime('now') WHERE id = ?`, name, content, id)
}

// UpdateScenarioContent updates a scenario's content only.
func (s *Store) UpdateScenarioContent(id int64, content string) {
	s.db.Exec(`UPDATE scenarios SET content = ?, updated_at = datetime('now') WHERE id = ?`, content, id)
}

// InsertScenario creates a new scenario and returns its ID.
func (s *Store) InsertScenario(fileID int64, name, content string) (int64, error) {
	result, err := s.db.Exec(`INSERT INTO scenarios (file_id, name, content) VALUES (?, ?, ?)`, fileID, name, content)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// InsertScenarioWithID creates a new scenario using an explicit id, adopted
// from an existing @ft:<id> tag on a scenario in a file not yet tracked in
// the DB (e.g. after a rebuild). Callers must confirm via ScenarioExists that
// the id isn't already claimed before calling this.
func (s *Store) InsertScenarioWithID(id, fileID int64, name, content string) error {
	_, err := s.db.Exec(`INSERT INTO scenarios (id, file_id, name, content) VALUES (?, ?, ?, ?)`, id, fileID, name, content)
	return err
}

// DeleteScenario removes a scenario by ID.
func (s *Store) DeleteScenario(id int64) {
	s.db.Exec(`DELETE FROM scenarios WHERE id = ?`, id)
}

// FindActiveFileID looks up the ID of a non-deleted file by path.
func (s *Store) FindActiveFileID(path string) (int64, bool, error) {
	var fileID int64
	err := s.db.QueryRow(`SELECT id FROM files WHERE file_path = ? AND deleted = FALSE`, path).Scan(&fileID)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return fileID, true, nil
}

// FindDeletedFileID looks up the ID of a deleted file by path.
func (s *Store) FindDeletedFileID(path string) (int64, bool, error) {
	var fileID int64
	err := s.db.QueryRow(`SELECT id FROM files WHERE file_path = ? AND deleted = TRUE`, path).Scan(&fileID)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return fileID, true, nil
}

// UndeleteFile clears the deleted flag on a file.
func (s *Store) UndeleteFile(id int64) {
	s.db.Exec(`UPDATE files SET deleted = FALSE, updated_at = datetime('now') WHERE id = ?`, id)
}

// InsertFile creates a new file record and returns its ID.
func (s *Store) InsertFile(path string) (int64, error) {
	result, err := s.db.Exec(`INSERT INTO files (file_path) VALUES (?)`, path)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// ActiveFiles returns all non-deleted files.
func (s *Store) ActiveFiles() ([]FileRecord, error) {
	rows, err := s.db.Query(`SELECT id, file_path FROM files WHERE deleted = FALSE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []FileRecord
	for rows.Next() {
		var f FileRecord
		if err := rows.Scan(&f.ID, &f.FilePath); err != nil {
			continue
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// MarkFileDeleted flags a file as deleted.
func (s *Store) MarkFileDeleted(id int64) {
	s.db.Exec(`UPDATE files SET deleted = TRUE, updated_at = datetime('now') WHERE id = ?`, id)
}

// AllScenarioIDs returns the set of every scenario ID currently in the database.
func (s *Store) AllScenarioIDs() (map[int64]bool, error) {
	validIDs := make(map[int64]bool)
	rows, err := s.db.Query(`SELECT id FROM scenarios`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		validIDs[id] = true
	}
	return validIDs, rows.Err()
}

// ReplaceTestLinks atomically replaces the full contents of the test_links table.
func (s *Store) ReplaceTestLinks(links []TestLinkRecord) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	if _, err := tx.Exec(`DELETE FROM test_links`); err != nil {
		tx.Rollback()
		return err
	}

	stmt, err := tx.Prepare(`INSERT INTO test_links (scenario_id, file_path, line_number) VALUES (?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, l := range links {
		stmt.Exec(l.ScenarioID, l.FilePath, l.LineNumber)
	}

	return tx.Commit()
}
