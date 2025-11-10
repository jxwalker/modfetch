package state

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ModelMetadata stores rich metadata about downloaded models
type ModelMetadata struct {
	ID int64 `json:"id"`

	// Link to downloads table
	DownloadURL string `json:"download_url"`
	Dest        string `json:"dest,omitempty"` // Destination file path

	// Basic model info
	ModelName string `json:"model_name,omitempty"`
	ModelID   string `json:"model_id,omitempty"` // e.g., "TheBloke/Llama-2-7B-GGUF"
	Version   string `json:"version,omitempty"`  // e.g., "Q4_K_M", "fp16"
	Source    string `json:"source,omitempty"`   // "huggingface", "civitai", "direct"

	// Metadata
	Description string   `json:"description,omitempty"`
	Author      string   `json:"author,omitempty"`
	AuthorURL   string   `json:"author_url,omitempty"`
	License     string   `json:"license,omitempty"`
	Tags        []string `json:"tags,omitempty"`

	// Model specs
	ModelType      string `json:"model_type,omitempty"`      // "LLM", "LoRA", "Checkpoint", "VAE", "Embedding"
	BaseModel      string `json:"base_model,omitempty"`      // "llama-2", "sdxl", etc.
	Architecture   string `json:"architecture,omitempty"`    // "transformer", "unet", etc.
	ParameterCount string `json:"parameter_count,omitempty"` // "7B", "13B", "70B"
	Quantization   string `json:"quantization,omitempty"`    // "Q4_K_M", "Q5_K_S", "fp16"

	// File info
	FileSize   int64  `json:"file_size,omitempty"`
	FileFormat string `json:"file_format,omitempty"` // "gguf", "safetensors", "ckpt", "bin"

	// Usage stats
	DownloadCount int        `json:"download_count"`
	LastUsed      *time.Time `json:"last_used,omitempty"`
	TimesUsed     int        `json:"times_used"`

	// External links
	HomepageURL      string `json:"homepage_url,omitempty"`
	RepoURL          string `json:"repo_url,omitempty"`
	DocumentationURL string `json:"documentation_url,omitempty"`
	ThumbnailURL     string `json:"thumbnail_url,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// User data
	UserNotes  string `json:"user_notes,omitempty"`
	UserRating int    `json:"user_rating,omitempty"` // 1-5 stars
	Favorite   bool   `json:"favorite"`
}

// MetadataFilters provides filtering options for ListMetadata
type MetadataFilters struct {
	Source    string   // Filter by source: "huggingface", "civitai", etc.
	ModelType string   // Filter by type: "LLM", "LoRA", "Checkpoint", etc.
	Favorite  bool     // Show only favorites
	MinRating int      // Minimum user rating (1-5)
	Tags      []string // Filter by tags
	OrderBy   string   // "last_used", "name", "size", "rating", "created_at"
	Limit     int      // Limit results
}

// InitMetadataTable creates the model_metadata table
func (db *DB) InitMetadataTable() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS model_metadata (
			id INTEGER PRIMARY KEY AUTOINCREMENT,

			-- Link to downloads
			download_url TEXT NOT NULL UNIQUE,
			dest TEXT,

			-- Basic info
			model_name TEXT,
			model_id TEXT,
			version TEXT,
			source TEXT,

			-- Metadata
			description TEXT,
			author TEXT,
			author_url TEXT,
			license TEXT,
			tags TEXT,  -- JSON array

			-- Model specs
			model_type TEXT,
			base_model TEXT,
			architecture TEXT,
			parameter_count TEXT,
			quantization TEXT,

			-- File info
			file_size INTEGER,
			file_format TEXT,

			-- Usage stats
			download_count INTEGER DEFAULT 0,
			last_used INTEGER,
			times_used INTEGER DEFAULT 0,

			-- External links
			homepage_url TEXT,
			repo_url TEXT,
			documentation_url TEXT,
			thumbnail_url TEXT,

			-- Timestamps
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,

			-- User data
			user_notes TEXT,
			user_rating INTEGER,
			favorite INTEGER DEFAULT 0
		);`,
		`CREATE INDEX IF NOT EXISTS idx_metadata_source ON model_metadata(source);`,
		`CREATE INDEX IF NOT EXISTS idx_metadata_type ON model_metadata(model_type);`,
		`CREATE INDEX IF NOT EXISTS idx_metadata_favorite ON model_metadata(favorite);`,
		`CREATE INDEX IF NOT EXISTS idx_metadata_last_used ON model_metadata(last_used);`,
		`CREATE INDEX IF NOT EXISTS idx_metadata_updated_at ON model_metadata(updated_at);`,
		`CREATE INDEX IF NOT EXISTS idx_metadata_dest ON model_metadata(dest);`,
		`CREATE INDEX IF NOT EXISTS idx_metadata_model_name ON model_metadata(model_name);`,
	}

	for _, stmt := range stmts {
		if _, err := db.SQL.Exec(stmt); err != nil {
			return fmt.Errorf("init metadata schema: %w", err)
		}
	}

	return nil
}

