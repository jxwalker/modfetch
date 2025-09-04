package downloader

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"modfetch/internal/logging"
)

// adjustSafetensors trims any trailing bytes beyond what the safetensors header declares.
// If the file is shorter than required, returns an error.
// Returns true when the file was modified (truncated).
func adjustSafetensors(path string, log *logging.Logger) (bool, error) {
	if !strings.HasSuffix(strings.ToLower(path), ".safetensors") && !strings.HasSuffix(strings.ToLower(path), ".sft") {
		return false, nil
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil {
		return false, err
	}
	if fi.Size() < 8 {
		return false, fmt.Errorf("safetensors: file too small: %d", fi.Size())
	}
	// Read header length
	var hdrLen uint64
	if _, err := f.Seek(0, 0); err != nil {
		return false, err
	}
	if err := binary.Read(f, binary.LittleEndian, &hdrLen); err != nil {
		return false, err
	}
	if hdrLen == 0 || hdrLen > uint64(fi.Size()-8) || hdrLen > 64*1024*1024 {
		return false, fmt.Errorf("safetensors: invalid header len %d", hdrLen)
	}
	hdr := make([]byte, int(hdrLen))
	if _, err := io.ReadFull(f, hdr); err != nil {
		return false, err
	}
	var meta map[string]any
	if err := json.Unmarshal(hdr, &meta); err != nil {
		return false, fmt.Errorf("safetensors: header json: %w", err)
	}
	// Compute data segment length and validate against offsets
	dataLen := fi.Size() - 8 - int64(hdrLen)
	if dataLen < 0 {
		return false, fmt.Errorf("safetensors: negative data length")
	}
	// Compute max data end from top-level tensor entries
	var maxEnd int64 = 0
	for k, v := range meta {
		lk := strings.ToLower(k)
		if lk == "metadata" || lk == "__metadata__" {
			continue
		}
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		off, ok := m["data_offsets"].([]any)
		if !ok || len(off) != 2 {
			continue
		}
		_, ok1 := toInt64(off[0])
		end, ok2 := toInt64(off[1])
		if !ok1 || !ok2 {
			continue
		}
		if end > dataLen {
			return false, fmt.Errorf("safetensors: data end %d beyond data length %d", end, dataLen)
		}
		if end > maxEnd {
			maxEnd = end
		}
	}
	req := int64(8) + int64(hdrLen) + maxEnd
	if req < 0 {
		return false, fmt.Errorf("safetensors: computed size invalid")
	}
	if fi.Size() < req {
		return false, fmt.Errorf("safetensors: file incomplete: have=%d need=%d", fi.Size(), req)
	}
	if fi.Size() > req {
		if err := f.Truncate(req); err != nil {
			return false, err
		}
		if log != nil {
			log.Warnf("safetensors: truncated trailing %d bytes", fi.Size()-req)
		}
		return true, nil
	}
	return false, nil
}

// deepVerifySafetensors validates header coverage, data offsets and exact file length match.
// Returns ok, declared_total_size, error.
func deepVerifySafetensors(path string) (bool, int64, error) {
	if !strings.HasSuffix(strings.ToLower(path), ".safetensors") && !strings.HasSuffix(strings.ToLower(path), ".sft") {
		return true, 0, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return false, 0, err
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil {
		return false, 0, err
	}
	if fi.Size() < 8 {
		return false, 0, fmt.Errorf("file too small: %d", fi.Size())
	}
	var hdrLen uint64
	if err := binary.Read(f, binary.LittleEndian, &hdrLen); err != nil {
		return false, 0, err
	}
	if hdrLen == 0 || hdrLen > uint64(fi.Size()-8) || hdrLen > 64*1024*1024 {
		return false, 0, fmt.Errorf("invalid safetensors header length: %d", hdrLen)
	}
	hdr := make([]byte, int(hdrLen))
	if _, err := io.ReadFull(f, hdr); err != nil {
		return false, 0, err
	}
	var meta map[string]any
	if err := json.Unmarshal(hdr, &meta); err != nil {
		return false, 0, fmt.Errorf("header json: %w", err)
	}
	dataLen := fi.Size() - 8 - int64(hdrLen)
	if dataLen < 0 {
		return false, 0, fmt.Errorf("negative data length")
	}
	var maxEnd int64
	for k, v := range meta {
		lk := strings.ToLower(k)
		if lk == "metadata" || lk == "__metadata__" {
			continue
		}
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		off, ok := m["data_offsets"].([]any)
		if !ok || len(off) != 2 {
			continue
		}
		start, ok1 := toInt64(off[0])
		end, ok2 := toInt64(off[1])
		if !ok1 || !ok2 {
			return false, 0, fmt.Errorf("tensor %q: invalid data_offsets", k)
		}
		if start < 0 || end < 0 || end < start {
			return false, 0, fmt.Errorf("tensor %q: invalid range %d-%d", k, start, end)
		}
		if end > dataLen {
			return false, 0, fmt.Errorf("tensor %q: data end %d beyond data length %d", k, end, dataLen)
		}
		if dtRaw, ok := m["dtype"].(string); ok {
			if shp, ok := m["shape"].([]any); ok {
				exp := dtypeBytes(dtRaw)
				if exp > 0 {
					var cnt int64 = 1
					for _, dim := range shp {
						d, ok := toInt64(dim)
						if !ok || d <= 0 {
							cnt = 0
							break
						}
						cnt *= d
					}
					if cnt > 0 {
						sz := end - start
						if sz != cnt*exp {
							return false, 0, fmt.Errorf("tensor %q: size %d != %d*%d", k, sz, cnt, exp)
						}
					}
				}
			}
		}
		if end > maxEnd {
			maxEnd = end
		}
	}
	declared := int64(8) + int64(hdrLen) + maxEnd
	if fi.Size() < declared {
		return false, declared, fmt.Errorf("incomplete: have=%d need=%d", fi.Size(), declared)
	}
	if fi.Size() > declared {
		return false, declared, fmt.Errorf("extra bytes: have=%d need=%d", fi.Size(), declared)
	}
	return true, declared, nil
}

func dtypeBytes(dt string) int64 {
	switch strings.ToUpper(strings.TrimSpace(dt)) {
	case "F64":
		return 8
	case "F32":
		return 4
	case "F16", "BF16":
		return 2
	case "F8", "F8_E4M3FN", "F8_E5M2":
		return 1
	case "I64":
		return 8
	case "I32":
		return 4
	case "I16":
		return 2
	case "I8", "U8", "BOOL":
		return 1
	default:
		return 0
	}
}

func toInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case float64:
		return int64(x), true
	case int64:
		return x, true
	case int:
		return int64(x), true
	default:
		return 0, false
	}
}
