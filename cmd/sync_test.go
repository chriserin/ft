package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chriserin/ft/internal/db"
)

func runSync(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, RunSync(&buf))
	return buf.String()
}

func TestSync_RegisterNewFile(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(""), 0o644))

	out := runSync(t)

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var filePath string
	require.NoError(t, sqlDB.QueryRow(`SELECT file_path FROM files WHERE file_path = ?`, "fts/login.ft").Scan(&filePath))
	assert.Equal(t, "fts/login.ft", filePath)
	assert.Contains(t, out, "new  fts/login.ft")
}

func TestSync_RegisterMultipleFiles(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(""), 0o644))
	require.NoError(t, os.WriteFile("fts/checkout.ft", []byte(""), 0o644))

	out := runSync(t)

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var count int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM files`).Scan(&count))
	assert.Equal(t, 2, count)
	assert.Contains(t, out, "new  fts/login.ft")
	assert.Contains(t, out, "new  fts/checkout.ft")
}

func TestSync_ShowAlreadyTrackedFile(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(""), 0o644))

	runSync(t) // first sync registers

	out := runSync(t) // second sync shows tracked

	assert.Contains(t, out, "trk  fts/login.ft")
}

func TestSync_NoFtFiles(t *testing.T) {
	inTempDir(t)
	runInit(t)

	out := runSync(t)

	assert.Contains(t, out, "synced 0 files")
}

func TestSync_FilesRecordStoresFilePath(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(""), 0o644))

	runSync(t)

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var filePath string
	require.NoError(t, sqlDB.QueryRow(`SELECT file_path FROM files WHERE file_path = ?`, "fts/login.ft").Scan(&filePath))
	assert.Equal(t, "fts/login.ft", filePath)
}

func TestSync_FilesRecordStoresTimestamps(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(""), 0o644))

	runSync(t)

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var createdAt, updatedAt string
	require.NoError(t, sqlDB.QueryRow(
		`SELECT created_at, updated_at FROM files WHERE file_path = ?`, "fts/login.ft",
	).Scan(&createdAt, &updatedAt))
	assert.NotEmpty(t, createdAt)
	assert.NotEmpty(t, updatedAt)
}

func TestSync_NonFtFilesIgnored(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/notes.txt", []byte(""), 0o644))
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(""), 0o644))

	runSync(t)

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var count int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM files WHERE file_path = ?`, "fts/notes.txt").Scan(&count))
	assert.Equal(t, 0, count)

	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM files WHERE file_path = ?`, "fts/login.ft").Scan(&count))
	assert.Equal(t, 1, count)
}

func TestSync_IsIdempotent(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(""), 0o644))

	runSync(t)
	out := runSync(t)

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var count int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM files WHERE file_path = ?`, "fts/login.ft").Scan(&count))
	assert.Equal(t, 1, count)
	assert.Contains(t, out, "trk  fts/login.ft")
}

func TestSync_SummaryLine(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(""), 0o644))

	out := runSync(t)

	assert.Contains(t, out, "synced 1 files")
}

func TestSync_WithoutInit(t *testing.T) {
	inTempDir(t)

	var buf bytes.Buffer
	err := RunSync(&buf)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "run `ft init` first")
}