// UpsertMetadata inserts or updates model metadata
func (db *DB) UpsertMetadata(meta *ModelMetadata) error {
	if meta.DownloadURL == "" {
		return fmt.Errorf("download_url is required")
	}

	now := time.Now().Unix()
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = time.Unix(now, 0)
	}
	meta.UpdatedAt = time.Unix(now, 0)

	// Serialize tags to JSON
	tagsJSON, err := json.Marshal(meta.Tags)
	if err != nil {
		return fmt.Errorf("serialize tags: %w", err)
	}

	var lastUsedUnix *int64
	if meta.LastUsed != nil {
		lu := meta.LastUsed.Unix()
		lastUsedUnix = &lu
	}

	stmt := `INSERT INTO model_metadata(
		download_url, dest, model_name, model_id, version, source,
		description, author, author_url, license, tags,
		model_type, base_model, architecture, parameter_count, quantization,
		file_size, file_format,
		download_count, last_used, times_used,
		homepage_url, repo_url, documentation_url, thumbnail_url,
		created_at, updated_at,
		user_notes, user_rating, favorite
	) VALUES(?,?,?,?,?,?, ?,?,?,?,?, ?,?,?,?,?, ?,?, ?,?,?, ?,?,?,?, ?,?, ?,?,?)
	ON CONFLICT(download_url) DO UPDATE SET
		dest=excluded.dest,
		model_name=excluded.model_name,
		model_id=excluded.model_id,
		version=excluded.version,
		source=excluded.source,
		description=excluded.description,
		author=excluded.author,
		author_url=excluded.author_url,
		license=excluded.license,
		tags=excluded.tags,
		model_type=excluded.model_type,
		base_model=excluded.base_model,
		architecture=excluded.architecture,
		parameter_count=excluded.parameter_count,
		quantization=excluded.quantization,
		file_size=excluded.file_size,
		file_format=excluded.file_format,
		download_count=excluded.download_count,
		last_used=excluded.last_used,
		times_used=excluded.times_used,
		homepage_url=excluded.homepage_url,
		repo_url=excluded.repo_url,
		documentation_url=excluded.documentation_url,
		thumbnail_url=excluded.thumbnail_url,
		updated_at=excluded.updated_at,
		user_notes=excluded.user_notes,
		user_rating=excluded.user_rating,
		favorite=excluded.favorite`

	_, err = db.SQL.Exec(stmt,
		meta.DownloadURL, meta.Dest, meta.ModelName, meta.ModelID, meta.Version, meta.Source,
		meta.Description, meta.Author, meta.AuthorURL, meta.License, string(tagsJSON),
		meta.ModelType, meta.BaseModel, meta.Architecture, meta.ParameterCount, meta.Quantization,
		meta.FileSize, meta.FileFormat,
		meta.DownloadCount, lastUsedUnix, meta.TimesUsed,
		meta.HomepageURL, meta.RepoURL, meta.DocumentationURL, meta.ThumbnailURL,
		meta.CreatedAt.Unix(), meta.UpdatedAt.Unix(),
		meta.UserNotes, meta.UserRating, meta.Favorite,
	)

	return err
}

