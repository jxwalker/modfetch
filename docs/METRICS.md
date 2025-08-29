# Metrics

modfetch can emit Prometheus textfile metrics when enabled via config. Point a node_exporter textfile collector to the configured directory.

Enable
```yaml
metrics:
  prometheus_textfile:
    enabled: true
    path: "/var/lib/node_exporter/textfile_collector/modfetch.prom"
```

Metrics
- modfetch_bytes_downloaded_total (counter)
- modfetch_retries_total (counter)
- modfetch_downloads_success_total (counter)
- modfetch_last_download_seconds (gauge)
- modfetch_active_downloads (gauge)
- modfetch_metrics_timestamp_seconds (gauge)

Example scrape
```text
# HELP modfetch_bytes_downloaded_total Total bytes downloaded.
# TYPE modfetch_bytes_downloaded_total counter
modfetch_bytes_downloaded_total 123456
# HELP modfetch_retries_total Total chunk retries.
# TYPE modfetch_retries_total counter
modfetch_retries_total 2
# HELP modfetch_downloads_success_total Total successful downloads.
# TYPE modfetch_downloads_success_total counter
modfetch_downloads_success_total 1
# HELP modfetch_last_download_seconds Duration of the last completed download in seconds.
# TYPE modfetch_last_download_seconds gauge
modfetch_last_download_seconds 12.345
# HELP modfetch_active_downloads Number of active downloads.
# TYPE modfetch_active_downloads gauge
modfetch_active_downloads 0
# HELP modfetch_metrics_timestamp_seconds UNIX timestamp when this file was written.
# TYPE modfetch_metrics_timestamp_seconds gauge
modfetch_metrics_timestamp_seconds 1730000000
```

