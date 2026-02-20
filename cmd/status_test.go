package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/chriserin/ft/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runStatusUpdate(t *testing.T, id, status string) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, RunStatusUpdate(&buf, id, status))
	return buf.String()
}

func runStatusReport(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, RunStatusReport(&buf))
	return buf.String()
}

func setupScenario(t *testing.T, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(content), 0o644))
	runSync(t)
}

// @ft:57
func TestStatus_SetScenarioStatus(t *testing.T) {
	inTempDir(t)
	runInit(t)
	setupScenario(t, "Feature: Login\n  Scenario: User logs in\n    Given a user\n")

	runStatusUpdate(t, "1", "accepted")

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var status string
	require.NoError(t, sqlDB.QueryRow(`SELECT status FROM statuses WHERE scenario_id = 1`).Scan(&status))
	assert.Equal(t, "accepted", status)
}

// @ft:58
func TestStatus_AcceptsAtFtPrefix(t *testing.T) {
	inTempDir(t)
	runInit(t)
	setupScenario(t, "Feature: Login\n  Scenario: User logs in\n    Given a user\n")

	runStatusUpdate(t, "@ft:1", "in-progress")

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var status string
	require.NoError(t, sqlDB.QueryRow(`SELECT status FROM statuses WHERE scenario_id = 1`).Scan(&status))
	assert.Equal(t, "in-progress", status)
}

// @ft:59
func TestStatus_AcceptsAnyText(t *testing.T) {
	inTempDir(t)
	runInit(t)
	setupScenario(t, "Feature: Login\n  Scenario: User logs in\n    Given a user\n")

	runStatusUpdate(t, "1", "my-custom-status")

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var status string
	require.NoError(t, sqlDB.QueryRow(`SELECT status FROM statuses WHERE scenario_id = 1`).Scan(&status))
	assert.Equal(t, "my-custom-status", status)
}

// @ft:60
func TestStatus_UnknownIDReturnsError(t *testing.T) {
	inTempDir(t)
	runInit(t)

	var buf bytes.Buffer
	err := RunStatusUpdate(&buf, "999", "accepted")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "999")
}

