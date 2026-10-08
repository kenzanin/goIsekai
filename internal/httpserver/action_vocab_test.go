package httpserver

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The bulk-action dropdown and the handler's switch are two halves of one
// contract: every option value must be a case the handler accepts. The two
// drifted apart before — the dropdown offered actions the switch did not know,
// and one flag was reachable under two names (skip, and show/hide).
func TestDropdownActionsAreAllHandled(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(tmplRoot, "partials", "detail_chapters.lua"))
	if err != nil {
		t.Fatalf("read partial: %v", err)
	}
	opts := regexp.MustCompile(`<option value="([a-z-]+)"`).FindAllStringSubmatch(string(body), -1)
	if len(opts) < 5 {
		t.Fatalf("expected the dropdown options in the partial, found %d", len(opts))
	}

	handler, err := os.ReadFile("actions_progress.go")
	if err != nil {
		t.Fatalf("read handler: %v", err)
	}
	src := string(handler)
	confirm := regexp.MustCompile(`data-confirm-actions="([^"]*)"`).FindStringSubmatch(string(body))
	confirmed := map[string]bool{}
	if len(confirm) == 2 {
		for a := range strings.SplitSeq(confirm[1], ",") {
			confirmed[a] = true
		}
	}

	seen := map[string]bool{}
	for _, o := range opts {
		value := o[1]
		if seen[value] {
			t.Errorf("dropdown offers %q more than once", value)
		}
		seen[value] = true
		if !strings.Contains(src, `"`+value+`"`) {
			t.Errorf("dropdown offers %q but actions_progress.go has no case for it", value)
		}
		// Destructive actions must stay behind the confirm dialog. "to-read"
		// counts: a range action marks every chapter on one side of the ticked
		// row, which is exactly as hard to undo as clearing the read flag. The
		// original rule missed mark-up-to for that reason.
		destructive := strings.Contains(value, "unread") ||
			strings.Contains(value, "clear") ||
			strings.HasSuffix(value, "-to-read")
		if !confirmed[value] && destructive {
			t.Errorf("%q is destructive but missing from data-confirm-actions", value)
		}
	}
}

// One flag must have one name. is_skipped is toggled per row as Skip/Unskip, so
// the bulk action has to use the same word.
func TestSkipActionUsesTheRowButtonWording(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(tmplRoot, "partials", "detail_chapters.lua"))
	if err != nil {
		t.Fatalf("read partial: %v", err)
	}
	src := string(body)
	if !strings.Contains(src, "toggle-skip/") {
		t.Fatal("no per-row skip form found; this test needs updating")
	}
	if !strings.Contains(src, "Skip ticked") || !strings.Contains(src, "Unskip ticked") {
		t.Error("bulk skip options do not use the same Skip/Unskip wording as the row button")
	}
	if strings.Contains(src, "as show") || strings.Contains(src, "as hide") {
		t.Error("dropdown still offers show/hide for is_skipped; the row button calls it Skip")
	}
}
