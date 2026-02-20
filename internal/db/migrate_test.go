package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestMigrate_CreatesSchemaVersionTable(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, Migrate(db))

	var name string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='schema_version'`).Scan(&name)
	require.NoError(t, err)
	assert.Equal(t, "schema_version", name)
}

func TestMigrate_InitializesVersionToZero(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, Migrate(db))

	var version int
	require.NoError(t, db.QueryRow(`SELECT version FROM schema_version`).Scan(&version))
	assert.Equal(t, 0, version)
}

func TestMigrate_RunsPendingMigrations(t *testing.T) {
	origAll := All
	defer func() { All = origAll }()

	All = []string{
		`CREATE TABLE test_one (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE test_two (id INTEGER PRIMARY KEY)`,
	}

	db := openTestDB(t)
	require.NoError(t, Migrate(db))

	var version int
	require.NoError(t, db.QueryRow(`SELECT version FROM schema_version`).Scan(&version))
	assert.Equal(t, 2, version)

	// Verify tables were created
	var name string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='test_one'`).Scan(&name)
	require.NoError(t, err)
	err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='test_two'`).Scan(&name)
	require.NoError(t, err)
}

func TestMigrate_SkipsAlreadyAppliedMigrations(t *testing.T) {
	origAll := All
	defer func() { All = origAll }()

	All = []string{
		`CREATE TABLE test_idem (id INTEGER PRIMARY KEY)`,
	}

	db := openTestDB(t)
	require.NoError(t, Migrate(db))
	require.NoError(t, Migrate(db))

	var version int
	require.NoError(t, db.QueryRow(`SELECT version FROM schema_version`).Scan(&version))
	assert.Equal(t, 1, version)
}

func TestMigrate_RollsBackOnFailure(t *testing.T) {
	origAll := All
	defer func() { All = origAll }()

	All = []string{
		`CREATE TABLE test_good (id INTEGER PRIMARY KEY)`,
		`INVALID SQL STATEMENT`,
	}

	db := openTestDB(t)
	err := Migrate(db)
	require.Error(t, err)

	var version int
	require.NoError(t, db.QueryRow(`SELECT version FROM schema_version`).Scan(&version))
	assert.Equal(t, 1, version)
}
