package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DataDir is the project-relative directory that holds ft's data.
const DataDir = "fts"

const dbFileName = "ft.db"

// ErrNotInitialized is returned by OpenProjectStore when the project
// has not been set up with `ft init` yet.
var ErrNotInitialized = errors.New("run `ft init` first")

// Path returns the project-relative path to the database file.
func Path() string {
	return filepath.Join(DataDir, dbFileName)
}

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}

	if err := Migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// DataDirExists reports whether the project's data directory has been created.
func DataDirExists() bool {
	_, err := os.Stat(DataDir)
	return err == nil
}

// EnsureDataDir creates the project's data directory if it does not already exist.
func EnsureDataDir() error {
	return os.MkdirAll(DataDir, 0o755)
}

// FileExists reports whether the project's database file has been created.
func FileExists() bool {
	_, err := os.Stat(Path())
	return err == nil
}

// OpenProjectStore opens the project's database, wrapped in a Store.
// It returns ErrNotInitialized if the project has not been set up with `ft init`.
func OpenProjectStore() (*Store, error) {
	if !DataDirExists() {
		return nil, ErrNotInitialized
	}

	sqlDB, err := Open(Path())
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	return NewStore(sqlDB), nil
}
