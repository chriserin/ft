package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chriserin/ft/internal/db/dbtest"
)

func inTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(orig) })
	return dir
}

func runInit(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, RunInit(&buf))
	return buf.String()
}

// @ft:1
func TestInit_CreatesFtsDirectory(t *testing.T) {
	dir := inTempDir(t)
	out := runInit(t)

	info, err := os.Stat(filepath.Join(dir, "fts"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.Contains(t, out, "fts/ created")
}

// @ft:2
func TestInit_FtsDirectoryAlreadyExists(t *testing.T) {
	dir := inTempDir(t)
	require.NoError(t, os.Mkdir(filepath.Join(dir, "fts"), 0o755))

	out := runInit(t)

	info, err := os.Stat(filepath.Join(dir, "fts"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.Contains(t, out, "fts/ already exists")
}

// @ft:3
func TestInit_InitializesSQLiteDatabase(t *testing.T) {
	dir := inTempDir(t)
	out := runInit(t)

	dbPath := filepath.Join(dir, "fts", "ft.db")
	_, err := os.Stat(dbPath)
	require.NoError(t, err)

	fx := dbtest.Open(t, dbPath)
	assert.Equal(t, "wal", fx.JournalMode())
	assert.Contains(t, out, "fts/ft.db created")
}

// @ft:4
func TestInit_DatabaseAlreadyExists(t *testing.T) {
	inTempDir(t)
	runInit(t)

	out := runInit(t)
	assert.Contains(t, out, "fts/ft.db already exists")
}

// @ft:5
func TestInit_AddsMigrationSystem(t *testing.T) {
	inTempDir(t)
	runInit(t)

	fx := dbtest.Open(t, "fts/ft.db")
	assert.Equal(t, 6, fx.SchemaVersion())
}

// @ft:6
func TestInit_AddsToGitignore(t *testing.T) {
	dir := inTempDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules\n"), 0o644))

	out := runInit(t)

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "fts/ft.db\n")
	assert.Contains(t, string(data), "node_modules\n")
	assert.Contains(t, out, "fts/ft.db added to .gitignore")
}

// @ft:7
func TestInit_GitignoreAlreadyHasEntry(t *testing.T) {
	dir := inTempDir(t)
	original := "node_modules\nfts/ft.db\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(original), 0o644))

	out := runInit(t)

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	require.NoError(t, err)
	assert.Equal(t, original, string(data))
	assert.Contains(t, out, "fts/ft.db already in .gitignore")
}

// @ft:8
func TestInit_NoGitignoreExists(t *testing.T) {
	dir := inTempDir(t)
	out := runInit(t)

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	require.NoError(t, err)
	assert.Equal(t, "fts/ft.db\n", string(data))
	assert.Contains(t, out, ".gitignore created")
	assert.Contains(t, out, "fts/ft.db added to .gitignore")
}

// @ft:201
func TestInit_CreatesStatusesFile(t *testing.T) {
	dir := inTempDir(t)
	out := runInit(t)

	data, err := os.ReadFile(filepath.Join(dir, "fts", "statuses.csv"))
	require.NoError(t, err)
	assert.Equal(t, "id,status\n", string(data))
	assert.Contains(t, out, "fts/statuses.csv created")
}

// @ft:202
func TestInit_StatusesFileAlreadyExists(t *testing.T) {
	dir := inTempDir(t)
	runInit(t)

	statusesPath := filepath.Join(dir, "fts", "statuses.csv")
	require.NoError(t, os.WriteFile(statusesPath, []byte("id,status\n1,accepted\n"), 0o644))

	out := runInit(t)

	data, err := os.ReadFile(statusesPath)
	require.NoError(t, err)
	assert.Equal(t, "id,status\n1,accepted\n", string(data))
	assert.Contains(t, out, "fts/statuses.csv already exists")
}

// @ft:203
func TestInit_StatusesFileNotAddedToGitignore(t *testing.T) {
	dir := inTempDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules\n"), 0o644))

	runInit(t)

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "statuses.csv")
}

// @ft:237
func TestInit_DoesNotMentionAgentInstructions(t *testing.T) {
	inTempDir(t)

	out := runInit(t)

	assert.NotContains(t, out, "agent-instructions")
}
