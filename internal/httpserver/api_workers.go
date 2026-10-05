package httpserver

import "net/http"

// apiWorkerLane is the JSON shape for one lane in GET /api/workers.
type apiWorkerLane struct {
	Lane      string `json:"lane"`
	Waiting   int    `json:"waiting"`
	Running   int    `json:"running"`
	Capacity  int    `json:"capacity"`
	Completed int64  `json:"completed"`
	Failed    int64  `json:"failed"`
	Abandoned int64  `json:"abandoned"`

	Samples      int   `json:"duration_samples"`
	MedianMS     int64 `json:"median_ms"`
	P99MS        int64 `json:"p99_ms"`
	HasDurations bool  `json:"has_durations"`
}

// apiWorkers reports worker-lane state for GET /api/workers. Read-only and
// in-process: no external collector, per the worker-observability spec.
func (s *Server) apiWorkers(w http.ResponseWriter, r *http.Request) {
	states := s.service.GetPool().LaneStates()
	out := make([]apiWorkerLane, 0, len(states))
	for _, st := range states {
		out = append(out, apiWorkerLane{
			Lane:         st.Lane.String(),
			Waiting:      st.Waiting,
			Running:      st.Running,
			Capacity:     st.Capacity,
			Completed:    st.Completed,
			Failed:       st.Failed,
			Abandoned:    st.Abandoned,
			Samples:      st.Durations.Samples,
			MedianMS:     st.Durations.Median.Milliseconds(),
			P99MS:        st.Durations.P99.Milliseconds(),
			HasDurations: st.Durations.HasData,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"lanes": out})
}
