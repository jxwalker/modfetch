package state

import (
	"database/sql"
	"errors"
)

type HostCaps struct {
	Host          string
	HeadOK        bool
	AcceptRanges  bool
}

func (db *DB) InitHostCapsTable() error {
	if db == nil || db.SQL == nil { return errors.New("nil db") }
	_, err := db.SQL.Exec(`CREATE TABLE IF NOT EXISTS host_caps (
		host TEXT PRIMARY KEY,
		head_ok INTEGER NOT NULL,
		accept_ranges INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);`)
	return err
}

func (db *DB) UpsertHostCaps(host string, headOK, acceptRanges bool) error {
	if db == nil || db.SQL == nil { return errors.New("nil db") }
	_, err := db.SQL.Exec(`INSERT INTO host_caps(host, head_ok, accept_ranges, updated_at) VALUES(?,?,?,strftime('%s','now'))
	ON CONFLICT(host) DO UPDATE SET head_ok=excluded.head_ok, accept_ranges=excluded.accept_ranges, updated_at=strftime('%s','now')`,
		host, boolToInt(headOK), boolToInt(acceptRanges))
	return err
}

func (db *DB) GetHostCaps(host string) (HostCaps, bool, error) {
	if db == nil || db.SQL == nil { return HostCaps{}, false, errors.New("nil db") }
	var hc HostCaps
	var headOK, acc int
	row := db.SQL.QueryRow(`SELECT head_ok, accept_ranges FROM host_caps WHERE host=?`, host)
	switch err := row.Scan(&headOK, &acc); err {
	case sql.ErrNoRows:
		return HostCaps{}, false, nil
	case nil:
		hc = HostCaps{Host: host, HeadOK: headOK != 0, AcceptRanges: acc != 0}
		return hc, true, nil
	default:
		return HostCaps{}, false, err
	}
}

// ListHostCaps returns all cached host capabilities.
func (db *DB) ListHostCaps() ([]HostCaps, error) {
	if db == nil || db.SQL == nil { return nil, errors.New("nil db") }
	rows, err := db.SQL.Query(`SELECT host, head_ok, accept_ranges FROM host_caps ORDER BY host`)
	if err != nil { return nil, err }
	defer func() { _ = rows.Close() }()
	var out []HostCaps
	for rows.Next() {
		var host string
		var head, acc int
		if err := rows.Scan(&host, &head, &acc); err != nil { return nil, err }
		out = append(out, HostCaps{Host: host, HeadOK: head != 0, AcceptRanges: acc != 0})
	}
	return out, rows.Err()
}

// DeleteHostCaps deletes a single host from the cache.
func (db *DB) DeleteHostCaps(host string) error {
	if db == nil || db.SQL == nil { return errors.New("nil db") }
	_, err := db.SQL.Exec(`DELETE FROM host_caps WHERE host=?`, host)
	return err
}

// ClearHostCaps deletes all host capabilities from the cache.
func (db *DB) ClearHostCaps() error {
	if db == nil || db.SQL == nil { return errors.New("nil db") }
	_, err := db.SQL.Exec(`DELETE FROM host_caps`)
	return err
}

func boolToInt(b bool) int { if b { return 1 }; return 0 }

