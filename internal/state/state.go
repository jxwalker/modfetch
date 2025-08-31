package state

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/glebarez/sqlite"
	"modfetch/internal/config"
)

type DB struct {
	SQL *sql.DB
	Path string
}

func Open(cfg *config.Config) (*DB, error) {
	if cfg == nil { return nil, errors.New("nil config") }
	if cfg.General.DataRoot == "" { return nil, errors.New("general.data_root required") }
	if err := os.MkdirAll(cfg.General.DataRoot, 0o755); err != nil { return nil, err }
path := filepath.Join(cfg.General.DataRoot, "state.db")
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout=5000&_pragma=journal_mode(WAL)&_fk=1", path)
	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil { return nil, err }
	if err := initSchema(sqldb); err != nil { return nil, err }
	db := &DB{SQL: sqldb, Path: path}
	if err := db.InitChunksTable(); err != nil { return nil, err }
	if err := db.InitHostCapsTable(); err != nil { return nil, err }
	return db, nil
}

func initSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS downloads (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url TEXT NOT NULL,
			dest TEXT NOT NULL,
			expected_sha256 TEXT,
			actual_sha256 TEXT,
			etag TEXT,
			last_modified TEXT,
			size INTEGER,
			status TEXT,
			retries INTEGER DEFAULT 0,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			last_error TEXT,
			UNIQUE(url, dest)
		);`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil { return err }
	}
	// Try to add new columns in case of existing DB without them
	_, _ = db.Exec(`ALTER TABLE downloads ADD COLUMN actual_sha256 TEXT`)
	_, _ = db.Exec(`ALTER TABLE downloads ADD COLUMN retries INTEGER DEFAULT 0`)
	_, _ = db.Exec(`ALTER TABLE downloads ADD COLUMN last_error TEXT`)
	return nil
}

type DownloadRow struct {
	URL            string
	Dest           string
	ExpectedSHA256 string
	ActualSHA256   string
	ETag           string
	LastModified   string
	Size           int64
	Status         string
	Retries        int64
	UpdatedAt      int64
	LastError      string
}

func (db *DB) UpsertDownload(row DownloadRow) error {
	now := time.Now().Unix()
	_, err := db.SQL.Exec(`INSERT INTO downloads(url, dest, expected_sha256, actual_sha256, etag, last_modified, size, status, last_error, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(url, dest) DO UPDATE SET expected_sha256=excluded.expected_sha256, actual_sha256=excluded.actual_sha256, etag=excluded.etag, last_modified=excluded.last_modified, size=excluded.size, status=excluded.status, last_error=excluded.last_error, updated_at=?`,
		row.URL, row.Dest, row.ExpectedSHA256, row.ActualSHA256, row.ETag, row.LastModified, row.Size, row.Status, row.LastError, now, now, now)
	return err
}

// IncDownloadRetries increments the retries counter for a download row.
func (db *DB) IncDownloadRetries(url, dest string, delta int64) error {
	if delta == 0 { return nil }
	_, err := db.SQL.Exec(`UPDATE downloads SET retries = COALESCE(retries,0) + ?, updated_at=strftime('%s','now') WHERE url=? AND dest=?`, delta, url, dest)
	return err
}

// DeleteDownload removes a download row for the given url+dest.
func (db *DB) DeleteDownload(url, dest string) error {
	_, err := db.SQL.Exec(`DELETE FROM downloads WHERE url=? AND dest=?`, url, dest)
	return err
}

// ListDownloads returns a snapshot of the downloads table
func (db *DB) ListDownloads() ([]DownloadRow, error) {
rows, err := db.SQL.Query(`SELECT url, dest,
    COALESCE(expected_sha256, ''),
    COALESCE(actual_sha256, ''),
    COALESCE(etag, ''),
    COALESCE(last_modified, ''),
    COALESCE(size, 0),
    COALESCE(status, ''),
    COALESCE(retries, 0),
    updated_at,
    COALESCE(last_error, '')
  FROM downloads
  ORDER BY updated_at DESC`)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []DownloadRow
	for rows.Next() {
		var r DownloadRow
		if err := rows.Scan(&r.URL, &r.Dest, &r.ExpectedSHA256, &r.ActualSHA256, &r.ETag, &r.LastModified, &r.Size, &r.Status, &r.Retries, &r.UpdatedAt, &r.LastError); err != nil { return nil, err }
		out = append(out, r)
	}
	return out, rows.Err()
}

