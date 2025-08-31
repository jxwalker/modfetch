package main

import "strings"

// hostIs returns true if h equals root or is a subdomain of root.
// It trims trailing dots and compares case-insensitively.
func hostIs(h, root string) bool {
	_h := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(h)), ".")
	_r := strings.ToLower(strings.TrimSpace(root))
	return _h == _r || strings.HasSuffix(_h, "."+_r)
}
