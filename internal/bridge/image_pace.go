package bridge

import (
	"strconv"
	"time"

	"goisekai/pkg/types"
)

// respStatus formats the status/err of a retry attempt for logging.
func respStatus(resp types.HTTPResponse, err error) string {
	if err != nil {
		return err.Error()
	}
	return strconv.Itoa(resp.Status)
}

// hostSem returns the per-host image fetch semaphore, creating it on first
// use. Capacity 1 per host keeps MD@Home nodes happy while letting different
// hosts download in parallel.
func (s *AppService) hostSem(host string) chan struct{} {
	s.imgSemMu.Lock()
	defer s.imgSemMu.Unlock()
	if s.imgSem == nil {
		s.imgSem = make(map[string]chan struct{})
	}
	sem, ok := s.imgSem[host]
	if !ok {
		sem = make(chan struct{}, 1)
		s.imgSem[host] = sem
	}
	return sem
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
