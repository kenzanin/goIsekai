package workers

import (
	"time"
)

// Config sizes the lanes. Zero values mean "use the code default".
type Config struct {
	InteractiveSize  int // worker count, default 4
	InteractiveQueue int
	FetchSize        int // default 2
	FetchQueue       int // default 8
	ImageSize        int // default 8
	ImageQueue       int // default 64 (shared by both priorities)
	MaintenanceSize  int // default 1 (serial)
	MaintenanceQueue int
	// BackgroundEnqueueTimeout bounds how long a background lane enqueue
	// blocks before ErrBusy. Default 500ms.
	BackgroundEnqueueTimeout time.Duration
}

const (
	defaultInteractiveSize  = 4
	defaultInteractiveQueue = 32
	defaultFetchSize        = 2
	defaultFetchQueue       = 8
	defaultImageSize        = 8
	defaultImageQueue       = 64
	defaultMaintenanceSize  = 1
	defaultMaintenanceQueue = 8
	defaultBGEnqueueTimeout = 500 * time.Millisecond
	defaultMaxAttempts      = 3
)

func (c Config) withDefaults() Config {
	if c.InteractiveSize <= 0 {
		c.InteractiveSize = defaultInteractiveSize
	}
	if c.InteractiveQueue <= 0 {
		c.InteractiveQueue = defaultInteractiveQueue
	}
	if c.FetchSize <= 0 {
		c.FetchSize = defaultFetchSize
	}
	if c.FetchQueue <= 0 {
		c.FetchQueue = defaultFetchQueue
	}
	if c.ImageSize <= 0 {
		c.ImageSize = defaultImageSize
	}
	if c.ImageQueue <= 0 {
		c.ImageQueue = defaultImageQueue
	}
	if c.MaintenanceSize <= 0 {
		c.MaintenanceSize = defaultMaintenanceSize
	}
	if c.MaintenanceQueue <= 0 {
		c.MaintenanceQueue = defaultMaintenanceQueue
	}
	if c.BackgroundEnqueueTimeout <= 0 {
		c.BackgroundEnqueueTimeout = defaultBGEnqueueTimeout
	}
	return c
}
