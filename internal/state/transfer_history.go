package state

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

type TransferHistoryRow struct {
	Host        string  `json:"host"`
	Tool        string  `json:"tool"`
	Connections int     `json:"connections"`
	ChunkSizeMB int     `json:"chunk_size_mb"`
	AvgBPS      float64 `json:"avg_bps"`
	Samples     int     `json:"samples"`
	RateLimited bool    `json:"rate_limited"`
	LastStatus  string  `json:"last_status"`
	LastError   string  `json:"last_error,omitempty"`
	UpdatedAt   int64   `json:"updated_at"`
}

func (db *DB) InitTransferHistoryTable() error {
	if db == nil || db.SQL == nil {
		return errors.New("nil db")
	}
	_, err := db.SQL.Exec(`CREATE TABLE IF NOT EXISTS transfer_history (
		host TEXT NOT NULL,
		tool TEXT NOT NULL,
		connections INTEGER NOT NULL,
		chunk_size_mb INTEGER NOT NULL,
		avg_bps REAL NOT NULL,
		samples INTEGER NOT NULL,
		rate_limited INTEGER NOT NULL DEFAULT 0,
		last_status TEXT NOT NULL,
		last_error TEXT,
		updated_at INTEGER NOT NULL,
		UNIQUE(host, tool, connections, chunk_size_mb)
	);
	CREATE INDEX IF NOT EXISTS idx_transfer_history_host_tool ON transfer_history(host, tool);
	CREATE INDEX IF NOT EXISTS idx_transfer_history_updated_at ON transfer_history(updated_at);`)
	return err
}

func (db *DB) UpsertTransferHistory(row TransferHistoryRow) error {
	if db == nil || db.SQL == nil {
		return errors.New("nil db")
	}
	row.Host = strings.ToLower(strings.TrimSpace(row.Host))
	row.Tool = strings.ToLower(strings.TrimSpace(row.Tool))
	row.LastStatus = strings.ToLower(strings.TrimSpace(row.LastStatus))
	if row.Host == "" || row.Tool == "" {
		return errors.New("transfer history host and tool required")
	}
	if row.Connections <= 0 {
		row.Connections = 1
	}
	if row.ChunkSizeMB < 0 {
		row.ChunkSizeMB = 0
	}
	if row.AvgBPS < 0 {
		row.AvgBPS = 0
	}
	if row.LastStatus == "" {
		row.LastStatus = "unknown"
	}
	now := time.Now().Unix()
	_, err := db.SQL.Exec(`INSERT INTO transfer_history(host, tool, connections, chunk_size_mb, avg_bps, samples, rate_limited, last_status, last_error, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(host, tool, connections, chunk_size_mb) DO UPDATE SET
			avg_bps=((transfer_history.avg_bps * CASE WHEN transfer_history.samples >= 20 THEN 19 ELSE transfer_history.samples END) + excluded.avg_bps) /
				(CASE WHEN transfer_history.samples >= 20 THEN 20 ELSE transfer_history.samples + 1 END),
			samples=CASE WHEN transfer_history.samples >= 20 THEN 20 ELSE transfer_history.samples + 1 END,
			rate_limited=excluded.rate_limited,
			last_status=excluded.last_status,
			last_error=excluded.last_error,
			updated_at=excluded.updated_at`,
		row.Host, row.Tool, row.Connections, row.ChunkSizeMB, row.AvgBPS, 1, boolToInt(row.RateLimited), row.LastStatus, row.LastError, now)
	if err == nil {
		db.NotifyChange()
	}
	return err
}

func (db *DB) BestTransferHistory(host, tool string) (TransferHistoryRow, bool, error) {
	if db == nil || db.SQL == nil {
		return TransferHistoryRow{}, false, errors.New("nil db")
	}
	host = strings.ToLower(strings.TrimSpace(host))
	tool = strings.ToLower(strings.TrimSpace(tool))
	if host == "" || tool == "" {
		return TransferHistoryRow{}, false, nil
	}
	row := db.SQL.QueryRow(`SELECT host, tool, connections, chunk_size_mb, avg_bps, samples, rate_limited, last_status, COALESCE(last_error, ''), updated_at
		FROM transfer_history
		WHERE host=? AND tool=? AND avg_bps > 0 AND last_status != 'error'
		ORDER BY rate_limited ASC, avg_bps DESC, samples DESC, updated_at DESC
		LIMIT 1`, host, tool)
	var out TransferHistoryRow
	var rateLimited int
	switch err := row.Scan(&out.Host, &out.Tool, &out.Connections, &out.ChunkSizeMB, &out.AvgBPS, &out.Samples, &rateLimited, &out.LastStatus, &out.LastError, &out.UpdatedAt); err {
	case sql.ErrNoRows:
		return TransferHistoryRow{}, false, nil
	case nil:
		out.RateLimited = rateLimited != 0
		return out, true, nil
	default:
		return TransferHistoryRow{}, false, err
	}
}

func (db *DB) ListTransferHistory() ([]TransferHistoryRow, error) {
	if db == nil || db.SQL == nil {
		return nil, errors.New("nil db")
	}
	rows, err := db.SQL.Query(`SELECT host, tool, connections, chunk_size_mb, avg_bps, samples, rate_limited, last_status, COALESCE(last_error, ''), updated_at
		FROM transfer_history
		ORDER BY host, tool, rate_limited ASC, avg_bps DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []TransferHistoryRow
	for rows.Next() {
		var row TransferHistoryRow
		var rateLimited int
		if err := rows.Scan(&row.Host, &row.Tool, &row.Connections, &row.ChunkSizeMB, &row.AvgBPS, &row.Samples, &rateLimited, &row.LastStatus, &row.LastError, &row.UpdatedAt); err != nil {
			return nil, err
		}
		row.RateLimited = rateLimited != 0
		out = append(out, row)
	}
	return out, rows.Err()
}
