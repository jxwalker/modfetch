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

func boolToInt(b bool) int { if b { return 1 }; return 0 }

