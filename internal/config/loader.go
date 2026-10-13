package config

import (
	"os"
)

// LoadOrInit loads the config from path or creates a default if missing.
func LoadOrInit(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := Default()
		if err := cfg.Save(path); err != nil {
			return nil, err
		}
		return cfg, nil
	}
	return Load(path)
}
