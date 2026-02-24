package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runList(t *testing.T, includes ...string) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, RunList(&buf, includes, nil))
	return buf.String()
}

func TestList_SingleScenario(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)

	out := runList(t)

	assert.Contains(t, out, "@ft:1")
	assert.Contains(t, out, "login.ft")
	assert.Contains(t, out, "User logs in")
}

func TestList_MultipleScenariosFromOneFile(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user

  Scenario: User fails login
    Given a user
`), 0o644))
	runSync(t)

	out := runList(t)

	assert.Contains(t, out, "User logs in")
	assert.Contains(t, out, "User fails login")
}

func TestList_ScenariosFromMultipleFiles(t *testing.T) {
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

	out := runList(t)

	assert.Contains(t, out, "login.ft")
	assert.Contains(t, out, "checkout.ft")
}

func TestList_SortedByFilePathThenID(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/checkout.ft", []byte(`Feature: Checkout
  Scenario: User completes purchase
    Given a cart
`), 0o644))
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)

	out := runList(t)

	checkoutIdx := strings.Index(out, "checkout.ft")
	loginIdx := strings.Index(out, "login.ft")
	require.True(t, checkoutIdx >= 0, "output should contain checkout.ft")
	require.True(t, loginIdx >= 0, "output should contain login.ft")
	assert.True(t, checkoutIdx < loginIdx, "checkout.ft should appear before login.ft")
}

func TestList_NoStatusShowsNoActivity(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)

	out := runList(t)

	assert.Contains(t, out, "no-activity")
}

func TestList_EmptyWhenNoScenarios(t *testing.T) {
	inTempDir(t)
	runInit(t)

	out := runList(t)

	assert.Empty(t, out)
}

func TestList_ColumnsAligned(t *testing.T) {
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

	out := runList(t)

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	require.True(t, len(lines) >= 2, "should have at least 2 lines")

	// The "no-activity" status column should be aligned across all rows
	statusPositions := make([]int, len(lines))
	for i, line := range lines {
		statusPositions[i] = strings.Index(line, "no-activity")
		require.True(t, statusPositions[i] >= 0, "each line should contain no-activity")
	}
	for i := 1; i < len(statusPositions); i++ {
		assert.Equal(t, statusPositions[0], statusPositions[i], "status columns should be aligned across rows")
	}
}

func TestList_FileNameShowsBasename(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)

	out := runList(t)

	assert.Contains(t, out, "login.ft")
	assert.NotContains(t, out, "fts/login.ft")
}

func TestList_FilterBySingleStatus(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user

  Scenario: User fails login
    Given a user

  Scenario: User resets password
    Given a user
`), 0o644))
	runSync(t)
	runStatusUpdate(t, "1", "accepted")
	runStatusUpdate(t, "2", "in-progress")

	out := runList(t, "accepted")

	assert.Contains(t, out, "User logs in")
	assert.NotContains(t, out, "User fails login")
	assert.NotContains(t, out, "User resets password")
}

func TestList_FilterByNegatedStatus(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user

  Scenario: User fails login
    Given a user

  Scenario: User resets password
    Given a user
`), 0o644))
	runSync(t)
	runStatusUpdate(t, "1", "accepted")
	runStatusUpdate(t, "2", "in-progress")
	runStatusUpdate(t, "3", "removed")

	var buf bytes.Buffer
	require.NoError(t, RunList(&buf, nil, []string{"removed"}))
	out := buf.String()

	assert.Contains(t, out, "User logs in")
	assert.Contains(t, out, "User fails login")
	assert.NotContains(t, out, "User resets password")
}

func TestList_FilterByMultiplePositiveStatuses(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user

  Scenario: User fails login
    Given a user

  Scenario: User resets password
    Given a user
`), 0o644))
	runSync(t)
	runStatusUpdate(t, "1", "accepted")
	runStatusUpdate(t, "2", "ready")
	runStatusUpdate(t, "3", "in-progress")

	out := runList(t, "accepted", "ready")

	assert.Contains(t, out, "User logs in")
	assert.Contains(t, out, "User fails login")
	assert.NotContains(t, out, "User resets password")
}

func TestList_FilterByMultipleNegatedStatuses(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user

  Scenario: User fails login
    Given a user

  Scenario: User resets password
    Given a user
`), 0o644))
	runSync(t)
	runStatusUpdate(t, "1", "accepted")
	runStatusUpdate(t, "2", "removed")

	var buf bytes.Buffer
	require.NoError(t, RunList(&buf, nil, []string{"removed", "no-activity"}))
	out := buf.String()

	assert.Contains(t, out, "User logs in")
	assert.NotContains(t, out, "User fails login")
	assert.NotContains(t, out, "User resets password")
}

func TestList_FilterMixedPositiveAndNegated(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user

  Scenario: User fails login
    Given a user

  Scenario: User resets password
    Given a user
`), 0o644))
	runSync(t)
	runStatusUpdate(t, "1", "ready")
	runStatusUpdate(t, "2", "accepted")

	var buf bytes.Buffer
	require.NoError(t, RunList(&buf, []string{"ready"}, []string{"no-activity"}))
	out := buf.String()

	assert.Contains(t, out, "User logs in")
	assert.NotContains(t, out, "User fails login")
	assert.NotContains(t, out, "User resets password")
}

func TestList_FilterNoMatchesEmpty(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)
	runStatusUpdate(t, "1", "accepted")

	out := runList(t, "done")

	assert.Empty(t, out)
}

func TestList_NoFilterShowsAll(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user

  Scenario: User fails login
    Given a user

  Scenario: User resets password
    Given a user
`), 0o644))
	runSync(t)
	runStatusUpdate(t, "1", "accepted")
	runStatusUpdate(t, "2", "in-progress")

	out := runList(t)

	assert.Contains(t, out, "User logs in")
	assert.Contains(t, out, "User fails login")
	assert.Contains(t, out, "User resets password")
}

func TestList_RequiresInit(t *testing.T) {
	inTempDir(t)

	var buf bytes.Buffer
	err := RunList(&buf, nil, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "run `ft init` first")
}
