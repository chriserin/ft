package db

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

const statusesFileName = "statuses.csv"

var statusesHeader = []string{"id", "status"}

// StatusesPath returns the project-relative path to the statuses file.
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
	return WriteStatusesFile(nil)
}

// StatusRow is a scenario's id and its current status, as recorded in the
// statuses file — a snapshot, not a history of every transition.
type StatusRow struct {
	ScenarioID int64
	Status     string
}

// ReadStatusesFile parses every row in the statuses file, sorted by id.
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
		if len(rec) != 2 {
			continue
		}
		id, err := strconv.ParseInt(rec[0], 10, 64)
		if err != nil {
			continue
		}
		rows = append(rows, StatusRow{ScenarioID: id, Status: rec[1]})
	}
	return rows, nil
}

// WriteStatusesFile overwrites the statuses file with the given rows,
// preceded by the header row, sorted by id. Used both to create an empty
// file and to backfill it from the DB's current statuses.
func WriteStatusesFile(rows []StatusRow) error {
	sorted := make([]StatusRow, len(rows))
	copy(sorted, rows)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ScenarioID < sorted[j].ScenarioID })

	tmpPath := StatusesPath() + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	w := csv.NewWriter(f)
	if err := w.Write(statusesHeader); err != nil {
		f.Close()
		return err
	}
	for _, r := range sorted {
		if err := w.Write([]string{strconv.FormatInt(r.ScenarioID, 10), r.Status}); err != nil {
			f.Close()
			return err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	return os.Rename(tmpPath, StatusesPath())
}

// upsertStatusRow updates the given scenario's row in the statuses file, or
// inserts a new one in sorted position if it doesn't have one yet. Creates
// the file first if it doesn't exist.
func upsertStatusRow(id int64, status string) error {
	rows, err := ReadStatusesFile()
	if err != nil {
		return err
	}

	found := false
	for i, r := range rows {
		if r.ScenarioID == id {
			rows[i].Status = status
			found = true
			break
		}
	}
	if !found {
		rows = append(rows, StatusRow{ScenarioID: id, Status: status})
	}

	return WriteStatusesFile(rows)
}
