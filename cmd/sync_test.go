package cmd

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chriserin/ft/internal/db/dbtest"
)

func runSync(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, RunSync(&buf))
	return buf.String()
}

// @ft:9
func TestSync_RegisterNewFile(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(""), 0o644))

	out := runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, "fts/login.ft", fx.FilePath("fts/login.ft"))
	assert.Contains(t, out, "new  fts/login.ft")
}

// @ft:10
func TestSync_RegisterMultipleFiles(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(""), 0o644))
	require.NoError(t, os.WriteFile("fts/checkout.ft", []byte(""), 0o644))

	out := runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 2, fx.CountFiles())
	assert.Contains(t, out, "new  fts/login.ft")
	assert.Contains(t, out, "new  fts/checkout.ft")
}

// @ft:11
func TestSync_ShowAlreadyTrackedFile(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(""), 0o644))

	runSync(t) // first sync registers

	out := runSync(t) // second sync shows tracked

	assert.Contains(t, out, "trk  fts/login.ft")
}

// @ft:12
func TestSync_NoFtFiles(t *testing.T) {
	inTempDir(t)
	runInit(t)

	out := runSync(t)

	assert.Contains(t, out, "synced 0 files")
}

// @ft:13
func TestSync_FilesRecordStoresFilePath(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(""), 0o644))

	runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, "fts/login.ft", fx.FilePath("fts/login.ft"))
}

// @ft:14
func TestSync_FilesRecordStoresTimestamps(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(""), 0o644))

	runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	createdAt, updatedAt := fx.FileTimestamps("fts/login.ft")
	assert.NotEmpty(t, createdAt)
	assert.NotEmpty(t, updatedAt)
}

// @ft:15
func TestSync_NonFtFilesIgnored(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/notes.txt", []byte(""), 0o644))
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(""), 0o644))

	runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 0, fx.CountFilesWithPath("fts/notes.txt"))
	assert.Equal(t, 1, fx.CountFilesWithPath("fts/login.ft"))
}

// @ft:16
func TestSync_IsIdempotent(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(""), 0o644))

	runSync(t)
	out := runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 1, fx.CountFilesWithPath("fts/login.ft"))
	assert.Contains(t, out, "trk  fts/login.ft")
}

// @ft:17
func TestSync_SummaryLine(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(""), 0o644))

	out := runSync(t)

	assert.Contains(t, out, "synced 1 files")
}

// @ft:18
func TestSync_WithoutInit(t *testing.T) {
	inTempDir(t)

	var buf bytes.Buffer
	err := RunSync(&buf)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "run `ft init` first")
}

// @ft:19
func TestSync_FilesTableMigration(t *testing.T) {
	inTempDir(t)
	runInit(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.True(t, fx.TableExists("files"))
	assert.Equal(t, 6, fx.SchemaVersion())
}

// Phase 3 tests

// @ft:20
func TestSync_RegisterNewScenario(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	out := runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 1, fx.CountScenariosByName("User logs in"))
	assert.Contains(t, out, "@ft:1 User logs in")

	// File name should appear above the scenario line
	fileIdx := strings.Index(out, "new  fts/login.ft")
	scenarioIdx := strings.Index(out, "@ft:1 User logs in")
	require.True(t, fileIdx >= 0, "output should contain file line")
	require.True(t, scenarioIdx >= 0, "output should contain scenario line")
	assert.True(t, fileIdx < scenarioIdx, "file line should appear before scenario line")
}

// @ft:21
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

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 2, fx.CountScenarios())
	assert.Equal(t, "User logs in", fx.ScenarioName(1))
	assert.Equal(t, "User fails login", fx.ScenarioName(2))
}

// @ft:22
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

// @ft:23
func TestSync_AlreadyTaggedScenarioIsSkipped(t *testing.T) {
	inTempDir(t)
	runInit(t)

	// Pre-create the scenario in the DB
	setupFx := dbtest.Open(t, "fts/ft.db")
	setupFx.InsertFile("fts/login.ft")
	setupFx.InsertScenarioStub(1, "User logs in")
	setupFx.Close()

	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  @ft:1
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 1, fx.CountScenarios())
}

// @ft:24
func TestSync_ScenarioRecordStoresMetadata(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	name, fileID, createdAt, updatedAt := fx.ScenarioMeta(1)

	assert.Equal(t, "User logs in", name)

	// Verify file_id matches the files record for fts/login.ft
	assert.Equal(t, fx.FileID("fts/login.ft"), fileID)

	assert.NotEmpty(t, createdAt)
	assert.NotEmpty(t, updatedAt)
}

// @ft:25
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

