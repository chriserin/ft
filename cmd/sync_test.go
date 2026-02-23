package cmd

import (
	"bytes"
	"database/sql"
	"fmt"
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
	assert.Equal(t, 5, version)
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

func TestSync_TaggedScenarioWithoutDBRecordGetsNewID(t *testing.T) {
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

	runSync(t)

	sqlDB, err = db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	// Should have a record with name "User logs in" (any id)
	var id int
	var name string
	require.NoError(t, sqlDB.QueryRow(`SELECT id, name FROM scenarios WHERE name = ?`, "User logs in").Scan(&id, &name))
	assert.Equal(t, "User logs in", name)

	// Stale ID 5 should not be used
	var count int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM scenarios WHERE id = 5`).Scan(&count))
	assert.Equal(t, 0, count)

	// File should contain the new @ft:<id> tag, not @ft:5
	data, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, fmt.Sprintf("@ft:%d", id))
	assert.NotContains(t, content, "@ft:5")
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
	assert.Equal(t, 5, version)
}

// Phase 7 tests

// @ft:82
func TestSync_UpdateScenarioNameWhenTagMatches(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t) // first sync

	// Rename scenario but keep the tag
	data, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	updated := strings.Replace(string(data), "User logs in", "User signs in", 1)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(updated), 0o644))

	out := runSync(t) // second sync

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var name string
	require.NoError(t, sqlDB.QueryRow(`SELECT name FROM scenarios WHERE id = 1`).Scan(&name))
	assert.Equal(t, "User signs in", name)
	assert.Contains(t, out, "~")
	assert.Contains(t, out, "User signs in")
}

// @ft:83
func TestSync_UpdateScenarioContentWhenTagMatches(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)

	// Get original updated_at
	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	var origUpdatedAt string
	require.NoError(t, sqlDB.QueryRow(`SELECT updated_at FROM scenarios WHERE id = 1`).Scan(&origUpdatedAt))
	sqlDB.Close()

	// Change step content
	data, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	updated := strings.Replace(string(data), "Given a user", "Given an admin", 1)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(updated), 0o644))

	out := runSync(t)

	sqlDB, err = db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var content string
	require.NoError(t, sqlDB.QueryRow(`SELECT content FROM scenarios WHERE id = 1`).Scan(&content))
	assert.Contains(t, content, "Given an admin")
	assert.Contains(t, out, "~")
}

// @ft:84
func TestSync_ModifiedFileShowsModMarker(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)

	// Modify file
	data, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	updated := strings.Replace(string(data), "User logs in", "User signs in", 1)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(updated), 0o644))

	out := runSync(t)

	assert.Contains(t, out, "mod  fts/login.ft")
}

// @ft:85
func TestSync_UnmodifiedFileShowsTrkMarker(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)
	// Re-read the file after first sync (tags written) and sync again
	out := runSync(t)

	assert.Contains(t, out, "trk  fts/login.ft")
}

// @ft:86
func TestSync_UntaggedScenarioFallsBackToNameMatch(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)

	// Remove the tag but keep the scenario name
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	out := runSync(t)

	// Should match by name and write tag back
	data, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	assert.Contains(t, string(data), "@ft:1")

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var count int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM scenarios`).Scan(&count))
	assert.Equal(t, 1, count)

	assert.Contains(t, out, "mod  fts/login.ft")
}

// @ft:87
func TestSync_UntaggedScenarioNoNameMatchIsNew(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)

	// Read the tagged file, add a new untagged scenario
	data, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	updated := string(data) + `
  Scenario: User resets password
    Given a user
`
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(updated), 0o644))

	out := runSync(t)

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var count int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM scenarios`).Scan(&count))
	assert.Equal(t, 2, count)

	assert.Contains(t, out, "+")
	assert.Contains(t, out, "User resets password")
}

// @ft:88
func TestSync_UnknownTagFallsBackToNameMatch(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)

	// Replace tag with unknown @ft:999 but keep same name
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  @ft:999
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)

	// Should match by name and correct the tag
	data, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	assert.Contains(t, string(data), "@ft:1")
	assert.NotContains(t, string(data), "@ft:999")

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var count int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM scenarios`).Scan(&count))
	assert.Equal(t, 1, count)
}

// @ft:89
func TestSync_UnknownTagNoNameMatchIsNew(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)

	// Replace with @ft:999 and a new name
	data, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	// Replace the tag line with @ft:999 and rename the scenario
	updated := strings.Replace(string(data), "@ft:1", "@ft:999", 1)
	updated = strings.Replace(updated, "User logs in", "User registers", 1)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(updated), 0o644))

	out := runSync(t)

	assert.Contains(t, out, "+")
	assert.Contains(t, out, "User registers")

	// The stale @ft:999 should be replaced with a new ID
	data, err = os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	assert.NotContains(t, string(data), "@ft:999")
}

