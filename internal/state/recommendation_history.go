package state

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

type RecommendationHistoryRow struct {
	Task         string `json:"task"`
	Query        string `json:"query"`
	Provider     string `json:"provider"`
	ModelID      string `json:"model_id"`
	URI          string `json:"uri"`
	Action       string `json:"action"`
	Score        int    `json:"score"`
	Fit          string `json:"fit"`
	HardwareKey  string `json:"hardware_key"`
	Count        int    `json:"count"`
	LastSelected int64  `json:"last_selected,omitempty"`
	LastSkipped  int64  `json:"last_skipped,omitempty"`
	LastShown    int64  `json:"last_shown,omitempty"`
	UpdatedAt    int64  `json:"updated_at"`
}

func (db *DB) InitRecommendationHistoryTable() error {
	if db == nil || db.SQL == nil {
		return errors.New("nil db")
	}
	_, err := db.SQL.Exec(`CREATE TABLE IF NOT EXISTS recommendation_history (
		task TEXT NOT NULL,
		query TEXT NOT NULL,
		provider TEXT NOT NULL,
		model_id TEXT NOT NULL,
		uri TEXT NOT NULL,
		action TEXT NOT NULL,
		score INTEGER NOT NULL,
		fit TEXT NOT NULL,
		hardware_key TEXT NOT NULL,
		count INTEGER NOT NULL DEFAULT 1,
		last_selected INTEGER,
		last_skipped INTEGER,
		last_shown INTEGER,
		updated_at INTEGER NOT NULL,
		UNIQUE(task, query, uri, action, hardware_key)
	);
	CREATE INDEX IF NOT EXISTS idx_recommendation_history_lookup ON recommendation_history(task, query, uri, hardware_key);
	CREATE INDEX IF NOT EXISTS idx_recommendation_history_updated_at ON recommendation_history(updated_at);`)
	return err
}

func (db *DB) UpsertRecommendationHistory(row RecommendationHistoryRow) error {
	if db == nil || db.SQL == nil {
		return errors.New("nil db")
	}
	row.Task = strings.ToLower(strings.TrimSpace(row.Task))
	row.Query = strings.TrimSpace(row.Query)
	row.Provider = strings.ToLower(strings.TrimSpace(row.Provider))
	row.ModelID = strings.TrimSpace(row.ModelID)
	row.URI = strings.TrimSpace(row.URI)
	row.Action = strings.ToLower(strings.TrimSpace(row.Action))
	row.Fit = strings.ToLower(strings.TrimSpace(row.Fit))
	row.HardwareKey = strings.ToLower(strings.TrimSpace(row.HardwareKey))
	if row.Task == "" || row.Query == "" || row.URI == "" || row.Action == "" || row.HardwareKey == "" {
		return errors.New("recommendation history task, query, uri, action, and hardware key required")
	}
	if row.Provider == "" {
		row.Provider = "unknown"
	}
	if row.Fit == "" {
		row.Fit = "unknown"
	}
	if row.Count <= 0 {
		row.Count = 1
	}
	now := time.Now().Unix()
	switch row.Action {
	case "selected":
		row.LastSelected = now
	case "skipped":
		row.LastSkipped = now
	default:
		row.Action = "shown"
		row.LastShown = now
	}
	_, err := db.SQL.Exec(`INSERT INTO recommendation_history(task, query, provider, model_id, uri, action, score, fit, hardware_key, count, last_selected, last_skipped, last_shown, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(task, query, uri, action, hardware_key) DO UPDATE SET
			provider=excluded.provider,
			model_id=excluded.model_id,
			score=excluded.score,
			fit=excluded.fit,
			count=recommendation_history.count + excluded.count,
			last_selected=COALESCE(excluded.last_selected, recommendation_history.last_selected),
			last_skipped=COALESCE(excluded.last_skipped, recommendation_history.last_skipped),
			last_shown=COALESCE(excluded.last_shown, recommendation_history.last_shown),
			updated_at=excluded.updated_at`,
		row.Task, row.Query, row.Provider, row.ModelID, row.URI, row.Action, row.Score, row.Fit, row.HardwareKey, row.Count, nullableUnix(row.LastSelected), nullableUnix(row.LastSkipped), nullableUnix(row.LastShown), now)
	if err == nil {
		db.NotifyChange()
	}
	return err
}

func (db *DB) RecommendationHistoryFor(task, query, hardwareKey string) ([]RecommendationHistoryRow, error) {
	if db == nil || db.SQL == nil {
		return nil, errors.New("nil db")
	}
	task = strings.ToLower(strings.TrimSpace(task))
	query = strings.TrimSpace(query)
	hardwareKey = strings.ToLower(strings.TrimSpace(hardwareKey))
	if task == "" || query == "" || hardwareKey == "" {
		return nil, nil
	}
	rows, err := db.SQL.Query(`SELECT task, query, provider, model_id, uri, action, score, fit, hardware_key, count,
			COALESCE(last_selected, 0), COALESCE(last_skipped, 0), COALESCE(last_shown, 0), updated_at
		FROM recommendation_history
		WHERE task=? AND query=? AND hardware_key=?
		ORDER BY updated_at DESC`, task, query, hardwareKey)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanRecommendationHistory(rows)
}

func (db *DB) ListRecommendationHistory(limit int) ([]RecommendationHistoryRow, error) {
	if db == nil || db.SQL == nil {
		return nil, errors.New("nil db")
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.SQL.Query(`SELECT task, query, provider, model_id, uri, action, score, fit, hardware_key, count,
			COALESCE(last_selected, 0), COALESCE(last_skipped, 0), COALESCE(last_shown, 0), updated_at
		FROM recommendation_history
		ORDER BY updated_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanRecommendationHistory(rows)
}

func scanRecommendationHistory(rows *sql.Rows) ([]RecommendationHistoryRow, error) {
	var out []RecommendationHistoryRow
	for rows.Next() {
		var row RecommendationHistoryRow
		if err := rows.Scan(&row.Task, &row.Query, &row.Provider, &row.ModelID, &row.URI, &row.Action, &row.Score, &row.Fit, &row.HardwareKey, &row.Count, &row.LastSelected, &row.LastSkipped, &row.LastShown, &row.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func nullableUnix(v int64) any {
	if v <= 0 {
		return nil
	}
	return v
}