// @ft:26
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

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 2, fx.CountScenarios())
	assert.Equal(t, 1, fx.CountScenariosByName("User logs in"))
	assert.Equal(t, 1, fx.CountScenariosByName("User completes purchase"))
}

// @ft:27
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

// @ft:28
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

// @ft:29
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

// @ft:30
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

// @ft:31
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

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 0, fx.CountScenarios())
}

// @ft:32
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

// @ft:33
func TestSync_ParseIsIdempotent(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)
	runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 1, fx.CountScenariosByName("User logs in"))

	// Check file has exactly one @ft tag
	data, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(data), "@ft:"))
}

// @ft:34
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

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 1, fx.CountScenariosByName("User logs in"))
	assert.Equal(t, 0, fx.CountScenariosByName("Background"))
}

// @ft:35
func TestSync_TaggedScenarioWithoutDBRecordGetsNewID(t *testing.T) {
	inTempDir(t)
	runInit(t)

	// Pre-create the file record so the file is tracked
	setupFx := dbtest.Open(t, "fts/ft.db")
	setupFx.InsertFile("fts/login.ft")
	setupFx.Close()

	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  @ft:5
  Scenario: User logs in
    Given a user
`), 0o644))

	runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")

	// Should have a record with name "User logs in" (any id)
	id, name := fx.ScenarioByName("User logs in")
	assert.Equal(t, "User logs in", name)

	// Stale ID 5 should not be used
	assert.Equal(t, 0, fx.CountScenariosByID(5))

	// File should contain the new @ft:<id> tag, not @ft:5
	data, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, fmt.Sprintf("@ft:%d", id))
	assert.NotContains(t, content, "@ft:5")
}

// @ft:36
func TestSync_NewFileStripsStaleTagsAndAssignsNewIDs(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  @ft:99
  Scenario: User logs in
    Given a user
`), 0o644))

	out := runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")

	// Should have a fresh ID (1), not the stale 99
	assert.Equal(t, "User logs in", fx.ScenarioName(1))

	// Stale ID should not exist
	assert.Equal(t, 0, fx.CountScenariosByID(99))

	// File should have @ft:1, not @ft:99
	data, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "@ft:1")
	assert.NotContains(t, content, "@ft:99")

	assert.Contains(t, out, "@ft:1 User logs in")
}

// @ft:37
func TestSync_ScenariosTableMigration(t *testing.T) {
	inTempDir(t)
	runInit(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.True(t, fx.TableExists("scenarios"))
	assert.Equal(t, 6, fx.SchemaVersion())
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

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, "User signs in", fx.ScenarioName(1))
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
	setupFx := dbtest.Open(t, "fts/ft.db")
	_ = setupFx.ScenarioUpdatedAt(1)
	setupFx.Close()

	// Change step content
	data, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	updated := strings.Replace(string(data), "Given a user", "Given an admin", 1)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(updated), 0o644))

	out := runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Contains(t, fx.ScenarioContent(1).String, "Given an admin")
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

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 1, fx.CountScenarios())

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

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 2, fx.CountScenarios())

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

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 1, fx.CountScenarios())
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
	setupFx := dbtest.Open(t, "fts/ft.db")
	setupFx.InsertStatus(1, "pass")
	setupFx.Close()

	// Remove the scenario from the file
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
`), 0o644))

	out := runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")

	// Scenario should still exist
	assert.Equal(t, 1, fx.CountScenariosByID(1))

	// Should have a "removed" status
	assert.Equal(t, "removed", fx.LatestStatusByID(1))

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

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 0, fx.CountScenariosByID(1))
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

	fx := dbtest.Open(t, "fts/ft.db")
	assert.True(t, fx.FileDeleted("fts/login.ft"))

	// Scenario should be deleted (no history)
	assert.Equal(t, 0, fx.CountScenariosByID(1))
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
	setupFx := dbtest.Open(t, "fts/ft.db")
	setupFx.InsertStatus(1, "pass")
	setupFx.Close()

	require.NoError(t, os.Remove("fts/login.ft"))

	runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.True(t, fx.FileDeleted("fts/login.ft"))

	// Scenario preserved
	assert.Equal(t, 1, fx.CountScenariosByID(1))

	// "removed" status added
	assert.Equal(t, "removed", fx.LatestStatusByID(1))
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
	setupFx := dbtest.Open(t, "fts/ft.db")
	setupFx.MarkFileDeletedByPath("fts/login.ft")
	setupFx.Close()

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

	fx := dbtest.Open(t, "fts/ft.db")
	content := fx.ScenarioContent(1)
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

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Contains(t, fx.ScenarioContent(1).String, "Given an admin")
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

	setupFx := dbtest.Open(t, "fts/ft.db")
	_ = setupFx.ScenarioUpdatedAt(1)
	setupFx.Close()

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
	setupFx := dbtest.Open(t, "fts/ft.db")
	setupFx.InsertStatus(1, "pass")
	setupFx.Close()

	// Remove the scenario from the file
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
`), 0o644))

	runSync(t) // first sync after removal — adds "removed" status

	checkFx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 1, checkFx.CountStatusesByStatus(1, "removed"))
	checkFx.Close()

	out := runSync(t) // second sync after removal — should NOT add another "removed"

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 1, fx.CountStatusesByStatus(1, "removed"), "should not add duplicate removed status")

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
	setupFx := dbtest.Open(t, "fts/ft.db")
	setupFx.InsertStatus(1, "pass")
	setupFx.Close()

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

	fx := dbtest.Open(t, "fts/ft.db")

	// Scenario should still exist
	assert.Equal(t, 1, fx.CountScenariosByID(1))

	// Should have a "restored" status as the latest
	assert.Equal(t, "restored", fx.LatestStatusByID(1))

	// Should only have one "removed" status (not re-removed)
	assert.Equal(t, 1, fx.CountStatusesByStatus(1, "removed"))

	// Output should show + indicator for restored scenario
	assert.Contains(t, out, "+")
	assert.Contains(t, out, "@ft:1 User logs in")
}