// GetMetadata retrieves metadata for a specific download URL
func (db *DB) GetMetadata(downloadURL string) (*ModelMetadata, error) {
	stmt := `SELECT
		id, download_url, dest, model_name, model_id, version, source,
		description, author, author_url, license, tags,
		model_type, base_model, architecture, parameter_count, quantization,
		file_size, file_format,
		download_count, last_used, times_used,
		homepage_url, repo_url, documentation_url, thumbnail_url,
		created_at, updated_at,
		user_notes, user_rating, favorite
	FROM model_metadata WHERE download_url = ?`

	var meta ModelMetadata
	var tagsJSON string
	var lastUsedUnix *int64
	var createdAtUnix, updatedAtUnix int64
	var favorite int

	err := db.SQL.QueryRow(stmt, downloadURL).Scan(
		&meta.ID, &meta.DownloadURL, &meta.Dest, &meta.ModelName, &meta.ModelID, &meta.Version, &meta.Source,
		&meta.Description, &meta.Author, &meta.AuthorURL, &meta.License, &tagsJSON,
		&meta.ModelType, &meta.BaseModel, &meta.Architecture, &meta.ParameterCount, &meta.Quantization,
		&meta.FileSize, &meta.FileFormat,
		&meta.DownloadCount, &lastUsedUnix, &meta.TimesUsed,
		&meta.HomepageURL, &meta.RepoURL, &meta.DocumentationURL, &meta.ThumbnailURL,
		&createdAtUnix, &updatedAtUnix,
		&meta.UserNotes, &meta.UserRating, &favorite,
	)
	if err != nil {
		return nil, err
	}

	// Deserialize tags
	if tagsJSON != "" {
		if err := json.Unmarshal([]byte(tagsJSON), &meta.Tags); err != nil {
			return nil, fmt.Errorf("deserialize tags: %w", err)
		}
	}

	// Convert timestamps
	meta.CreatedAt = time.Unix(createdAtUnix, 0)
	meta.UpdatedAt = time.Unix(updatedAtUnix, 0)
	if lastUsedUnix != nil {
		lu := time.Unix(*lastUsedUnix, 0)
		meta.LastUsed = &lu
	}
	meta.Favorite = favorite != 0

	return &meta, nil
}

// GetMetadataByDest retrieves metadata for a specific destination path
// This is optimized with an index for fast lookups by file path
func (db *DB) GetMetadataByDest(dest string) (*ModelMetadata, error) {
	if dest == "" {
		return nil, fmt.Errorf("dest path is required")
	}

	stmt := `SELECT
		id, download_url, dest, model_name, model_id, version, source,
		description, author, author_url, license, tags,
		model_type, base_model, architecture, parameter_count, quantization,
		file_size, file_format,
		download_count, last_used, times_used,
		homepage_url, repo_url, documentation_url, thumbnail_url,
		created_at, updated_at,
		user_notes, user_rating, favorite
	FROM model_metadata WHERE dest = ?`

	var meta ModelMetadata
	var tagsJSON string
	var lastUsedUnix *int64
	var createdAtUnix, updatedAtUnix int64
	var favorite int

	err := db.SQL.QueryRow(stmt, dest).Scan(
		&meta.ID, &meta.DownloadURL, &meta.Dest, &meta.ModelName, &meta.ModelID, &meta.Version, &meta.Source,
		&meta.Description, &meta.Author, &meta.AuthorURL, &meta.License, &tagsJSON,
		&meta.ModelType, &meta.BaseModel, &meta.Architecture, &meta.ParameterCount, &meta.Quantization,
		&meta.FileSize, &meta.FileFormat,
		&meta.DownloadCount, &lastUsedUnix, &meta.TimesUsed,
		&meta.HomepageURL, &meta.RepoURL, &meta.DocumentationURL, &meta.ThumbnailURL,
		&createdAtUnix, &updatedAtUnix,
		&meta.UserNotes, &meta.UserRating, &favorite,
	)
	if err == sql.ErrNoRows {
		return nil, nil // Not found - return nil without error
	}
	if err != nil {
		return nil, err
	}

	// Deserialize tags
	if tagsJSON != "" {
		if err := json.Unmarshal([]byte(tagsJSON), &meta.Tags); err != nil {
			return nil, fmt.Errorf("deserialize tags: %w", err)
		}
	}

	// Convert timestamps
	meta.CreatedAt = time.Unix(createdAtUnix, 0)
	meta.UpdatedAt = time.Unix(updatedAtUnix, 0)
	if lastUsedUnix != nil {
		lu := time.Unix(*lastUsedUnix, 0)
		meta.LastUsed = &lu
	}
	meta.Favorite = favorite != 0

	return &meta, nil
}

