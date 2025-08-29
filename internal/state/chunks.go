package state

import (
	"errors"
)

type ChunkRow struct {
	URL      string
	Dest     string
	Index    int
	Start    int64
	End      int64
	Size     int64
	SHA256   string
	Status   string // pending | running | complete | dirty
}

func init() {}

func (db *DB) InitChunksTable() error {
	if db == nil || db.SQL == nil { return errors.New("nil db") }
	_, err := db.SQL.Exec(`CREATE TABLE IF NOT EXISTS chunks (
		url TEXT NOT NULL,
		dest TEXT NOT NULL,
		idx INTEGER NOT NULL,
		start INTEGER NOT NULL,
		end INTEGER NOT NULL,
		size INTEGER NOT NULL,
		sha256 TEXT,
		status TEXT NOT NULL,
		updated_at INTEGER NOT NULL,
		UNIQUE(url, dest, idx)
	);
	CREATE INDEX IF NOT EXISTS idx_chunks_url_dest ON chunks(url, dest);`)
	return err
}

func (db *DB) UpsertChunk(c ChunkRow) error {
	_, err := db.SQL.Exec(`INSERT INTO chunks(url,dest,idx,start,end,size,sha256,status,updated_at) VALUES(?,?,?,?,?,?,?,?,strftime('%s','now'))
	ON CONFLICT(url,dest,idx) DO UPDATE SET start=excluded.start,end=excluded.end,size=excluded.size,sha256=excluded.sha256,status=excluded.status,updated_at=strftime('%s','now')`,
		c.URL, c.Dest, c.Index, c.Start, c.End, c.Size, c.SHA256, c.Status)
	return err
}

func (db *DB) ListChunks(url, dest string) ([]ChunkRow, error) {
	rows, err := db.SQL.Query(`SELECT url,dest,idx,start,end,size,sha256,status FROM chunks WHERE url=? AND dest=? ORDER BY idx`, url, dest)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []ChunkRow
	for rows.Next() {
		var c ChunkRow
		if err := rows.Scan(&c.URL, &c.Dest, &c.Index, &c.Start, &c.End, &c.Size, &c.SHA256, &c.Status); err != nil { return nil, err }
		out = append(out, c)
	}
	return out, rows.Err()
}

func (db *DB) UpdateChunkStatus(url, dest string, idx int, status string) error {
	_, err := db.SQL.Exec(`UPDATE chunks SET status=?, updated_at=strftime('%s','now') WHERE url=? AND dest=? AND idx=?`, status, url, dest, idx)
	return err
}

func (db *DB) UpdateChunkSHA(url, dest string, idx int, sha string) error {
	_, err := db.SQL.Exec(`UPDATE chunks SET sha256=?, updated_at=strftime('%s','now') WHERE url=? AND dest=? AND idx=?`, sha, url, dest, idx)
	return err
}

