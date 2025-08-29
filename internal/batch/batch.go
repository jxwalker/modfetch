package batch

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type File struct {
	Version int        `yaml:"version"`
	Jobs    []BatchJob `yaml:"jobs"`
}

type BatchJob struct {
	URI      string `yaml:"uri"`
	Dest     string `yaml:"dest"`
	SHA256   string `yaml:"sha256"`
	Type     string `yaml:"type"`
	Place    bool   `yaml:"place"`
	Mode     string `yaml:"mode"` // symlink|hardlink|copy
}

func Load(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil { return nil, err }
	var f File
	if err := yaml.Unmarshal(b, &f); err != nil { return nil, err }
	if f.Version != 1 { return nil, fmt.Errorf("unsupported batch version: %d", f.Version) }
	if len(f.Jobs) == 0 { return nil, fmt.Errorf("batch has no jobs") }
	return &f, nil
}

