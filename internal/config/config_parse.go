package config

import (
	"os"
	"strings"
	"time"

	"gopkg.in/ini.v1"
)

// Load reads the INI file at path, applying defaults for any missing or
// invalid keys. A missing file is not an error: it yields the default config.
func Load(path string) (*Config, error) {
	c := Default()
	f, err := ini.LoadSources(ini.LoadOptions{IgnoreInlineComment: true}, path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	// Iterate rather than look up by key so that set() keeps applying the
	// per-key validation and the alias/enhance sections verbatim.
	for _, s := range f.Sections() {
		for _, k := range s.Keys() {
			c.set(s.Name(), k.Name(), k.String())
		}
	}
	return c, nil
}

// Watch polls path's mtime every interval. When the file changes it re-reads
// the config and calls onChange with the fresh value. It returns a stop
// function that terminates the background goroutine.
func Watch(path string, interval time.Duration, onChange func(*Config)) (stop func()) {
	done := make(chan struct{})
	var lastMod time.Time
	if fi, err := os.Stat(path); err == nil {
		lastMod = fi.ModTime()
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				fi, err := os.Stat(path)
				if err != nil {
					continue
				}
				if !fi.ModTime().After(lastMod) {
					continue
				}
				lastMod = fi.ModTime()
				cfg, err := Load(path)
				if err != nil {
					continue
				}
				onChange(cfg)
			}
		}
	}()
	return func() { close(done) }
}

// addEnhanceMode records one `[enhance]` line. "default" sets the global mode;
// any other key overrides it for that plugin ID. An unrecognized value is
// ignored, so a typo leaves the previous (safer) mode in place.
func (c *Config) addEnhanceMode(key, val string) {
	mode := strings.ToLower(strings.TrimSpace(val))
	if mode != "auto" && mode != "off" {
		return
	}
	if strings.EqualFold(key, "default") {
		c.EnhanceDefault = mode
		return
	}
	if c.EnhancePlugins == nil {
		c.EnhancePlugins = map[string]string{}
	}
	c.EnhancePlugins[strings.ToLower(key)] = mode
}

// WorkerPoolConfig maps the [workers] INI section onto the workers pool
// sizing (zero values mean "use code default" there).
func (c *Config) WorkerPoolConfig() WorkersSizing {
	return WorkersSizing{
		InteractiveSize:  c.WorkersInteractiveSize,
		InteractiveQueue: c.WorkersInteractiveQueue,
		FetchSize:        c.WorkersFetchSize,
		FetchQueue:       c.WorkersFetchQueue,
		ImageSize:        c.WorkersImageSize,
		ImageQueue:       c.WorkersImageQueue,
		MaintenanceSize:  c.WorkersMaintenanceSize,
		MaintenanceQueue: c.WorkersMaintenanceQueue,
	}
}

// WorkersSizing is the config-facing view of workers.Config lane sizes.
type WorkersSizing struct {
	InteractiveSize  int
	InteractiveQueue int
	FetchSize        int
	FetchQueue       int
	ImageSize        int
	ImageQueue       int
	MaintenanceSize  int
	MaintenanceQueue int
}
