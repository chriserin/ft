// Package dbtest provides direct database access for test setup and
// assertions. It is kept separate from db.Store so that test-only queries
// never leak into the production data-access API used by business logic.
package dbtest

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chriserin/ft/internal/db"
)

// Fixture wraps a *sql.DB with helpers for seeding and inspecting database
// state directly from tests.
type Fixture struct {
	t     *testing.T
	sqlDB *sql.DB
}

// Open opens the sqlite database at path and wraps it in a Fixture. The
// underlying connection is closed automatically when the test completes,
// unless closed early via Close.
func Open(t *testing.T, path string) *Fixture {
	t.Helper()
	sqlDB, err := db.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	return &Fixture{t: t, sqlDB: sqlDB}
}

// Close releases the underlying database connection early, e.g. to avoid
// holding it open across a subsequent RunSync call.
func (f *Fixture) Close() {
	f.sqlDB.Close()
}

// JournalMode returns the database's current journal mode.
func (f *Fixture) JournalMode() string {
	f.t.Helper()
	var mode string
	require.NoError(f.t, f.sqlDB.QueryRow(`PRAGMA journal_mode`).Scan(&mode))
	return mode
}

// SchemaVersion returns the currently applied migration version.
func (f *Fixture) SchemaVersion() int {
	f.t.Helper()
	var version int
	require.NoError(f.t, f.sqlDB.QueryRow(`SELECT version FROM schema_version`).Scan(&version))
	return version
}