func TestSync_FilesTableMigration(t *testing.T) {
	inTempDir(t)
	runInit(t)

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var name string
	err = sqlDB.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='files'`).Scan(&name)
	require.NoError(t, err)
	assert.Equal(t, "files", name)

	var version int
	require.NoError(t, sqlDB.QueryRow(`SELECT version FROM schema_version`).Scan(&version))
	assert.Equal(t, 3, version)
}

// Phase 3 tests

func TestSync_RegisterNewScenario(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	out := runSync(t)

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var name string
	require.NoError(t, sqlDB.QueryRow(`SELECT name FROM scenarios WHERE name = ?`, "User logs in").Scan(&name))
	assert.Equal(t, "User logs in", name)
	assert.Contains(t, out, "@ft:1 User logs in")

	// File name should appear above the scenario line
	fileIdx := strings.Index(out, "new  fts/login.ft")
	scenarioIdx := strings.Index(out, "@ft:1 User logs in")
	require.True(t, fileIdx >= 0, "output should contain file line")
	require.True(t, scenarioIdx >= 0, "output should contain scenario line")
	assert.True(t, fileIdx < scenarioIdx, "file line should appear before scenario line")
}

func TestSync_RegisterMultipleScenariosFromOneFile(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
    When  they log in
    Then  they see the dashboard

  Scenario: User fails login
    Given a user
    When  they enter a wrong password
    Then  they see an error
`), 0o644))

	runSync(t)

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var count int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM scenarios`).Scan(&count))
	assert.Equal(t, 2, count)

	var name1, name2 string
	require.NoError(t, sqlDB.QueryRow(`SELECT name FROM scenarios WHERE id = 1`).Scan(&name1))
	require.NoError(t, sqlDB.QueryRow(`SELECT name FROM scenarios WHERE id = 2`).Scan(&name2))
	assert.Equal(t, "User logs in", name1)
	assert.Equal(t, "User fails login", name2)
}

func TestSync_WriteFtTagToFile(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)

	data, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "@ft:1")

	// Tag should be on the line immediately above Scenario:
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, "Scenario: User logs in") {
			require.True(t, i > 0, "Scenario should not be on first line")
			assert.Contains(t, lines[i-1], "@ft:1")
			break
		}
	}
}

func TestSync_AlreadyTaggedScenarioIsSkipped(t *testing.T) {
	inTempDir(t)
	runInit(t)

	// Pre-create the scenario in the DB
	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	_, err = sqlDB.Exec(`INSERT INTO files (file_path) VALUES (?)`, "fts/login.ft")
	require.NoError(t, err)
	_, err = sqlDB.Exec(`INSERT INTO scenarios (file_id, name) VALUES (1, ?)`, "User logs in")
	require.NoError(t, err)
	sqlDB.Close()

	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  @ft:1
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)

	sqlDB, err = db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var count int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM scenarios`).Scan(&count))
	assert.Equal(t, 1, count)
}

func TestSync_ScenarioRecordStoresMetadata(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var name string
	var fileID int64
	var createdAt, updatedAt string
	require.NoError(t, sqlDB.QueryRow(
		`SELECT name, file_id, created_at, updated_at FROM scenarios WHERE id = 1`,
	).Scan(&name, &fileID, &createdAt, &updatedAt))

	assert.Equal(t, "User logs in", name)

	// Verify file_id matches the files record for fts/login.ft
	var filesFileID int64
	require.NoError(t, sqlDB.QueryRow(`SELECT id FROM files WHERE file_path = ?`, "fts/login.ft").Scan(&filesFileID))
	assert.Equal(t, filesFileID, fileID)

	assert.NotEmpty(t, createdAt)
	assert.NotEmpty(t, updatedAt)
}

func TestSync_NewScenarioOutputMarker(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	out := runSync(t)

	assert.Contains(t, out, "@ft:1 User logs in")
}

func TestSync_ScenariosFromMultipleFiles(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	require.NoError(t, os.WriteFile("fts/checkout.ft", []byte(`Feature: Checkout
  Scenario: User completes purchase
    Given a cart
`), 0o644))

	runSync(t)

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var count int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM scenarios`).Scan(&count))
	assert.Equal(t, 2, count)

	var name1, name2 string
	require.NoError(t, sqlDB.QueryRow(`SELECT name FROM scenarios WHERE name = ?`, "User logs in").Scan(&name1))
	require.NoError(t, sqlDB.QueryRow(`SELECT name FROM scenarios WHERE name = ?`, "User completes purchase").Scan(&name2))
	assert.Equal(t, "User logs in", name1)
	assert.Equal(t, "User completes purchase", name2)
}

func TestSync_RejectScenarioOutline(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario Outline: User logs in
    Given a user
`), 0o644))

	out := runSync(t)

	assert.Contains(t, out, "err  fts/login.ft")
	assert.Contains(t, out, "Scenario Outline is not supported")
}

