package catalog

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"time"

	"github.com/jxwalker/modfetch/internal/state"
)

const Version = 1

type Catalog struct {
	App                string         `json:"app"`
	CatalogVersion     int            `json:"catalog_version"`
	StateSchemaVersion int            `json:"state_schema_version"`
	ExportedAt         time.Time      `json:"exported_at"`
	Models             []CatalogEntry `json:"models"`
}

type CatalogEntry struct {
	Metadata state.ModelMetadata `json:"metadata"`
	Download *DownloadSnapshot   `json:"download,omitempty"`
}

type DownloadSnapshot struct {
	URL            string `json:"url"`
	Dest           string `json:"dest"`
	ExpectedSHA256 string `json:"expected_sha256,omitempty"`
	ActualSHA256   string `json:"actual_sha256,omitempty"`
	Size           int64  `json:"size,omitempty"`
	Status         string `json:"status,omitempty"`
}

type ImportOptions struct {
	DryRun bool
}

type ImportResult struct {
	DryRun    bool                `json:"dry_run"`
	Creates   int                 `json:"creates"`
	Updates   int                 `json:"updates"`
	Skips     int                 `json:"skips"`
	Conflicts int                 `json:"conflicts"`
	Entries   []ImportEntryResult `json:"entries"`
}

type ImportEntryResult struct {
	DownloadURL string `json:"download_url"`
	Dest        string `json:"dest,omitempty"`
	Action      string `json:"action"`
	Reason      string `json:"reason,omitempty"`
}

func Export(db *state.DB, w io.Writer) error {
	cat, err := Build(db)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(cat)
}

func Build(db *state.DB) (*Catalog, error) {
	if db == nil {
		return nil, fmt.Errorf("nil db")
	}
	metadataRows, err := db.ListMetadata(state.MetadataFilters{OrderBy: "name"})
	if err != nil {
		return nil, fmt.Errorf("list metadata: %w", err)
	}
	downloads, err := db.ListDownloads()
	if err != nil {
		return nil, fmt.Errorf("list downloads: %w", err)
	}
	byURLDest := map[string]state.DownloadRow{}
	for _, row := range downloads {
		byURLDest[downloadKey(row.URL, row.Dest)] = row
	}

	entries := make([]CatalogEntry, 0, len(metadataRows))
	for _, meta := range metadataRows {
		entry := CatalogEntry{Metadata: meta}
		if row, ok := byURLDest[downloadKey(meta.DownloadURL, meta.Dest)]; ok {
			entry.Download = snapshotDownload(row)
		}
		entries = append(entries, entry)
	}

	return &Catalog{
		App:                "modfetch",
		CatalogVersion:     Version,
		StateSchemaVersion: state.SchemaVersion,
		ExportedAt:         time.Now().UTC(),
		Models:             entries,
	}, nil
}

func Import(db *state.DB, r io.Reader, opts ImportOptions) (*ImportResult, error) {
	if db == nil {
		return nil, fmt.Errorf("nil db")
	}
	var cat Catalog
	dec := json.NewDecoder(r)
	if err := dec.Decode(&cat); err != nil {
		return nil, fmt.Errorf("decode catalog: %w", err)
	}
	if cat.App != "" && cat.App != "modfetch" {
		return nil, fmt.Errorf("unsupported catalog app %q", cat.App)
	}
	if cat.CatalogVersion > Version {
		return nil, fmt.Errorf("catalog version %d is newer than supported version %d", cat.CatalogVersion, Version)
	}

	existingDownloads, err := db.ListDownloads()
	if err != nil {
		return nil, fmt.Errorf("list downloads: %w", err)
	}
	downloadsByKey := map[string]state.DownloadRow{}
	for _, row := range existingDownloads {
		downloadsByKey[downloadKey(row.URL, row.Dest)] = row
	}
	existingMetadata, err := db.ListMetadata(state.MetadataFilters{})
	if err != nil {
		return nil, fmt.Errorf("list metadata: %w", err)
	}
	metadataByURL := map[string]state.ModelMetadata{}
	metadataByDest := map[string]state.ModelMetadata{}
	for _, meta := range existingMetadata {
		metadataByURL[meta.DownloadURL] = meta
		if meta.Dest != "" {
			metadataByDest[meta.Dest] = meta
		}
	}

	result := &ImportResult{DryRun: opts.DryRun}
	for _, entry := range cat.Models {
		action := importOne(db, downloadsByKey, metadataByURL, metadataByDest, entry, opts)
		result.Entries = append(result.Entries, action)
		switch action.Action {
		case "create":
			result.Creates++
		case "update":
			result.Updates++
		case "skip":
			result.Skips++
		case "conflict":
			result.Conflicts++
		}
	}
	return result, nil
}