// @ft:90
func TestSync_RemovedScenarioWithHistoryGetsRemovedStatus(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)

	// Add a status to the scenario
	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	_, err = sqlDB.Exec(`INSERT INTO statuses (scenario_id, status) VALUES (1, 'pass')`)
	require.NoError(t, err)
	sqlDB.Close()

	// Remove the scenario from the file
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
`), 0o644))

	out := runSync(t)

	sqlDB, err = db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	// Scenario should still exist
	var count int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM scenarios WHERE id = 1`).Scan(&count))
	assert.Equal(t, 1, count)

	// Should have a "removed" status
	var status string
	require.NoError(t, sqlDB.QueryRow(`SELECT status FROM statuses WHERE scenario_id = 1 ORDER BY id DESC LIMIT 1`).Scan(&status))
	assert.Equal(t, "removed", status)

	assert.Contains(t, out, "-")
	assert.Contains(t, out, "User logs in")
}

// @ft:91
func TestSync_RemovedScenarioNoHistoryIsDeleted(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)

	// Remove the scenario (no status history)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
`), 0o644))

	runSync(t)

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var count int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM scenarios WHERE id = 1`).Scan(&count))
	assert.Equal(t, 0, count)
}

// @ft:92
func TestSync_DeletedFileShowsDelMarker(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)

	// Delete the file
	require.NoError(t, os.Remove("fts/login.ft"))

	out := runSync(t)

	assert.Contains(t, out, "del  fts/login.ft")
}

// @ft:93
func TestSync_DeletedFileMarkedInDatabase(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)

	require.NoError(t, os.Remove("fts/login.ft"))

	runSync(t)

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var deleted bool
	require.NoError(t, sqlDB.QueryRow(`SELECT deleted FROM files WHERE file_path = ?`, "fts/login.ft").Scan(&deleted))
	assert.True(t, deleted)

	// Scenario should be deleted (no history)
	var count int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM scenarios WHERE id = 1`).Scan(&count))
	assert.Equal(t, 0, count)
}

// @ft:94
func TestSync_DeletedFileWithHistoryPreservesScenario(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)

	// Add status history
	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	_, err = sqlDB.Exec(`INSERT INTO statuses (scenario_id, status) VALUES (1, 'pass')`)
	require.NoError(t, err)
	sqlDB.Close()

	require.NoError(t, os.Remove("fts/login.ft"))

	runSync(t)

	sqlDB, err = db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var deleted bool
	require.NoError(t, sqlDB.QueryRow(`SELECT deleted FROM files WHERE file_path = ?`, "fts/login.ft").Scan(&deleted))
	assert.True(t, deleted)

	// Scenario preserved
	var count int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM scenarios WHERE id = 1`).Scan(&count))
	assert.Equal(t, 1, count)

	// "removed" status added
	var status string
	require.NoError(t, sqlDB.QueryRow(`SELECT status FROM statuses WHERE scenario_id = 1 ORDER BY id DESC LIMIT 1`).Scan(&status))
	assert.Equal(t, "removed", status)
}

// @ft:95
func TestSync_AlreadyDeletedFileSkipped(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)

	// Mark file as deleted in DB
	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	_, err = sqlDB.Exec(`UPDATE files SET deleted = TRUE WHERE file_path = ?`, "fts/login.ft")
	require.NoError(t, err)
	sqlDB.Close()

	// Remove the file
	require.NoError(t, os.Remove("fts/login.ft"))

	out := runSync(t)

	// Should not show del line for already-deleted file
	assert.NotContains(t, out, "del  fts/login.ft")
}

// @ft:96
func TestSync_ContentStoredOnSync(t *testing.T) {
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

	var content sql.NullString
	require.NoError(t, sqlDB.QueryRow(`SELECT content FROM scenarios WHERE id = 1`).Scan(&content))
	assert.True(t, content.Valid)
	assert.NotEmpty(t, content.String)
}

// @ft:97
func TestSync_ContentUpdatedOnChange(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)

	// Change the content
	data, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	updated := strings.Replace(string(data), "Given a user", "Given an admin", 1)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(updated), 0o644))

	runSync(t)

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var content string
	require.NoError(t, sqlDB.QueryRow(`SELECT content FROM scenarios WHERE id = 1`).Scan(&content))
	assert.Contains(t, content, "Given an admin")
}

// @ft:100
func TestSync_SummaryCountsModifiedFiles(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)

	// Modify
	data, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	updated := strings.Replace(string(data), "User logs in", "User signs in", 1)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(updated), 0o644))

	out := runSync(t)

	assert.Contains(t, out, "synced 1 files, 1 scenarios")
}

