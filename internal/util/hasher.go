package util

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// HashFileSHA256 computes the SHA256 of a file in a streaming fashion
// using a 1 MiB buffer to reduce syscall overhead without large memory use.
func HashFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	return HashReaderSHA256(f)
}

// HashReaderSHA256 computes SHA256 from an io.Reader using a 1 MiB buffer.
func HashReaderSHA256(r io.Reader) (string, error) {
	h := sha256.New()
	buf := make([]byte, 1<<20) // 1 MiB
	if _, err := io.CopyBuffer(h, r, buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
