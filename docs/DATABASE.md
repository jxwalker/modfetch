# Database schema

modfetch stores local state in SQLite at `general.data_root/state.db`.

## Schema version

The database uses SQLite `PRAGMA user_version` as the migration baseline. The current schema version is `1`.

## Tables

### downloads

Tracks download attempts and final artifacts. The logical key is `(url, dest)`.

Key columns:
- `url`, `dest`
- `expected_sha256`, `actual_sha256`
- `etag`, `last_modified`, `size`
- `status`, `retries`, `last_error`
- `created_at`, `updated_at`

Indexes:
- `idx_downloads_status`
- `idx_downloads_dest`
- `idx_downloads_updated_at`

### chunks

Tracks resumable chunk plans and per-chunk verification state. The logical key is `(url, dest, idx)`.

Key columns:
- `url`, `dest`, `idx`
- `start`, `end`, `size`
- `sha256`, `status`, `updated_at`

Indexes:
- `idx_chunks_url_dest`

### host_caps

Caches host capability probes so repeated downloads do not rediscover HEAD/range support on every run.

Key columns:
- `host`
- `head_ok`, `accept_ranges`
- `updated_at`

### model_metadata

Stores library metadata for downloaded or scanned models. The logical key is `download_url`.

Key columns:
- `download_url`, `dest`
- `model_name`, `model_id`, `version`, `source`
- `description`, `author`, `license`, `tags`
- `model_type`, `base_model`, `architecture`, `parameter_count`, `quantization`
- `file_size`, `file_format`
- `download_count`, `last_used`, `times_used`
- `homepage_url`, `repo_url`, `documentation_url`, `author_url`, `thumbnail_url`
- `user_notes`, `user_rating`, `favorite`
- `created_at`, `updated_at`

Indexes:
- `idx_metadata_source`
- `idx_metadata_type`
- `idx_metadata_favorite`
- `idx_metadata_last_used`
- `idx_metadata_updated_at`
- `idx_metadata_dest`
- `idx_metadata_model_name`

## Migration notes

Version `1` preserves the current table layout. Databases with `PRAGMA user_version = 0`
are migrated through an explicit v0-to-v1 step that adds the legacy `downloads`
columns introduced before the schema version baseline: `actual_sha256`,
`retries`, and `last_error`. Future v1.x migrations should advance
`PRAGMA user_version` and keep compatibility shims explicit rather than relying
on silent best-effort `ALTER TABLE` calls.