// ListMetadata retrieves metadata with optional filters
func (db *DB) ListMetadata(filters MetadataFilters) ([]ModelMetadata, error) {
	query := `SELECT
		id, download_url, dest, model_name, model_id, version, source,
		description, author, author_url, license, tags,
		model_type, base_model, architecture, parameter_count, quantization,
		file_size, file_format,
		download_count, last_used, times_used,
		homepage_url, repo_url, documentation_url, thumbnail_url,
		created_at, updated_at,
		user_notes, user_rating, favorite
	FROM model_metadata WHERE 1=1`

	var args []interface{}

	// Apply filters
	if filters.Source != "" {
		query += " AND source = ?"
		args = append(args, filters.Source)
	}
	if filters.ModelType != "" {
		query += " AND model_type = ?"
		args = append(args, filters.ModelType)
	}
	if filters.Favorite {
		query += " AND favorite = 1"
	}
	if filters.MinRating > 0 {
		query += " AND user_rating >= ?"
		args = append(args, filters.MinRating)
	}
	if len(filters.Tags) > 0 {
		// Search for any of the tags in the JSON array
		tagConditions := make([]string, len(filters.Tags))
		for i, tag := range filters.Tags {
			tagConditions[i] = "tags LIKE ?"
			args = append(args, "%"+tag+"%")
		}
		query += " AND (" + strings.Join(tagConditions, " OR ") + ")"
	}

	// Apply ordering
	switch filters.OrderBy {
	case "last_used":
		query += " ORDER BY last_used DESC NULLS LAST"
	case "name":
		query += " ORDER BY model_name COLLATE NOCASE"
	case "size":
		query += " ORDER BY file_size DESC"
	case "rating":
		query += " ORDER BY user_rating DESC NULLS LAST"
	case "created_at":
		query += " ORDER BY created_at DESC"
	default:
		query += " ORDER BY updated_at DESC"
	}

	// Apply limit
	if filters.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filters.Limit)
	}

	rows, err := db.SQL.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ModelMetadata
	for rows.Next() {
		var meta ModelMetadata
		var tagsJSON string
		var lastUsedUnix *int64
		var createdAtUnix, updatedAtUnix int64
		var favorite int

		err := rows.Scan(
			&meta.ID, &meta.DownloadURL, &meta.Dest, &meta.ModelName, &meta.ModelID, &meta.Version, &meta.Source,
			&meta.Description, &meta.Author, &meta.AuthorURL, &meta.License, &tagsJSON,
			&meta.ModelType, &meta.BaseModel, &meta.Architecture, &meta.ParameterCount, &meta.Quantization,
			&meta.FileSize, &meta.FileFormat,
			&meta.DownloadCount, &lastUsedUnix, &meta.TimesUsed,
			&meta.HomepageURL, &meta.RepoURL, &meta.DocumentationURL, &meta.ThumbnailURL,
			&createdAtUnix, &updatedAtUnix,
			&meta.UserNotes, &meta.UserRating, &favorite,
		)
		if err != nil {
			return nil, err
		}

		// Deserialize tags
		if tagsJSON != "" {
			if err := json.Unmarshal([]byte(tagsJSON), &meta.Tags); err != nil {
				return nil, fmt.Errorf("deserialize tags: %w", err)
			}
		}

		// Convert timestamps
		meta.CreatedAt = time.Unix(createdAtUnix, 0)
		meta.UpdatedAt = time.Unix(updatedAtUnix, 0)
		if lastUsedUnix != nil {
			lu := time.Unix(*lastUsedUnix, 0)
			meta.LastUsed = &lu
		}
		meta.Favorite = favorite != 0

		results = append(results, meta)
	}

	return results, rows.Err()
}

