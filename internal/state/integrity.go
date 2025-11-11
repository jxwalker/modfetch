package state

import (
	"database/sql"
	"fmt"
)

// CheckIntegrity runs SQLite's integrity check on the database
func (db *DB) CheckIntegrity() error {
	if db == nil || db.SQL == nil {
		return fmt.Errorf("database not open")
	}

	var result string
	err := db.SQL.QueryRow("PRAGMA integrity_check").Scan(&result)
	if err != nil {
		return fmt.Errorf("integrity check failed to run: %w", err)
	}

	if result != "ok" {
		return fmt.Errorf("database integrity check failed: %s", result)
	}

	return nil
}

// CheckOrphans checks for and counts orphaned records
func (db *DB) CheckOrphans() (orphanedChunks int, err error) {
	if db == nil || db.SQL == nil {
		return 0, fmt.Errorf("database not open")
	}

	// Count orphaned chunks (chunks without corresponding downloads)
	err = db.SQL.QueryRow(`
		SELECT COUNT(*) FROM chunks c
		WHERE NOT EXISTS (
			SELECT 1 FROM downloads d
			WHERE d.url = c.url AND d.dest = c.dest
		)
	`).Scan(&orphanedChunks)

	if err != nil {
		return 0, fmt.Errorf("failed to count orphaned chunks: %w", err)
	}

	return orphanedChunks, nil
}

// RepairOrphans removes orphaned chunks
func (db *DB) RepairOrphans() (int, error) {
	if db == nil || db.SQL == nil {
		return 0, fmt.Errorf("database not open")
	}

	// Delete orphaned chunks
	result, err := db.SQL.Exec(`
		DELETE FROM chunks WHERE NOT EXISTS (
			SELECT 1 FROM downloads d
			WHERE d.url = chunks.url AND d.dest = chunks.dest
		)
	`)

	if err != nil {
		return 0, fmt.Errorf("failed to delete orphaned chunks: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	return int(rowsAffected), nil
}

// Vacuum optimizes the database by reclaiming unused space
func (db *DB) Vacuum() error {
	if db == nil || db.SQL == nil {
		return fmt.Errorf("database not open")
	}

	_, err := db.SQL.Exec("VACUUM")
	if err != nil {
		return fmt.Errorf("vacuum failed: %w", err)
	}

	return nil
}

// Backup creates a backup of the database
func (db *DB) Backup(destPath string) error {
	if db == nil || db.SQL == nil {
		return fmt.Errorf("database not open")
	}

	// Use SQLite backup API via VACUUM INTO
	query := fmt.Sprintf("VACUUM INTO '%s'", destPath)
	_, err := db.SQL.Exec(query)
	if err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	return nil
}

// GetStats returns database statistics
type DBStats struct {
	DatabaseSize     int64 // Size in bytes
	Downloads        int
	Chunks           int
	Metadata         int
	OrphanedChunks   int
	CompletedDownloads int
	FailedDownloads   int
}

// GetStats retrieves database statistics
func (db *DB) GetStats() (*DBStats, error) {
	if db == nil || db.SQL == nil {
		return nil, fmt.Errorf("database not open")
	}

	stats := &DBStats{}

	// Get file size
	if fileInfo, err := sql.Open("sqlite", db.Path); err == nil {
		var pageCount, pageSize int
		if err := fileInfo.QueryRow("PRAGMA page_count").Scan(&pageCount); err == nil {
			if err := fileInfo.QueryRow("PRAGMA page_size").Scan(&pageSize); err == nil {
				stats.DatabaseSize = int64(pageCount * pageSize)
			}
		}
		fileInfo.Close()
	}

	// Count downloads
	if err := db.SQL.QueryRow("SELECT COUNT(*) FROM downloads").Scan(&stats.Downloads); err != nil {
		return nil, fmt.Errorf("failed to count downloads: %w", err)
	}

	// Count chunks
	if err := db.SQL.QueryRow("SELECT COUNT(*) FROM chunks").Scan(&stats.Chunks); err != nil {
		return nil, fmt.Errorf("failed to count chunks: %w", err)
	}

	// Count metadata
	if err := db.SQL.QueryRow("SELECT COUNT(*) FROM metadata").Scan(&stats.Metadata); err != nil {
		// Table might not exist
		stats.Metadata = 0
	}

	// Count completed downloads
	if err := db.SQL.QueryRow("SELECT COUNT(*) FROM downloads WHERE status = 'complete'").Scan(&stats.CompletedDownloads); err != nil {
		return nil, fmt.Errorf("failed to count completed downloads: %w", err)
	}

	// Count failed downloads
	if err := db.SQL.QueryRow("SELECT COUNT(*) FROM downloads WHERE status IN ('error', 'failed', 'checksum_mismatch')").Scan(&stats.FailedDownloads); err != nil {
		return nil, fmt.Errorf("failed to count failed downloads: %w", err)
	}

	// Count orphaned chunks
	orphans, err := db.CheckOrphans()
	if err != nil {
		return nil, err
	}
	stats.OrphanedChunks = orphans

	return stats, nil
}