func TestSync_RejectRule(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Rule: Business rule
    Scenario: Test
`), 0o644))

	out := runSync(t)

	assert.Contains(t, out, "err  fts/login.ft")
	assert.Contains(t, out, "Rule is not supported")
}

func TestSync_RejectExamples(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Examples: Table
    | a |
`), 0o644))

	out := runSync(t)

	assert.Contains(t, out, "err  fts/login.ft")
	assert.Contains(t, out, "Examples is not supported")
}

func TestSync_ErrorCommentWrittenToFile(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Background:
    Given setup

  Scenario Outline: User logs in
    Given a user
`), 0o644))

	runSync(t)

	data, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	content := string(data)

	lines := strings.Split(content, "\n")
	assert.True(t, strings.HasPrefix(lines[0], "# ft error:"), "first line should be an error comment")
	assert.Contains(t, lines[0], "5") // line number
}

func TestSync_FileWithErrorIsSkipped(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario Outline: Parameterized
    Given a user

  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var count int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM scenarios`).Scan(&count))
	assert.Equal(t, 0, count)
}

func TestSync_SummaryIncludesScenarioCount(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user

  Scenario: User fails login
    Given a user
`), 0o644))

	out := runSync(t)

	assert.Contains(t, out, "synced 1 files, 2 scenarios")
}

func TestSync_ParseIsIdempotent(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)
	runSync(t)

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var count int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM scenarios WHERE name = ?`, "User logs in").Scan(&count))
	assert.Equal(t, 1, count)

	// Check file has exactly one @ft tag
	data, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(data), "@ft:"))
}

func TestSync_BackgroundBlockRecognized(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Background:
    Given a registered user

  Scenario: User logs in
    When  they log in
    Then  they see the dashboard
`), 0o644))

	runSync(t)

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var count int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM scenarios WHERE name = ?`, "User logs in").Scan(&count))
	assert.Equal(t, 1, count)

	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM scenarios WHERE name = ?`, "Background").Scan(&count))
	assert.Equal(t, 0, count)
}

func TestSync_TaggedScenarioWithoutDBRecordIsReRegistered(t *testing.T) {
	inTempDir(t)
	runInit(t)

	// Pre-create the file record so the file is tracked
	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	_, err = sqlDB.Exec(`INSERT INTO files (file_path) VALUES (?)`, "fts/login.ft")
	require.NoError(t, err)
	sqlDB.Close()

	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  @ft:5
  Scenario: User logs in
    Given a user
`), 0o644))

	out := runSync(t)

	sqlDB, err = db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var name string
	require.NoError(t, sqlDB.QueryRow(`SELECT name FROM scenarios WHERE id = 5`).Scan(&name))
	assert.Equal(t, "User logs in", name)
	assert.Contains(t, out, "@ft:5 User logs in")
}

func TestSync_NewFileStripsStaleTagsAndAssignsNewIDs(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  @ft:99
  Scenario: User logs in
    Given a user
`), 0o644))

	out := runSync(t)

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	// Should have a fresh ID (1), not the stale 99
	var name string
	require.NoError(t, sqlDB.QueryRow(`SELECT name FROM scenarios WHERE id = 1`).Scan(&name))
	assert.Equal(t, "User logs in", name)

	// Stale ID should not exist
	var count int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM scenarios WHERE id = 99`).Scan(&count))
	assert.Equal(t, 0, count)

	// File should have @ft:1, not @ft:99
	data, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "@ft:1")
	assert.NotContains(t, content, "@ft:99")

	assert.Contains(t, out, "@ft:1 User logs in")
}

func TestSync_ScenariosTableMigration(t *testing.T) {
	inTempDir(t)
	runInit(t)

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var name string
	err = sqlDB.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='scenarios'`).Scan(&name)
	require.NoError(t, err)
	assert.Equal(t, "scenarios", name)

	var version int
	require.NoError(t, sqlDB.QueryRow(`SELECT version FROM schema_version`).Scan(&version))
	assert.Equal(t, 3, version)
}
