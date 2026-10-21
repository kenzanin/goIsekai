package workers

import (
	"context"
)

// New creates a Pool with the given sizing and starts all workers.
func New(cfg Config) *Pool {
	cfg = cfg.withDefaults()
	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		cfg:              cfg,
		interactive:      &poolLane{name: LaneInteractive, queue: make(chan *jobHandle, cfg.InteractiveQueue)},
		fetch:            &poolLane{name: LaneFetch, queue: make(chan *jobHandle, cfg.FetchQueue)},
		imageHigh:        &poolLane{name: LaneImage, queue: make(chan *jobHandle, cfg.ImageQueue)},
		imageLow:         &poolLane{name: LaneImage, queue: make(chan *jobHandle, cfg.ImageQueue)},
		maintenance:      &poolLane{name: LaneMaintenance, queue: make(chan *jobHandle, cfg.MaintenanceQueue)},
		cdp:              &poolLane{name: LaneCDP, queue: make(chan *jobHandle, cfg.CDPQueue)},
		fetchInUse:       map[string]bool{},
		interactiveInUse: map[string]bool{},
		registry:         map[string]*jobHandle{},
		detail:           map[string]string{},
		dedupe:           map[string]string{},
		rootCtx:          ctx,
		stop:             cancel,
	}
	for i := 0; i < cfg.InteractiveSize; i++ {
		p.spawnInteractive()
	}
	for i := 0; i < cfg.FetchSize; i++ {
		p.spawnFetch()
	}
	// Image workers drain high before low (design 1.1).
	for i := 0; i < cfg.ImageSize; i++ {
		p.spawnImage()
	}
	for i := 0; i < cfg.MaintenanceSize; i++ {
		p.spawnMaintenance()
	}
	for i := 0; i < cfg.CDPSize; i++ {
		p.spawnCDP()
	}
	return p
}

func (p *Pool) spawnInteractive() {
	p.nInteractive.Add(1)
	p.wg.Add(1)
	go p.worker(p.interactive, p.interactiveGate)
}

func (p *Pool) spawnFetch() {
	p.nFetch.Add(1)
	p.wg.Add(1)
	go p.worker(p.fetch, p.fetchGate)
}

func (p *Pool) spawnImage() {
	p.nImage.Add(1)
	p.wg.Add(1)
	go p.imageWorker()
}

func (p *Pool) spawnMaintenance() {
	p.nMaintenance.Add(1)
	p.wg.Add(1)
	go p.worker(p.maintenance, nil)
}

func (p *Pool) spawnCDP() {
	p.nCDP.Add(1)
	p.wg.Add(1)
	go p.worker(p.cdp, nil)
}

// ResizeLane grows one lane's worker count to n (no-op on shrink). Wired to
// the config reload path (task 1.4); grow-only by design — shrinking needs
// drain protocols nothing requires yet.
func (p *Pool) ResizeLane(lane Lane, n int) {
	if p.closed.Load() {
		return
	}
	for {
		var cur int64
		switch lane {
		case LaneInteractive:
			cur = p.nInteractive.Load()
		case LaneFetch:
			cur = p.nFetch.Load()
		case LaneImage:
			cur = p.nImage.Load()
		case LaneMaintenance:
			cur = p.nMaintenance.Load()
		case LaneCDP:
			cur = p.nCDP.Load()
		default:
			return
		}
		if cur >= int64(n) {
			return
		}
		switch lane {
		case LaneInteractive:
			if p.nInteractive.CompareAndSwap(cur, cur+1) {
				p.wg.Add(1)
				go p.worker(p.interactive, p.interactiveGate)
			}
		case LaneFetch:
			if p.nFetch.CompareAndSwap(cur, cur+1) {
				p.wg.Add(1)
				go p.worker(p.fetch, p.fetchGate)
			}
		case LaneImage:
			if p.nImage.CompareAndSwap(cur, cur+1) {
				p.wg.Add(1)
				go p.imageWorker()
			}
		case LaneMaintenance:
			if p.nMaintenance.CompareAndSwap(cur, cur+1) {
				p.wg.Add(1)
				go p.worker(p.maintenance, nil)
			}
		case LaneCDP:
			if p.nCDP.CompareAndSwap(cur, cur+1) {
				p.wg.Add(1)
				go p.worker(p.cdp, nil)
			}
		}
	}
}