func importOne(db *state.DB, downloadsByKey map[string]state.DownloadRow, metadataByURL, metadataByDest map[string]state.ModelMetadata, entry CatalogEntry, opts ImportOptions) ImportEntryResult {
	meta := entry.Metadata
	res := ImportEntryResult{DownloadURL: meta.DownloadURL, Dest: meta.Dest}
	if meta.DownloadURL == "" {
		res.Action = "conflict"
		res.Reason = "metadata download_url is required"
		return res
	}
	if entry.Download != nil {
		if entry.Download.URL != "" && entry.Download.URL != meta.DownloadURL {
			res.Action = "conflict"
			res.Reason = fmt.Sprintf("download snapshot URL %s does not match metadata URL", entry.Download.URL)
			return res
		}
		if meta.Dest == "" && entry.Download.Dest != "" {
			meta.Dest = entry.Download.Dest
			res.Dest = meta.Dest
		}
		if meta.Dest != "" && entry.Download.Dest != "" && entry.Download.Dest != meta.Dest {
			res.Action = "conflict"
			res.Reason = fmt.Sprintf("download snapshot destination %s does not match metadata destination", entry.Download.Dest)
			return res
		}
	}
	if meta.Dest != "" {
		byDest, ok := metadataByDest[meta.Dest]
		if ok && byDest.DownloadURL != meta.DownloadURL {
			res.Action = "conflict"
			res.Reason = fmt.Sprintf("destination already belongs to %s", byDest.DownloadURL)
			return res
		}
	}

	existing, found := metadataByURL[meta.DownloadURL]
	if !found {
		res.Action = "create"
	} else if metadataEqual(existing, meta) && downloadEqual(downloadsByKey, entry) {
		res.Action = "skip"
		return res
	} else {
		res.Action = "update"
	}

	row, hasDownload := importDownloadRow(downloadsByKey, entry)
	if opts.DryRun {
		applyImportMaps(downloadsByKey, metadataByURL, metadataByDest, meta, row, hasDownload)
		return res
	}
	err := db.WithTx(func(tx *sql.Tx) error {
		if err := db.UpsertMetadataTx(tx, &meta); err != nil {
			return fmt.Errorf("upsert metadata: %w", err)
		}
		if hasDownload {
			if err := db.UpsertDownloadTx(tx, row); err != nil {
				return fmt.Errorf("upsert download: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		res.Action = "conflict"
		res.Reason = err.Error()
		return res
	}
	applyImportMaps(downloadsByKey, metadataByURL, metadataByDest, meta, row, hasDownload)
	return res
}

func importDownloadRow(downloadsByKey map[string]state.DownloadRow, entry CatalogEntry) (state.DownloadRow, bool) {
	if entry.Download != nil && entry.Download.URL != "" && entry.Download.Dest != "" {
		row := state.DownloadRow{
			URL:            entry.Download.URL,
			Dest:           entry.Download.Dest,
			ExpectedSHA256: entry.Download.ExpectedSHA256,
			ActualSHA256:   entry.Download.ActualSHA256,
			Size:           entry.Download.Size,
			Status:         entry.Download.Status,
		}
		if row.Status == "" {
			if existingRow, ok := downloadsByKey[downloadKey(row.URL, row.Dest)]; ok {
				row.Status = existingRow.Status
			} else {
				row.Status = "imported"
			}
		}
		return row, true
	}
	return state.DownloadRow{}, false
}

func applyImportMaps(downloadsByKey map[string]state.DownloadRow, metadataByURL, metadataByDest map[string]state.ModelMetadata, meta state.ModelMetadata, row state.DownloadRow, hasDownload bool) {
	if existing, ok := metadataByURL[meta.DownloadURL]; ok && existing.Dest != "" && existing.Dest != meta.Dest {
		delete(metadataByDest, existing.Dest)
	}
	metadataByURL[meta.DownloadURL] = meta
	if meta.Dest != "" {
		metadataByDest[meta.Dest] = meta
	}
	if hasDownload {
		downloadsByKey[downloadKey(row.URL, row.Dest)] = row
	}
}

func snapshotDownload(row state.DownloadRow) *DownloadSnapshot {
	return &DownloadSnapshot{
		URL:            row.URL,
		Dest:           row.Dest,
		ExpectedSHA256: row.ExpectedSHA256,
		ActualSHA256:   row.ActualSHA256,
		Size:           row.Size,
		Status:         row.Status,
	}
}

func downloadEqual(downloadsByKey map[string]state.DownloadRow, entry CatalogEntry) bool {
	if entry.Download == nil || entry.Download.URL == "" || entry.Download.Dest == "" {
		return true
	}
	row, ok := downloadsByKey[downloadKey(entry.Download.URL, entry.Download.Dest)]
	if !ok {
		return false
	}
	statusMatches := entry.Download.Status == "" || row.Status == entry.Download.Status
	return row.ExpectedSHA256 == entry.Download.ExpectedSHA256 &&
		row.ActualSHA256 == entry.Download.ActualSHA256 &&
		row.Size == entry.Download.Size &&
		statusMatches
}

func metadataEqual(a, b state.ModelMetadata) bool {
	a.ID, b.ID = 0, 0
	a.CreatedAt, b.CreatedAt = time.Time{}, time.Time{}
	a.UpdatedAt, b.UpdatedAt = time.Time{}, time.Time{}
	return reflect.DeepEqual(a, b)
}

func downloadKey(url, dest string) string {
	return url + "\x00" + dest
}
