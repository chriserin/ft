package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runTests(t *testing.T, id string) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, RunTests(&buf, id))
	return buf.String()
}

// @ft:179
func TestTests_ListsLinkedFiles(t *testing.T) {
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

	out := runTests(t, "1")

	assert.Contains(t, out, "pkg/login_test.go:2")
}

// @ft:180
func TestTests_AcceptsAtFtPrefix(t *testing.T) {
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

	out := runTests(t, "@ft:1")

	assert.Contains(t, out, "pkg/login_test.go:2")
}

// @ft:181
func TestTests_NoLinksShowsMessage(t *testing.T) {
	inTempDir(t)
	runInit(t)
	require.NoError(t, os.WriteFile("fts/login.ft", []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`), 0o644))
	runSync(t)

	out := runTests(t, "1")

	assert.Contains(t, out, "no linked tests for @ft:1")
}