// TableExists reports whether a table with the given name exists.
func (f *Fixture) TableExists(name string) bool {
	f.t.Helper()
	var found string
	err := f.sqlDB.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name = ?`, name).Scan(&found)
	return err == nil
}

// InsertFile inserts a bare file record and returns nothing; used to seed
// fixtures that don't go through RunSync.
func (f *Fixture) InsertFile(path string) {
	f.t.Helper()
	_, err := f.sqlDB.Exec(`INSERT INTO files (file_path) VALUES (?)`, path)
	require.NoError(f.t, err)
}

// MarkFileDeletedByPath flags a file as deleted, identified by path.
func (f *Fixture) MarkFileDeletedByPath(path string) {
	f.t.Helper()
	_, err := f.sqlDB.Exec(`UPDATE files SET deleted = TRUE WHERE file_path = ?`, path)
	require.NoError(f.t, err)
}

// CountFiles returns the total number of file records.
func (f *Fixture) CountFiles() int {
	f.t.Helper()
	var count int
	require.NoError(f.t, f.sqlDB.QueryRow(`SELECT COUNT(*) FROM files`).Scan(&count))
	return count
}

// CountFilesWithPath returns the number of file records with the given path.
func (f *Fixture) CountFilesWithPath(path string) int {
	f.t.Helper()
	var count int
	require.NoError(f.t, f.sqlDB.QueryRow(`SELECT COUNT(*) FROM files WHERE file_path = ?`, path).Scan(&count))
	return count
}

// FilePath returns the file_path column for the file matching path,
// confirming the record exists.
func (f *Fixture) FilePath(path string) string {
	f.t.Helper()
	var filePath string
	require.NoError(f.t, f.sqlDB.QueryRow(`SELECT file_path FROM files WHERE file_path = ?`, path).Scan(&filePath))
	return filePath
}

// FileTimestamps returns the created_at and updated_at columns for a file.
func (f *Fixture) FileTimestamps(path string) (createdAt, updatedAt string) {
	f.t.Helper()
	require.NoError(f.t, f.sqlDB.QueryRow(
		`SELECT created_at, updated_at FROM files WHERE file_path = ?`, path,
	).Scan(&createdAt, &updatedAt))
	return createdAt, updatedAt
}

// FileID returns the id of the file record with the given path.
func (f *Fixture) FileID(path string) int64 {
	f.t.Helper()
	var id int64
	require.NoError(f.t, f.sqlDB.QueryRow(`SELECT id FROM files WHERE file_path = ?`, path).Scan(&id))
	return id
}

// FileDeleted reports whether the file's deleted flag is set.
func (f *Fixture) FileDeleted(path string) bool {
	f.t.Helper()
	var deleted bool
	require.NoError(f.t, f.sqlDB.QueryRow(`SELECT deleted FROM files WHERE file_path = ?`, path).Scan(&deleted))
	return deleted
}

// InsertScenarioStub inserts a scenario with only a name, bypassing the
// normal sync flow, for tests that pre-seed state.
func (f *Fixture) InsertScenarioStub(fileID int64, name string) {
	f.t.Helper()
	_, err := f.sqlDB.Exec(`INSERT INTO scenarios (file_id, name) VALUES (?, ?)`, fileID, name)
	require.NoError(f.t, err)
}

// InsertScenarioWithID inserts a scenario using an explicit id, for tests
// that need to pre-claim a specific id (e.g. simulating an id collision).
func (f *Fixture) InsertScenarioWithID(id, fileID int64, name string) {
	f.t.Helper()
	_, err := f.sqlDB.Exec(`INSERT INTO scenarios (id, file_id, name) VALUES (?, ?, ?)`, id, fileID, name)
	require.NoError(f.t, err)
}

// CountScenarios returns the total number of scenario records.
func (f *Fixture) CountScenarios() int {
	f.t.Helper()
	var count int
	require.NoError(f.t, f.sqlDB.QueryRow(`SELECT COUNT(*) FROM scenarios`).Scan(&count))
	return count
}

// CountScenariosByID returns the number of scenarios with the given ID (0 or 1).
func (f *Fixture) CountScenariosByID(id int64) int {
	f.t.Helper()
	var count int
	require.NoError(f.t, f.sqlDB.QueryRow(`SELECT COUNT(*) FROM scenarios WHERE id = ?`, id).Scan(&count))
	return count
}

// CountScenariosByName returns the number of scenarios with the given name.
func (f *Fixture) CountScenariosByName(name string) int {
	f.t.Helper()
	var count int
	require.NoError(f.t, f.sqlDB.QueryRow(`SELECT COUNT(*) FROM scenarios WHERE name = ?`, name).Scan(&count))
	return count
}

// ScenarioName returns the name of the scenario with the given ID.
func (f *Fixture) ScenarioName(id int64) string {
	f.t.Helper()
	var name string
	require.NoError(f.t, f.sqlDB.QueryRow(`SELECT name FROM scenarios WHERE id = ?`, id).Scan(&name))
	return name
}

// ScenarioByName returns the id and name of the scenario matching name,
// confirming the record exists.
func (f *Fixture) ScenarioByName(name string) (id int64, foundName string) {
	f.t.Helper()
	require.NoError(f.t, f.sqlDB.QueryRow(`SELECT id, name FROM scenarios WHERE name = ?`, name).Scan(&id, &foundName))
	return id, foundName
}

// ScenarioMeta returns a scenario's name, file_id, created_at, and updated_at.
func (f *Fixture) ScenarioMeta(id int64) (name string, fileID int64, createdAt, updatedAt string) {
	f.t.Helper()
	require.NoError(f.t, f.sqlDB.QueryRow(
		`SELECT name, file_id, created_at, updated_at FROM scenarios WHERE id = ?`, id,
	).Scan(&name, &fileID, &createdAt, &updatedAt))
	return name, fileID, createdAt, updatedAt
}

// ScenarioUpdatedAt returns the updated_at column for a scenario.
func (f *Fixture) ScenarioUpdatedAt(id int64) string {
	f.t.Helper()
	var updatedAt string
	require.NoError(f.t, f.sqlDB.QueryRow(`SELECT updated_at FROM scenarios WHERE id = ?`, id).Scan(&updatedAt))
	return updatedAt
}

// ScenarioContent returns the (possibly NULL) content column for a scenario.
func (f *Fixture) ScenarioContent(id int64) sql.NullString {
	f.t.Helper()
	var content sql.NullString
	require.NoError(f.t, f.sqlDB.QueryRow(`SELECT content FROM scenarios WHERE id = ?`, id).Scan(&content))
	return content
}

// InsertStatus records a new status for a scenario.
func (f *Fixture) InsertStatus(scenarioID int64, status string) {
	f.t.Helper()
	_, err := f.sqlDB.Exec(`INSERT INTO statuses (scenario_id, status) VALUES (?, ?)`, scenarioID, status)
	require.NoError(f.t, err)
}

// Status returns a scenario's status, assuming exactly one status row exists.
func (f *Fixture) Status(scenarioID int64) string {
	f.t.Helper()
	var status string
	require.NoError(f.t, f.sqlDB.QueryRow(`SELECT status FROM statuses WHERE scenario_id = ?`, scenarioID).Scan(&status))
	return status
}

// LatestStatusByID returns the most recently inserted status (by id) for a scenario.
func (f *Fixture) LatestStatusByID(scenarioID int64) string {
	f.t.Helper()
	var status string
	require.NoError(f.t, f.sqlDB.QueryRow(
		`SELECT status FROM statuses WHERE scenario_id = ? ORDER BY id DESC LIMIT 1`, scenarioID,
	).Scan(&status))
	return status
}

// LatestStatusByChangedAt returns the most recently changed status for a scenario.
func (f *Fixture) LatestStatusByChangedAt(scenarioID int64) string {
	f.t.Helper()
	var status string
	require.NoError(f.t, f.sqlDB.QueryRow(
		`SELECT status FROM statuses WHERE scenario_id = ? ORDER BY changed_at DESC, id DESC LIMIT 1`, scenarioID,
	).Scan(&status))
	return status
}

// CountStatuses returns the number of status rows for a scenario.
func (f *Fixture) CountStatuses(scenarioID int64) int {
	f.t.Helper()
	var count int
	require.NoError(f.t, f.sqlDB.QueryRow(`SELECT COUNT(*) FROM statuses WHERE scenario_id = ?`, scenarioID).Scan(&count))
	return count
}

// CountStatusesByStatus returns the number of status rows for a scenario matching a specific status.
func (f *Fixture) CountStatusesByStatus(scenarioID int64, status string) int {
	f.t.Helper()
	var count int
	require.NoError(f.t, f.sqlDB.QueryRow(
		`SELECT COUNT(*) FROM statuses WHERE scenario_id = ? AND status = ?`, scenarioID, status,
	).Scan(&count))
	return count
}

// CountTestLinks returns the total number of test_links rows.
func (f *Fixture) CountTestLinks() int {
	f.t.Helper()
	var count int
	require.NoError(f.t, f.sqlDB.QueryRow(`SELECT COUNT(*) FROM test_links`).Scan(&count))
	return count
}

// CountTestLinksForScenario returns the number of test_links rows for a scenario.
func (f *Fixture) CountTestLinksForScenario(scenarioID int64) int {
	f.t.Helper()
	var count int
	require.NoError(f.t, f.sqlDB.QueryRow(`SELECT COUNT(*) FROM test_links WHERE scenario_id = ?`, scenarioID).Scan(&count))
	return count
}

// TestLink returns the file_path and line_number of a scenario's (first) test link.
func (f *Fixture) TestLink(scenarioID int64) (filePath string, lineNumber int) {
	f.t.Helper()
	require.NoError(f.t, f.sqlDB.QueryRow(
		`SELECT file_path, line_number FROM test_links WHERE scenario_id = ?`, scenarioID,
	).Scan(&filePath, &lineNumber))
	return filePath, lineNumber
}