// UpdateMetadataUsage increments usage stats and updates last_used timestamp
func (db *DB) UpdateMetadataUsage(downloadURL string) error {
	stmt := `UPDATE model_metadata
		SET times_used = times_used + 1,
		    last_used = strftime('%s','now'),
		    updated_at = strftime('%s','now')
		WHERE download_url = ?`

	result, err := db.SQL.Exec(stmt, downloadURL)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// DeleteMetadata removes metadata for a specific download URL
func (db *DB) DeleteMetadata(downloadURL string) error {
	_, err := db.SQL.Exec(`DELETE FROM model_metadata WHERE download_url = ?`, downloadURL)
	return err
}

// SearchMetadata performs a full-text search across model metadata
func (db *DB) SearchMetadata(query string) ([]ModelMetadata, error) {
	searchPattern := "%" + query + "%"
	stmt := `SELECT
		id, download_url, dest, model_name, model_id, version, source,
		description, author, author_url, license, tags,
		model_type, base_model, architecture, parameter_count, quantization,
		file_size, file_format,
		download_count, last_used, times_used,
		homepage_url, repo_url, documentation_url, thumbnail_url,
		created_at, updated_at,
		user_notes, user_rating, favorite
	FROM model_metadata
	WHERE model_name LIKE ?
	   OR model_id LIKE ?
	   OR description LIKE ?
	   OR author LIKE ?
	   OR tags LIKE ?
	ORDER BY updated_at DESC`

	rows, err := db.SQL.Query(stmt, searchPattern, searchPattern, searchPattern, searchPattern, searchPattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ModelMetadata
	for rows.Next() {
		var meta ModelMetadata
		var tagsJSON string
		var lastUsedUnix *int64
		var createdAtUnix, updatedAtUnix int64
		var favorite int

		err := rows.Scan(
			&meta.ID, &meta.DownloadURL, &meta.Dest, &meta.ModelName, &meta.ModelID, &meta.Version, &meta.Source,
			&meta.Description, &meta.Author, &meta.AuthorURL, &meta.License, &tagsJSON,
			&meta.ModelType, &meta.BaseModel, &meta.Architecture, &meta.ParameterCount, &meta.Quantization,
			&meta.FileSize, &meta.FileFormat,
			&meta.DownloadCount, &lastUsedUnix, &meta.TimesUsed,
			&meta.HomepageURL, &meta.RepoURL, &meta.DocumentationURL, &meta.ThumbnailURL,
			&createdAtUnix, &updatedAtUnix,
			&meta.UserNotes, &meta.UserRating, &favorite,
		)
		if err != nil {
			return nil, err
		}

		// Deserialize tags
		if tagsJSON != "" {
			if err := json.Unmarshal([]byte(tagsJSON), &meta.Tags); err != nil {
				return nil, fmt.Errorf("deserialize tags: %w", err)
			}
		}

		// Convert timestamps
		meta.CreatedAt = time.Unix(createdAtUnix, 0)
		meta.UpdatedAt = time.Unix(updatedAtUnix, 0)
		if lastUsedUnix != nil {
			lu := time.Unix(*lastUsedUnix, 0)
			meta.LastUsed = &lu
		}
		meta.Favorite = favorite != 0

		results = append(results, meta)
	}

	return results, rows.Err()
}