// @ft:155
func TestSync_ContentChangeInsertsModifiedStatus(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)
	runStatusUpdate(t, "1", "accepted")

	// Change the step content
	data, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	updated := strings.Replace(string(data), "Given a user", "Given an admin", 1)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(updated), 0o644))

	runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, "modified", fx.LatestStatusByID(1))
}

// @ft:156
func TestSync_NameChangeDoesNotInsertModifiedStatus(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)
	runStatusUpdate(t, "1", "accepted")

	// Change only the scenario name, not the steps
	data, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	updated := strings.Replace(string(data), "User logs in", "User signs in", 1)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(updated), 0o644))

	runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, "accepted", fx.LatestStatusByID(1))
}

// @ft:157
func TestSync_RepeatedSyncDoesNotDuplicateModified(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)
	runStatusUpdate(t, "1", "accepted")

	// Change content
	data, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	updated := strings.Replace(string(data), "Given a user", "Given an admin", 1)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(updated), 0o644))

	runSync(t)
	runSync(t) // sync again without further edits

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 1, fx.CountStatusesByStatus(1, "modified"))
}

// @ft:160
func TestSync_ContentChangeWhileAlreadyModifiedDoesNotDuplicate(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)
	runStatusUpdate(t, "1", "accepted")

	// First content change
	data, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	updated := strings.Replace(string(data), "Given a user", "Given an admin", 1)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(updated), 0o644))
	runSync(t)

	// Second content change while already "modified"
	data, err = os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	updated = strings.Replace(string(data), "Given an admin", "Given a superuser", 1)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(updated), 0o644))
	runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 1, fx.CountStatusesByStatus(1, "modified"))
}

// @ft:159
func TestSync_RestoredScenarioDoesNotGetModifiedStatus(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)
	runStatusUpdate(t, "1", "accepted")

	// Remove the scenario
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
`), 0o644))
	runSync(t)

	// Restore with different content
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  @ft:1
  Scenario: User logs in
    Given an admin
`), 0o644))
	runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, "restored", fx.LatestStatusByID(1))
	assert.Equal(t, 0, fx.CountStatusesByStatus(1, "modified"))
}

// @ft:165
func TestSync_NoStatusHistoryDoesNotGetModified(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)

	// Change content without ever setting a status
	data, err := os.ReadFile("fts/login.ft")
	require.NoError(t, err)
	updated := strings.Replace(string(data), "Given a user", "Given an admin", 1)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(updated), 0o644))
	runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 0, fx.CountStatuses(1))
}

// @ft:171
func TestSync_DiscoversTestLink(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)

	require.NoError(t, os.MkdirAll("pkg", 0o755))
	require.NoError(t, os.WriteFile("pkg/login_test.go", []byte(`package pkg
// @ft:1
func TestLogin(t *testing.T) {}
`), 0o644))

	runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	filePath, lineNumber := fx.TestLink(1)
	assert.Equal(t, "pkg/login_test.go", filePath)
	assert.Equal(t, 2, lineNumber)
}

