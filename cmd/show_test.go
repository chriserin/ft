package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runShowHistory(t *testing.T, id string) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, RunShowHistory(&buf, id))
	return buf.String()
}

func runShow(t *testing.T, id string) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, RunShow(&buf, id))
	return buf.String()
}

func TestShow_SingleScenario(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)

	out := runShow(t, "1")

	assert.Contains(t, out, "@ft:1")
	assert.Contains(t, out, "User logs in")
}

func TestShow_DisplaysGherkinContent(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given the user is on the login page
    When  the user enters valid credentials
    Then  the user sees the dashboard
`), 0o644))
	runSync(t)

	out := runShow(t, "1")

	assert.Contains(t, out, "Given the user is on the login page")
	assert.Contains(t, out, "When  the user enters valid credentials")
	assert.Contains(t, out, "Then  the user sees the dashboard")
}

func TestShow_NoStatusShowsNoActivity(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)

	out := runShow(t, "1")

	assert.Contains(t, out, "Status:")
	assert.Contains(t, out, "no-activity")
}

func TestShow_UnknownIDReturnsError(t *testing.T) {
	inTempDir(t)
	runInit(t)

	var buf bytes.Buffer
	err := RunShow(&buf, "999")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "999")
}

func TestShow_AcceptsAtFtPrefix(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)

	out := runShow(t, "@ft:1")

	assert.Contains(t, out, "@ft:1")
	assert.Contains(t, out, "User logs in")
}

func TestShow_FileNameShowsBasename(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)

	out := runShow(t, "1")

	assert.Contains(t, out, "login.ft")
	assert.NotContains(t, out, "fts/login.ft")
}

func TestShow_RequiresInit(t *testing.T) {
	inTempDir(t)

	var buf bytes.Buffer
	err := RunShow(&buf, "1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "run `ft init` first")
}

func TestShow_NoArgumentReturnsError(t *testing.T) {
	inTempDir(t)
	runInit(t)

	var buf bytes.Buffer
	err := RunShow(&buf, "notanumber")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid scenario ID")
}

func TestShow_ContentFromMultiScenarioFile(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given the user is on the login page
    When  the user enters valid credentials
    Then  the user sees the dashboard

  Scenario: User fails login
    Given the user is on the login page
    When  the user enters wrong credentials
    Then  the user sees an error
`), 0o644))
	runSync(t)

	out1 := runShow(t, "1")
	assert.Contains(t, out1, "User logs in")
	assert.Contains(t, out1, "the user enters valid credentials")
	assert.NotContains(t, out1, "User fails login")

	out2 := runShow(t, "2")
	assert.Contains(t, out2, "User fails login")
	assert.Contains(t, out2, "the user enters wrong credentials")
	assert.NotContains(t, out2, "User logs in")
}

func TestShow_IncludesBackgroundSection(t *testing.T) {
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

	out := runShow(t, "1")

	assert.Contains(t, out, "Background:")
	assert.Contains(t, out, "Given a registered user")
	assert.Contains(t, out, "User logs in")
}

func TestShow_HistoryFlag(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)
	runStatusUpdate(t, "1", "accepted")
	runStatusUpdate(t, "1", "in-progress")

	out := runShowHistory(t, "1")

	assert.Contains(t, out, "History:")
	assert.Contains(t, out, "@ft:1")
	assert.Contains(t, out, "User logs in")
	assert.Contains(t, out, "in-progress")
	assert.Contains(t, out, "accepted")
	assert.NotContains(t, out, "Scenario:")
}

func TestShow_HistoryFlagNoStatus(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)

	out := runShowHistory(t, "1")

	assert.Contains(t, out, "History:")
	assert.Contains(t, out, "@ft:1")
	assert.Contains(t, out, "no-activity")
}

// @ft:182
func TestShow_IncludesTestsSection(t *testing.T) {
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

	out := runShow(t, "1")

	assert.Contains(t, out, "\n\nTests:")
	assert.Contains(t, out, "pkg/login_test.go:2")
}

// @ft:183
func TestShow_OmitsTestsSectionWhenNoLinks(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)

	out := runShow(t, "1")

	assert.NotContains(t, out, "Tests:")
}

func TestShow_RemovedScenarioUsesStoredContent(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given the user is on the login page
    When  the user enters valid credentials
    Then  the user sees the dashboard
`), 0o644))
	runSync(t)
	runStatusUpdate(t, "1", "accepted")

	// Remove the scenario from the file
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
`), 0o644))
	runSync(t)

	out := runShow(t, "1")

	assert.Contains(t, out, "@ft:1")
	assert.Contains(t, out, "Given the user is on the login page")
	assert.Contains(t, out, "When  the user enters valid credentials")
	assert.Contains(t, out, "Then  the user sees the dashboard")
	assert.Contains(t, out, "removed")
}
