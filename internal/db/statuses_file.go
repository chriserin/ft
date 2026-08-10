package db

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
)

const statusesFileName = "statuses.csv"

var statusesHeader = []string{"id", "status", "changed_at"}

// StatusesPath returns the project-relative path to the statuses log file.
func StatusesPath() string {
	return filepath.Join(DataDir, statusesFileName)
}

// StatusesFileExists reports whether the project's statuses file has been created.
func StatusesFileExists() bool {
	_, err := os.Stat(StatusesPath())
	return err == nil
}

// EnsureStatusesFile creates the statuses file with its header row if it
// doesn't already exist. It's a no-op if the file is already present.
func EnsureStatusesFile() error {
	if StatusesFileExists() {
		return nil
	}

	f, err := os.Create(StatusesPath())
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(statusesHeader); err != nil {
		return err
	}
	w.Flush()
	return w.Error()
}

// StatusRow is a single parsed entry from the statuses file.
type StatusRow struct {
	ScenarioID int64
	Status     string
	ChangedAt  string
}

// ReadStatusesFile parses every entry in the statuses file, in file order.
// It returns no rows (and no error) if the file doesn't exist.
func ReadStatusesFile() ([]StatusRow, error) {
	f, err := os.Open(StatusesPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}

	var rows []StatusRow
	for _, rec := range records[1:] { // skip header
		if len(rec) != 3 {
			continue
		}
		id, err := strconv.ParseInt(rec[0], 10, 64)
		if err != nil {
			continue
		}
		rows = append(rows, StatusRow{ScenarioID: id, Status: rec[1], ChangedAt: rec[2]})
	}
	return rows, nil
}

// WriteStatusesFile overwrites the statuses file with the given rows,
// preceded by the header row. Used to backfill the file from the DB the
// first time it's created for a project that already has status history.
func WriteStatusesFile(rows []StatusRow) error {
	f, err := os.Create(StatusesPath())
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(statusesHeader); err != nil {
		return err
	}
	for _, r := range rows {
		if err := w.Write([]string{strconv.FormatInt(r.ScenarioID, 10), r.Status, r.ChangedAt}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

// appendStatusRow appends a single status event to the statuses file,
// creating it with a header first if it doesn't already exist.
func appendStatusRow(id int64, status, changedAt string) error {
	if err := EnsureStatusesFile(); err != nil {
		return err
	}

	f, err := os.OpenFile(StatusesPath(), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write([]string{strconv.FormatInt(id, 10), status, changedAt}); err != nil {
		return err
	}
	w.Flush()
	return w.Error()
}
