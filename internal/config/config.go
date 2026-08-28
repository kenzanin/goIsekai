// Package config loads and saves the reader's INI configuration file. It is
// deliberately dependency-free: the format is small enough that a hand-rolled
// parser is shorter and safer than pulling in an INI library.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds the reader's persisted settings.
type Config struct {
	// [app]
	DataDir string
	Title   string
	Width   int
	Height  int

	// [network] — default headers injected into plugin HTTP requests.
	UserAgent      string
	AcceptLanguage string
	Referer        string
}

// Default returns the built-in defaults.
func Default() *Config {
	return &Config{
		DataDir:        "app_data",
		Title:          "goIsekai",
		Width:          1200,
		Height:         800,
		UserAgent:      "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36",
		AcceptLanguage: "en-US,en;q=0.9",
		Referer:        "",
	}
}

// Load reads the INI file at path, applying defaults for any missing or
// invalid keys. A missing file is not an error: it yields the default config.
func Load(path string) (*Config, error) {
	c := Default()
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	defer f.Close()

	section := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue // not a key=value line; skip silently
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		c.set(section, key, val)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return c, nil
}

// Save writes the config to path as INI. It does not create parent
// directories; the caller is expected to point it at an existing directory.
func (c *Config) Save(path string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "[app]\n")
	fmt.Fprintf(&b, "data_dir = %s\n", c.DataDir)
	fmt.Fprintf(&b, "title = %s\n", c.Title)
	fmt.Fprintf(&b, "width = %d\n", c.Width)
	fmt.Fprintf(&b, "height = %d\n", c.Height)
	fmt.Fprintf(&b, "\n[network]\n")
	fmt.Fprintf(&b, "user_agent = %s\n", c.UserAgent)
	fmt.Fprintf(&b, "accept_language = %s\n", c.AcceptLanguage)
	fmt.Fprintf(&b, "referer = %s\n", c.Referer)
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// set applies a single key=value pair under a section, normalizing the key to
// lowercase with '-' mapped to '_' so "User-Agent" and "user_agent" both work.
// Unknown keys and invalid integers are ignored (the default is kept).
func (c *Config) set(section, key, val string) {
	key = strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	switch section {
	case "app":
		switch key {
		case "data_dir":
			c.DataDir = val
		case "title":
			c.Title = val
		case "width":
			if n, err := strconv.Atoi(val); err == nil {
				c.Width = n
			}
		case "height":
			if n, err := strconv.Atoi(val); err == nil {
				c.Height = n
			}
		}
	case "network":
		switch key {
		case "user_agent":
			c.UserAgent = val
		case "accept_language":
			c.AcceptLanguage = val
		case "referer":
			c.Referer = val
		}
	}
}
