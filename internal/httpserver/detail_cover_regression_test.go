package httpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCoverPickerDoesNotReSyncEnrichment pins that clicking the "Covers"
// (fetch-covers) button on the manga detail page does NOT re-sync the
// enrichment panel. Before the fix the detail-page submit handler called
// syncEnrichmentPanel() unconditionally after every form POST, so opening the
// alt-cover picker also reloaded the enrichment side panel — a distracting
// regression. The guard (scoped to /fetch-enrichment|reset-enrichment/ only)
// must remain in BOTH handlers that handle /action/ forms on the detail page:
// the global submitForm handler and the detail-page scoped handler. If either
// reverts to the unconditional call this test fails.
func TestCoverPickerDoesNotReSyncEnrichment(t *testing.T) {
	jsPath := filepath.Join(frontendLibDir, "alpine-components.js")
	src, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("read %s: %v", jsPath, err)
	}
	body := string(src)

	// The regression signature: the unconditional call present in the
	// detail-page handler before the fix.
	unconditional := strings.Count(body, "if (window.syncEnrichmentPanel) syncEnrichmentPanel();")
	if unconditional > 0 {
		t.Errorf("found %d unconditional syncEnrichmentPanel() call(s); the cover-picker regression is back", unconditional)
	}

	// The guarded call must appear in BOTH the global submitForm handler and
	// the detail-page scoped handler.
	guarded := strings.Count(body,
		"if (window.syncEnrichmentPanel && /fetch-enrichment|reset-enrichment/.test(action)) {")
	if guarded != 2 {
		t.Errorf("found %d guarded syncEnrichmentPanel() call(s); want 2 (one in each detail-page /action/ form handler)", guarded)
	}
}
