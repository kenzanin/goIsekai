package workers

import (
	"context"
	"fmt"
	"time"
)

// jobInfoKey is the context key for the running *Job (SA1029: no bare string keys).
type jobInfoKey struct{}

// wrappedJob wraps job execution with lane-qualified logging.
// All worker logs now carry [lane] prefix and include start, stop, dur_ms.
func wrappedJob(laneName string, jobID string, run func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		start := time.Now()
		// Extract job metadata from context for detailed logging
		var details string
		if jctx, ok := ctx.Value(jobInfoKey{}).(*Job); ok && jctx != nil {
			details = fmt.Sprintf(" plugin=%s", jctx.PluginKey)
		}
		// start log
		fmt.Printf("%s [%s] jobID=%s%s running\n", start.Format("15:04:05"), laneName, jobID, details)
		defer func() {
			dur := time.Since(start)
			fmt.Printf("%s [%s] jobID=%s done dur_ms=%d\n", time.Now().Format("15:04:05"), laneName, jobID, dur.Milliseconds())
		}()
		return run(ctx)
	}
}