// @ft:101
func TestSync_MultipleScenariosMixedChanges(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user

  Scenario: User fails login
    Given a user
    When  they enter a wrong password
`), 0o644))

	runSync(t)

	// Read file with tags
	data, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)

	// Rename first scenario, remove second, add a new one
	updated := strings.Replace(string(data), "User logs in", "User signs in", 1)
	// Remove the second scenario (everything from its @ft tag to end)
	lines := strings.Split(updated, "\n")
	var kept []string
	skipRemaining := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "@ft:2") {
			skipRemaining = true
			continue
		}
		if skipRemaining && strings.HasPrefix(trimmed, "Scenario:") {
			skipRemaining = true // skip Scenario: User fails login
			continue
		}
		if skipRemaining && (strings.HasPrefix(trimmed, "Given") || strings.HasPrefix(trimmed, "When") || strings.HasPrefix(trimmed, "Then") || strings.HasPrefix(trimmed, "And")) {
			continue
		}
		if skipRemaining && trimmed == "" {
			skipRemaining = false
			continue
		}
		kept = append(kept, line)
	}
	result := strings.Join(kept, "\n") + "\n  Scenario: User resets password\n    Given a user\n"
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(result), 0o644))

	out := runSync(t)

	assert.Contains(t, out, "mod  fts/login.ft")
	assert.Contains(t, out, "~") // modified
	assert.Contains(t, out, "-") // removed
	assert.Contains(t, out, "+") // new
}

// @ft:102
func TestSync_ChangeDetectionIsIdempotent(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)
	runSync(t) // second sync populates content

	// Read file state and DB state before third sync
	dataBefore, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	var updatedAtBefore string
	require.NoError(t, sqlDB.QueryRow(`SELECT updated_at FROM scenarios WHERE id = 1`).Scan(&updatedAtBefore))
	sqlDB.Close()

	out := runSync(t) // third sync

	// File should not change
	dataAfter, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	assert.Equal(t, string(dataBefore), string(dataAfter))

	// Should show trk, not mod
	assert.Contains(t, out, "trk  fts/login.ft")
	assert.NotContains(t, out, "mod  fts/login.ft")
}

// @ft:106
func TestSync_AlreadyRemovedScenarioNotReRemoved(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)

	// Add status history so the scenario is preserved on removal
	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	_, err = sqlDB.Exec(`INSERT INTO statuses (scenario_id, status) VALUES (1, 'pass')`)
	require.NoError(t, err)
	sqlDB.Close()

	// Remove the scenario from the file
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
`), 0o644))

	runSync(t) // first sync after removal — adds "removed" status

	sqlDB, err = db.Open("fts/ft.db")
	require.NoError(t, err)
	var countAfterFirst int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM statuses WHERE scenario_id = 1 AND status = 'removed'`).Scan(&countAfterFirst))
	assert.Equal(t, 1, countAfterFirst)
	sqlDB.Close()

	out := runSync(t) // second sync after removal — should NOT add another "removed"

	sqlDB, err = db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var countAfterSecond int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM statuses WHERE scenario_id = 1 AND status = 'removed'`).Scan(&countAfterSecond))
	assert.Equal(t, 1, countAfterSecond, "should not add duplicate removed status")

	// Should not show the removed scenario in output again
	assert.NotContains(t, out, "User logs in")
}

// @ft:107
func TestSync_RemovedScenarioReferencedByTagIsRestored(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)

	// Add status history so the scenario is preserved on removal
	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	_, err = sqlDB.Exec(`INSERT INTO statuses (scenario_id, status) VALUES (1, 'pass')`)
	require.NoError(t, err)
	sqlDB.Close()

	// Remove the scenario from the file
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
`), 0o644))

	runSync(t) // adds "removed" status

	// Re-add the scenario with the same @ft tag
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  @ft:1
  Scenario: User logs in
    Given a user
`), 0o644))

	out := runSync(t)

	sqlDB, err = db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	// Scenario should still exist
	var count int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM scenarios WHERE id = 1`).Scan(&count))
	assert.Equal(t, 1, count)

	// Should have a "restored" status as the latest
	var latestStatus string
	require.NoError(t, sqlDB.QueryRow(`SELECT status FROM statuses WHERE scenario_id = 1 ORDER BY id DESC LIMIT 1`).Scan(&latestStatus))
	assert.Equal(t, "restored", latestStatus)

	// Should only have one "removed" status (not re-removed)
	var removedCount int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM statuses WHERE scenario_id = 1 AND status = 'removed'`).Scan(&removedCount))
	assert.Equal(t, 1, removedCount)

	// Output should show + indicator for restored scenario
	assert.Contains(t, out, "+")
	assert.Contains(t, out, "@ft:1 User logs in")
}
