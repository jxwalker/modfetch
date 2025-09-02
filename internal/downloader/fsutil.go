package downloader

import (
	"os"
)

// writeAndSync writes content to path and fsyncs the file.
func writeAndSync(path string, b []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil { return err }
	defer func() { _ = f.Close() }()
	if _, err := f.Write(b); err != nil { return err }
	return f.Sync()
}

func fsyncDir(dir string) error {
	df, err := os.Open(dir)
	if err != nil { return err }
	defer func() { _ = df.Close() }()
	return df.Sync()
}

