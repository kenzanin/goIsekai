package bridge

import (
	"strconv"
	"sync"
	"time"

	"goisekai/pkg/types"
)

// Prio represents the priority of an image fetch request.
type Prio int

const (
	PrioLow Prio = iota
	PrioHigh
)

// hostLanes manages priority-based admission for a single host.
type hostLanes struct {
	mu          sync.Mutex
	highCount   int // current high-priority count
	lowCount    int // current low-priority count
	highWaiters []chan struct{}
	lowWaiters  []chan struct{}
}

// respStatus formats the status/err of a retry attempt for logging.
func respStatus(resp types.HTTPResponse, err error) string {
	if err != nil {
		return err.Error()
	}
	return strconv.Itoa(resp.Status)
}

// hostAcquire acquires admission for the given host and priority.
// Capacity: high=2, low=1. Blocks until slot available.
func (s *AppService) hostAcquire(host string, prio Prio) {
	s.hostLanesMu.Lock()
	if s.hostLanes == nil {
		s.hostLanes = make(map[string]*hostLanes)
	}
	lanes, ok := s.hostLanes[host]
	if !ok {
		lanes = &hostLanes{}
		s.hostLanes[host] = lanes
	}
	s.hostLanesMu.Unlock()

	lanes.mu.Lock()
	for {
		if prio == PrioHigh && lanes.highCount < 2 {
			lanes.highCount++
			lanes.mu.Unlock()
			return
		}
		if prio == PrioLow && lanes.lowCount < 1 {
			lanes.lowCount++
			lanes.mu.Unlock()
			return
		}

		// Lane full, need to queue
		waiter := make(chan struct{})
		if prio == PrioHigh {
			lanes.highWaiters = append(lanes.highWaiters, waiter)
		} else {
			lanes.lowWaiters = append(lanes.lowWaiters, waiter)
		}
		lanes.mu.Unlock()
		<-waiter
		lanes.mu.Lock()
		// Loop to try acquiring again
	}
}

// hostRelease releases a lane slot and wakes next waiter.
func (s *AppService) hostRelease(host string, prio Prio) {
	s.hostLanesMu.Lock()
	lanes, ok := s.hostLanes[host]
	s.hostLanesMu.Unlock()

	if !ok {
		return
	}

	lanes.mu.Lock()
	if prio == PrioHigh {
		lanes.highCount--
		// Wake high waiters first
		if len(lanes.highWaiters) > 0 {
			next := lanes.highWaiters[0]
			lanes.highWaiters = lanes.highWaiters[1:]
			lanes.mu.Unlock()
			close(next)
			return
		}
	} else {
		lanes.lowCount--
		// Wake low waiters
		if len(lanes.lowWaiters) > 0 {
			next := lanes.lowWaiters[0]
			lanes.lowWaiters = lanes.lowWaiters[1:]
			lanes.mu.Unlock()
			close(next)
			return
		}
	}
	lanes.mu.Unlock()
}

// paceImage blocks until at least a second has passed since the previous
// request to the same host — MD@Home nodes 404 rapid bursts (upstream
// convention is ~1 request/second per node).
func (s *AppService) paceImage(host string) {
	const gap = 1100 * time.Millisecond
	s.imgPaceMu.Lock()
	if s.imgPace == nil {
		s.imgPace = make(map[string]time.Time)
	}
	next, ok := s.imgPace[host]
	if !ok || time.Now().After(next) {
		s.imgPace[host] = time.Now().Add(gap)
		s.imgPaceMu.Unlock()
		return
	}
	wait := time.Until(next)
	s.imgPace[host] = next.Add(gap) // reserve a slot for this request
	s.imgPaceMu.Unlock()
	time.Sleep(wait)
}
