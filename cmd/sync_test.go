package cmd

import (
	"bytes"
	"os"
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
	assert.Equal(t, 1, version)
}
