package tui

import (
	"os"
)

func tryWrite(dir string) error {
	f, err := os.CreateTemp(dir, ".mf-wr-*")
	if err != nil {
		return err
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return nil
}