// this is a test
// @ft:172 @ft:171 @ft:999
func TestSync_DiscoversMultipleTestLinksInOneFile(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user

  Scenario: User fails login
    Given a user
`), 0o644))
	runSync(t)

	require.NoError(t, os.MkdirAll("pkg", 0o755))
	require.NoError(t, os.WriteFile("pkg/login_test.go", []byte(`package pkg
// @ft:1
func TestLogin(t *testing.T) {}
// @ft:2
func TestLoginFail(t *testing.T) {}
`), 0o644))

	runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 2, fx.CountTestLinks())
}

// @ft:173
func TestSync_DiscoversTestLinkFromMultipleFiles(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)

	require.NoError(t, os.MkdirAll("pkg", 0o755))
	require.NoError(t, os.MkdirAll("cmd", 0o755))
	require.NoError(t, os.WriteFile("pkg/login_test.go", []byte(`package pkg
// @ft:1
func TestLogin(t *testing.T) {}
`), 0o644))
	require.NoError(t, os.WriteFile("cmd/login_test.go", []byte(`package cmd
// @ft:1
func TestLoginCmd(t *testing.T) {}
`), 0o644))

	runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 2, fx.CountTestLinksForScenario(1))
}

// @ft:174
func TestSync_RemovesStaleTestLink(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)

	require.NoError(t, os.MkdirAll("pkg", 0o755))
	require.NoError(t, os.WriteFile("pkg/login_test.go", []byte(`package pkg
// @ft:1
func TestLogin(t *testing.T) {}
`), 0o644))
	runSync(t)

	// Remove the @ft tag from the test file
	require.NoError(t, os.WriteFile("pkg/login_test.go", []byte(`package pkg
func TestLogin(t *testing.T) {}
`), 0o644))
	runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 0, fx.CountTestLinks())
}

// @ft:175
func TestSync_UpdatesLineNumber(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)

	require.NoError(t, os.MkdirAll("pkg", 0o755))
	require.NoError(t, os.WriteFile("pkg/login_test.go", []byte(`package pkg
// @ft:1
func TestLogin(t *testing.T) {}
`), 0o644))
	runSync(t)

	// Move the tag to a different line
	require.NoError(t, os.WriteFile("pkg/login_test.go", []byte(`package pkg

import "testing"

// @ft:1
func TestLogin(t *testing.T) {}
`), 0o644))
	runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	_, lineNumber := fx.TestLink(1)
	assert.Equal(t, 5, lineNumber)
}

// @ft:176
func TestSync_SkipsNonTestFiles(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)

	require.NoError(t, os.MkdirAll("pkg", 0o755))
	require.NoError(t, os.WriteFile("pkg/login.go", []byte(`package pkg
// @ft:1
func Login() {}
`), 0o644))
	runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 0, fx.CountTestLinks())
}

// @ft:177
func TestSync_SkipsGitDirectory(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)

	require.NoError(t, os.MkdirAll(".git/hooks", 0o755))
	require.NoError(t, os.WriteFile(".git/hooks/pre_commit_test.go", []byte(`package hooks
// @ft:1
func TestHook(t *testing.T) {}
`), 0o644))
	runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 0, fx.CountTestLinks())
}

// @ft:178
func TestSync_IgnoresUnknownScenarioID(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)

	require.NoError(t, os.MkdirAll("pkg", 0o755))
	require.NoError(t, os.WriteFile("pkg/login_test.go", []byte(`package pkg
// @ft:999
func TestLogin(t *testing.T) {}
`), 0o644))
	runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 0, fx.CountTestLinks())
}

// @ft:184
func TestSync_OnlyMatchesTagsInComments(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)

	require.NoError(t, os.MkdirAll("pkg", 0o755))
	require.NoError(t, os.WriteFile("pkg/login_test.go", []byte(`package pkg

func TestLogin(t *testing.T) {
	tag := "@ft:1"
	_ = tag
}
`), 0o644))
	runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 0, fx.CountTestLinks())
}

// @ft:185
func TestSync_OnlyMatchesTagAboveFuncTest(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)

	require.NoError(t, os.MkdirAll("pkg", 0o755))
	require.NoError(t, os.WriteFile("pkg/login_test.go", []byte(`package pkg

// @ft:1
var testData = "some data"

func TestLogin(t *testing.T) {}
`), 0o644))
	runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 0, fx.CountTestLinks())
}

// @ft:186
func TestSync_IgnoresTagInsideRawStringLiteral(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)

	require.NoError(t, os.MkdirAll("pkg", 0o755))
	require.NoError(t, os.WriteFile("pkg/setup_test.go", []byte("package pkg\n"+
		"\n"+
		"import (\n"+
		"\t\"os\"\n"+
		"\t\"testing\"\n"+
		")\n"+
		"\n"+
		"func TestSetup(t *testing.T) {\n"+
		"\tos.WriteFile(\"other_test.go\", []byte(`package other\n"+
		"// @ft:1\n"+
		"func TestOther(t *testing.T) {}\n"+
		"`), 0o644)\n"+
		"}\n",
	), 0o644))
	runSync(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 0, fx.CountTestLinks())
}
