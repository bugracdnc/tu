package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds user preferences that change the default behaviour of tu.
type Config struct {
	ShowPast bool `json:"show_past"`

	filepath string
}

func LoadConfig(path string) (*Config, error) {
	c := &Config{filepath: path}
	byt, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	if len(byt) > 0 {
		if err := json.Unmarshal(byt, c); err != nil {
			return nil, fmt.Errorf("config file %s is not valid JSON: %w", path, err)
		}
	}
	return c, nil
}

func (c *Config) Save() error {
	byt, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.filepath), 0755); err != nil {
		return err
	}
	return os.WriteFile(c.filepath, byt, 0644)
}
