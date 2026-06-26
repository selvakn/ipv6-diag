package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite is single-writer
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS reports (
			id                  TEXT PRIMARY KEY,
			device_name         TEXT NOT NULL,
			device_model        TEXT NOT NULL,
			device_manufacturer TEXT NOT NULL,
			android_version     TEXT NOT NULL,
			device_id           TEXT NOT NULL,
			network_json        TEXT NOT NULL,
			test_results_json   TEXT NOT NULL,
			xlat_summary_json   TEXT,
			pass_count          INTEGER NOT NULL,
			total_count         INTEGER NOT NULL,
			run_timestamp       INTEGER NOT NULL,
			uploaded_at         INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_reports_uploaded_at ON reports(uploaded_at);
		CREATE INDEX IF NOT EXISTS idx_reports_device_name ON reports(device_name);
	`)
	return err
}
