package hostnet

import (
	"fmt"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// rodConnect opens a rod browser from the CDP config. For "chrome" it launches
// cfg.Path headless; for "lightpanda"/"obscura" it attaches to the ws://
// daemon. The caller must Close the browser.
func rodConnect(cfg CDPConfig) (*rod.Browser, error) {
	if cfg.Engine == "lightpanda" || cfg.Engine == "obscura" {
		if !strings.HasPrefix(cfg.Path, "ws://") && !strings.HasPrefix(cfg.Path, "wss://") {
			return nil, fmt.Errorf("hostnet: %s cdp_path must be a ws:// URL, got %q", cfg.Engine, cfg.Path)
		}
		b := rod.New().ControlURL(cfg.Path)
		if err := b.Connect(); err != nil {
			return nil, fmt.Errorf("hostnet: rod connect %s: %w", cfg.Engine, err)
		}
		return b, nil
	}
	wsURL, err := launcher.New().
		Bin(cfg.Path).
		Headless(true).
		Set("no-first-run").
		Set("no-default-browser-check").
		Set("disable-gpu").
		Launch()
	if err != nil {
		return nil, fmt.Errorf("hostnet: rod launch chrome: %w", err)
	}
	b := rod.New().ControlURL(wsURL)
	if err := b.Connect(); err != nil {
		return nil, fmt.Errorf("hostnet: rod connect chrome: %w", err)
	}
	return b, nil
}