// @ft:61
func TestStatus_MultipleChangesCreateHistory(t *testing.T) {
	inTempDir(t)
	runInit(t)
	setupScenario(t, "Feature: Login\n  Scenario: User logs in\n    Given a user\n")

	runStatusUpdate(t, "1", "accepted")
	runStatusUpdate(t, "1", "in-progress")

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var count int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM statuses WHERE scenario_id = 1`).Scan(&count))
	assert.Equal(t, 2, count)

	var latest string
	require.NoError(t, sqlDB.QueryRow(`SELECT status FROM statuses WHERE scenario_id = 1 ORDER BY changed_at DESC, id DESC LIMIT 1`).Scan(&latest))
	assert.Equal(t, "in-progress", latest)
}

// @ft:62
func TestStatus_ReportWithNoScenarios(t *testing.T) {
	inTempDir(t)
	runInit(t)

	out := runStatusReport(t)

	assert.Contains(t, out, "Scenarios: 0")
}

// @ft:63
func TestStatus_ReportCountsByStatus(t *testing.T) {
	inTempDir(t)
	runInit(t)
	setupScenario(t, "Feature: Login\n  Scenario: User logs in\n    Given a user\n\n  Scenario: User fails login\n    Given a user\n")
	require.NoError(t, os.WriteFile("fts/checkout.ft", []byte("Feature: Checkout\n  Scenario: User completes purchase\n    Given a cart\n"), 0o644))
	runSync(t)

	runStatusUpdate(t, "1", "accepted")
	runStatusUpdate(t, "2", "in-progress")

	out := runStatusReport(t)

	assert.Contains(t, out, "Scenarios: 3")
	assert.Contains(t, out, "accepted")
	assert.Contains(t, out, "in-progress")
	assert.Contains(t, out, "no-activity")
}

// @ft:64
func TestStatus_ReportNoActivityLast(t *testing.T) {
	inTempDir(t)
	runInit(t)
	setupScenario(t, "Feature: Login\n  Scenario: User logs in\n    Given a user\n")

	out := runStatusReport(t)

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	lastLine := lines[len(lines)-1]
	assert.Contains(t, lastLine, "no-activity")
}

// @ft:65
func TestStatus_ReportOrdersByCountDesc(t *testing.T) {
	inTempDir(t)
	runInit(t)
	// Create 4 scenarios
	require.NoError(t, os.WriteFile("fts/login.ft", []byte("Feature: Login\n  Scenario: S1\n    Given a\n\n  Scenario: S2\n    Given a\n\n  Scenario: S3\n    Given a\n\n  Scenario: S4\n    Given a\n"), 0o644))
	runSync(t)

	// 2 accepted, 1 in-progress, 1 no-activity
	runStatusUpdate(t, "1", "accepted")
	runStatusUpdate(t, "2", "accepted")
	runStatusUpdate(t, "3", "in-progress")

	out := runStatusReport(t)

	acceptedIdx := strings.Index(out, "accepted")
	inProgressIdx := strings.Index(out, "in-progress")
	noActivityIdx := strings.Index(out, "no-activity")

	assert.True(t, acceptedIdx < inProgressIdx, "accepted (count 2) should appear before in-progress (count 1)")
	assert.True(t, inProgressIdx < noActivityIdx, "in-progress should appear before no-activity")
}

// @ft:66
func TestStatus_RequiresInit(t *testing.T) {
	inTempDir(t)

	var buf bytes.Buffer
	err := RunStatusUpdate(&buf, "1", "accepted")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "run `ft init` first")
}

// @ft:67
func TestStatus_NoArgsReturnsReport(t *testing.T) {
	inTempDir(t)
	runInit(t)

	out := runStatusReport(t)

	assert.Contains(t, out, "Scenarios:")
}

// @ft:68
func TestShow_DisplaysCurrentStatusFromDB(t *testing.T) {
	inTempDir(t)
	runInit(t)
	setupScenario(t, "Feature: Login\n  Scenario: User logs in\n    Given a user\n")

	runStatusUpdate(t, "1", "accepted")

	out := runShow(t, "1")

	assert.Contains(t, out, "Status: accepted")
	assert.NotContains(t, out, "no-activity")
}

// @ft:69
func TestShow_DisplaysStatusHistory(t *testing.T) {
	inTempDir(t)
	runInit(t)
	setupScenario(t, "Feature: Login\n  Scenario: User logs in\n    Given a user\n")

	runStatusUpdate(t, "1", "accepted")
	runStatusUpdate(t, "1", "in-progress")

	out := runShow(t, "1")

	assert.Contains(t, out, "History:")
	// Find "accepted" after the History: line
	historyIdx := strings.Index(out, "History:")
	acceptedInHistory := strings.Index(out[historyIdx:], "accepted")
	inProgressInHistory := strings.Index(out[historyIdx:], "in-progress")
	assert.True(t, inProgressInHistory < acceptedInHistory, "in-progress should appear before accepted in history")

	// Each history entry should have a timestamp-like pattern (contains a comma for "Jan 2, 2006")
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "in-progress") || strings.HasPrefix(strings.TrimSpace(line), "accepted") {
			// Lines in the history section should have timestamps
			if strings.Contains(out[strings.Index(out, "History:"):], line) && strings.TrimSpace(line) != "" {
				assert.Contains(t, line, ",", "history entry should contain a timestamp with comma")
			}
		}
	}
}

// @ft:70 — already exists in show_test.go, this verifies no History: section
func TestShow_NoStatusStillShowsNoActivity(t *testing.T) {
	inTempDir(t)
	runInit(t)
	setupScenario(t, "Feature: Login\n  Scenario: User logs in\n    Given a user\n")

	out := runShow(t, "1")

	assert.Contains(t, out, "Status: no-activity")
	assert.NotContains(t, out, "History:")
}

// @ft:71
func TestList_ShowsCurrentStatusFromDB(t *testing.T) {
	inTempDir(t)
	runInit(t)
	setupScenario(t, "Feature: Login\n  Scenario: User logs in\n    Given a user\n")

	runStatusUpdate(t, "1", "accepted")

	out := runList(t)

	assert.Contains(t, out, "accepted")
	assert.NotContains(t, out, "no-activity")
}

// @ft:72
func TestList_FiltersByStatusFlag(t *testing.T) {
	inTempDir(t)
	runInit(t)
	setupScenario(t, "Feature: Login\n  Scenario: User logs in\n    Given a user\n\n  Scenario: User fails login\n    Given a user\n")

	runStatusUpdate(t, "1", "accepted")
	runStatusUpdate(t, "2", "in-progress")

	var buf bytes.Buffer
	require.NoError(t, RunList(&buf, "accepted", false))
	out := buf.String()

	assert.Contains(t, out, "User logs in")
	assert.NotContains(t, out, "User fails login")
}

// @ft:73
func TestList_FiltersByNoActivityFlag(t *testing.T) {
	inTempDir(t)
	runInit(t)
	setupScenario(t, "Feature: Login\n  Scenario: User logs in\n    Given a user\n\n  Scenario: User fails login\n    Given a user\n")

	runStatusUpdate(t, "1", "accepted")

	var buf bytes.Buffer
	require.NoError(t, RunList(&buf, "", true))
	out := buf.String()

	assert.Contains(t, out, "User fails login")
	assert.NotContains(t, out, "User logs in")
}

// @ft:74
func TestStatus_RequiresBothIdAndStatus(t *testing.T) {
	inTempDir(t)
	runInit(t)

	var buf bytes.Buffer
	// Simulate calling with just 1 arg by calling the command's RunE directly
	err := statusCmd.RunE(statusCmd, []string{"1"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "usage")
	_ = buf
}

// @ft:75
func TestStatus_UpdatePrintsConfirmation(t *testing.T) {
	inTempDir(t)
	runInit(t)
	setupScenario(t, "Feature: Login\n  Scenario: User logs in\n    Given a user\n")

	out := runStatusUpdate(t, "1", "accepted")

	assert.Contains(t, out, "@ft:1")
	assert.Contains(t, out, "accepted")
}

// @ft:76
func TestStatus_ReportOmitsZeroCounts(t *testing.T) {
	inTempDir(t)
	runInit(t)
	setupScenario(t, "Feature: Login\n  Scenario: User logs in\n    Given a user\n")

	runStatusUpdate(t, "1", "accepted")

	out := runStatusReport(t)

	assert.Contains(t, out, "accepted")
	assert.NotContains(t, out, "in-progress")
	assert.NotContains(t, out, "done")
	assert.NotContains(t, out, "blocked")
}

// @ft:77
func TestList_FilterNoMatchesReturnsEmpty(t *testing.T) {
	inTempDir(t)
	runInit(t)
	setupScenario(t, "Feature: Login\n  Scenario: User logs in\n    Given a user\n")

	runStatusUpdate(t, "1", "accepted")

	var buf bytes.Buffer
	err := RunList(&buf, "done", false)

	require.NoError(t, err)
	assert.Empty(t, buf.String())
}

// @ft:78
func TestStatus_ReportRequiresInit(t *testing.T) {
	inTempDir(t)

	var buf bytes.Buffer
	err := RunStatusReport(&buf)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "run `ft init` first")
}

// @ft:80
func TestShow_HistoryAlignsTimestamps(t *testing.T) {
	inTempDir(t)
	runInit(t)
	setupScenario(t, "Feature: Login\n  Scenario: User logs in\n    Given a user\n")

	runStatusUpdate(t, "1", "accepted")
	runStatusUpdate(t, "1", "in-progress")

	out := runShow(t, "1")

	assert.Contains(t, out, "History:")
	historyStart := strings.Index(out, "History:")
	historySection := out[historyStart:]
	lines := strings.Split(strings.TrimRight(historySection, "\n"), "\n")

	// Collect history entry lines (indented lines after "History:")
	var entryLines []string
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || !strings.HasPrefix(line, "  ") {
			break
		}
		entryLines = append(entryLines, line)
	}
	require.True(t, len(entryLines) >= 2, "should have at least 2 history entries")

	// Find the position of the comma (part of timestamp) in each line — they should align
	commaPositions := make([]int, len(entryLines))
	for i, line := range entryLines {
		commaPositions[i] = strings.Index(line, ",")
		require.True(t, commaPositions[i] >= 0, "each history line should contain a timestamp with comma")
	}
	for i := 1; i < len(commaPositions); i++ {
		assert.Equal(t, commaPositions[0], commaPositions[i], "timestamp columns should be aligned across history rows")
	}
}

// @ft:81
func TestStatus_UpdateConfirmationShowsPreviousStatus(t *testing.T) {
	inTempDir(t)
	runInit(t)
	setupScenario(t, "Feature: Login\n  Scenario: User logs in\n    Given a user\n")

	runStatusUpdate(t, "1", "accepted")
	out := runStatusUpdate(t, "1", "in-progress")

	assert.Contains(t, out, "accepted → in-progress")
	assert.Contains(t, out, "@ft:1")
}

// @ft:79
func TestShow_HistoryUsesHumanReadableTimestamps(t *testing.T) {
	inTempDir(t)
	runInit(t)
	setupScenario(t, "Feature: Login\n  Scenario: User logs in\n    Given a user\n")

	runStatusUpdate(t, "1", "accepted")

	out := runShow(t, "1")

	assert.Contains(t, out, "History:")
	// Should contain human-readable format like "Feb 20, 2026 3:04pm"
	// Should NOT contain ISO 8601 format like "2026-02-20T"
	historySection := out[strings.Index(out, "History:"):]
	assert.NotContains(t, historySection, "T")      // ISO 8601 uses T separator
	assert.Contains(t, historySection, ",")          // Human readable has comma in "Jan 2, 2006"
}
