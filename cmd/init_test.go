package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chriserin/ft/internal/db"
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

func TestInit_CreatesFtsDirectory(t *testing.T) {
	dir := inTempDir(t)
	out := runInit(t)

	info, err := os.Stat(filepath.Join(dir, "fts"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.Contains(t, out, "fts/ created")
}

func TestInit_FtsDirectoryAlreadyExists(t *testing.T) {
	dir := inTempDir(t)
	require.NoError(t, os.Mkdir(filepath.Join(dir, "fts"), 0o755))

	out := runInit(t)

	info, err := os.Stat(filepath.Join(dir, "fts"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.Contains(t, out, "fts/ already exists")
}

func TestInit_InitializesSQLiteDatabase(t *testing.T) {
	dir := inTempDir(t)
	out := runInit(t)

	dbPath := filepath.Join(dir, "fts", "ft.db")
	_, err := os.Stat(dbPath)
	require.NoError(t, err)

	sqlDB, err := db.Open(dbPath)
	require.NoError(t, err)
	defer sqlDB.Close()

	var mode string
	require.NoError(t, sqlDB.QueryRow("PRAGMA journal_mode").Scan(&mode))
	assert.Equal(t, "wal", mode)
	assert.Contains(t, out, "fts/ft.db created")
}

func TestInit_DatabaseAlreadyExists(t *testing.T) {
	inTempDir(t)
	runInit(t)

	out := runInit(t)
	assert.Contains(t, out, "fts/ft.db already exists")
}

func TestInit_AddsMigrationSystem(t *testing.T) {
	inTempDir(t)
	runInit(t)

	sqlDB, err := db.Open("fts/ft.db")
	require.NoError(t, err)
	defer sqlDB.Close()

	var version int
	require.NoError(t, sqlDB.QueryRow("SELECT version FROM schema_version").Scan(&version))
	assert.Equal(t, 5, version)
}

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

func TestInit_NoGitignoreExists(t *testing.T) {
	dir := inTempDir(t)
	out := runInit(t)

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	require.NoError(t, err)
	assert.Equal(t, "fts/ft.db\n", string(data))
	assert.Contains(t, out, ".gitignore created")
	assert.Contains(t, out, "fts/ft.db added to .gitignore")
}
