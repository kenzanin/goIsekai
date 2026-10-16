package workers

import (
	"context"
	"fmt"
	"time"
)

// wrappedJob wraps job execution with lane-qualified logging.
// All worker logs now carry [lane] prefix and include start, stop, dur_ms.
func wrappedJob(laneName string, jobID string, run func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		start := time.Now()
		// start log
		fmt.Printf("%s [%s] jobID=%s running\n", start.Format("15:04:05"), laneName, jobID)
		defer func() {
			dur := time.Since(start)
			fmt.Printf("%s [%s] jobID=%s done dur_ms=%d\n", time.Now().Format("15:04:05"), laneName, jobID, dur.Milliseconds())
		}()
		return run(ctx)
	}
}
