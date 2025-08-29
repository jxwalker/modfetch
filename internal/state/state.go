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
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout=5000&_fk=1", path)
	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil { return nil, err }
	if err := initSchema(sqldb); err != nil { return nil, err }
	return &DB{SQL: sqldb, Path: path}, nil
}

func initSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS downloads (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url TEXT NOT NULL,
			dest TEXT NOT NULL,
			expected_sha256 TEXT,
			etag TEXT,
			last_modified TEXT,
			size INTEGER,
			status TEXT,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			UNIQUE(url, dest)
		);`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil { return err }
	}
	return nil
}

type DownloadRow struct {
	URL            string
	Dest           string
	ExpectedSHA256 string
	ETag           string
	LastModified   string
	Size           int64
	Status         string
	UpdatedAt      int64
}

func (db *DB) UpsertDownload(row DownloadRow) error {
	now := time.Now().Unix()
	_, err := db.SQL.Exec(`INSERT INTO downloads(url, dest, expected_sha256, etag, last_modified, size, status, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?)
		ON CONFLICT(url, dest) DO UPDATE SET expected_sha256=excluded.expected_sha256, etag=excluded.etag, last_modified=excluded.last_modified, size=excluded.size, status=excluded.status, updated_at=?`,
		row.URL, row.Dest, row.ExpectedSHA256, row.ETag, row.LastModified, row.Size, row.Status, now, now, now)
	return err
}

