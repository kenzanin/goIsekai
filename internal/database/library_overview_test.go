package database

import (
	"strings"
	"testing"
)

// testStatusAlias mirrors the shipped [status] defaults. It is spelled out here
// rather than imported from config so the database package keeps its
// no-config-dependency layering, tests included.
func testStatusAlias() StatusAlias {
	return StatusAlias{
		"Ongoing":   {"ongoing", "publishing", "releasing", "on going"},
		"Completed": {"completed", "complete", "finished", "ended"},
		"Hiatus":    {"hiatus", "uncertain", "on hold", "paused"},
		"Cancelled": {"cancelled", "canceled", "dropped", "axed"},
		"Unknown":   {},
	}
}

// TestClassifyStatusBuckets pins the dashboard status buckets. The bug this
// guards against: classifyStatus returns the configured canonical name
// ("Completed"), while the aggregation switch compared lowercase literals
// ("done"), so every manga silently counted as unknown.
func TestClassifyStatusBuckets(t *testing.T) {
	aliases := testStatusAlias()
	buckets := map[string]string{
		"Completed":  "Completed",
		"completed":  "Completed",
		"finished":   "Completed",
		"Ongoing":    "Ongoing",
		"publishing": "Ongoing",
		"Hiatus":     "Hiatus",
		"on hold":    "Hiatus",
		"Cancelled":  "Cancelled",
		"":           "unknown",
		"gibberish":  "unknown",
	}
	for raw, want := range buckets {
		if got := classifyStatus(raw, aliases); got != want {
			t.Errorf("classifyStatus(%q) = %q, want %q", raw, got, want)
		}
	}
}

// TestStatusBucketingCountsEveryManga pins the dashboard aggregation: every
// recognised spelling lands in the same bucket the pre-remap code gave its
// base word, so "completed"/"finished" both count as done and "hiatus"
// counts as ongoing, as before. Cancelled has no dashboard bucket (done or
// ongoing), so it stays unknown, same as pre-remap.
func TestStatusBucketingCountsEveryManga(t *testing.T) {
	aliases := testStatusAlias()
	counts := map[string]int{}
	for _, status := range []string{
		"Ongoing", "ongoing", "Publishing", "Completed", "finished",
		"Hiatus", "on hold", "Cancelled", "canceled",
	} {
		switch bucket := classifyStatus(status, aliases); {
		case strings.EqualFold(bucket, "Completed"):
			counts["done"]++
		case strings.EqualFold(bucket, "Ongoing"), strings.EqualFold(bucket, "Hiatus"):
			counts["ongoing"]++
		default:
			counts["unknown"]++
		}
	}
	if counts["done"] != 2 {
		t.Errorf("done = %d, want 2 (Completed, finished)", counts["done"])
	}
	if counts["ongoing"] != 5 {
		t.Errorf("ongoing = %d, want 5 (hiatus counts as ongoing, as before)", counts["ongoing"])
	}
	if counts["unknown"] != 2 {
		t.Errorf("unknown = %d, want 2 (cancelled has no dashboard bucket)", counts["unknown"])
	}
}

// TestClassifyStatusHonorsConfig pins that the buckets follow the INI, not a
// built-in table: renaming a canonical leaves the old spelling unmatched.
func TestClassifyStatusHonorsConfig(t *testing.T) {
	aliases := StatusAlias{"Finished": {"completed", "done"}, "Running": {"ongoing"}}
	if got := classifyStatus("completed", aliases); got != "Finished" {
		t.Errorf("classifyStatus(completed) = %q, want Finished", got)
	}
	if got := classifyStatus("Ongoing", aliases); got != "Running" {
		t.Errorf("classifyStatus(Ongoing) = %q, want Running", got)
	}
	// A canonical the config did not define is no longer recognised.
	if got := classifyStatus("Hiatus", aliases); got != "unknown" {
		t.Errorf("classifyStatus(Hiatus) = %q, want unknown", got)
	}
}
