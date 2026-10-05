package httpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func frontendFile(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "cmd", "goisekai", "frontend", "lib", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(body)
}

// Every fetch that carries the CSRF token must handle the reload hint, or the
// refusal reaches the user as a dead error. There is no JS test runner in this
// repo, so this pins the shape instead.
func TestEveryTokenBearingFetchHandlesTheReloadHint(t *testing.T) {
	for _, name := range []string{"alpine-components.js", "reader.js"} {
		src := frontendFile(t, name)
		bearing := strings.Count(src, "'X-CSRF-Token':") + strings.Count(src, "X-CSRF-Token':")
		if bearing == 0 {
			continue
		}
		if !strings.Contains(src, "X-GoIsekai-Reload") {
			t.Errorf("%s sends the CSRF token on %d request(s) but never reads %s; "+
				"a stale-token refusal would reach the user as a dead error",
				name, bearing, "X-GoIsekai-Reload")
		}
		if !strings.Contains(src, "location.reload()") {
			t.Errorf("%s reads the reload hint but never reloads", name)
		}
	}
}

// Without a one-shot guard the reload can loop forever: a cached document that
// keeps serving a stale token would put the page in a refresh cycle.
func TestReloadIsGuardedAgainstLooping(t *testing.T) {
	for _, name := range []string{"alpine-components.js", "reader.js"} {
		src := frontendFile(t, name)
		if !strings.Contains(src, "gi_csrf_reloaded") {
			t.Errorf("%s has no sessionStorage guard; a stale cached page could reload in a loop", name)
			continue
		}
		if !strings.Contains(src, "sessionStorage.setItem") {
			t.Errorf("%s sets no guard flag before reloading", name)
		}
		if !strings.Contains(src, "sessionStorage.getItem") {
			t.Errorf("%s never reads the guard flag back, so the second refusal cannot fall through", name)
		}
	}
}

// The guard must be cleared once a page loads with a live token, or the second
// genuine failure in a session would be swallowed instead of reported.
func TestReloadGuardIsClearedOnAFreshPage(t *testing.T) {
	src := frontendFile(t, "alpine-components.js")
	if !strings.Contains(src, "sessionStorage.removeItem") {
		t.Error("nothing clears gi_csrf_reloaded; after one recovered restart every later " +
			"CSRF error would be silently dropped instead of shown")
	}
}

// reader.js posts progress as fire-and-forget, so a swallowed rejection there
// means the chapter silently never records as read.
func TestReaderProgressPostObservesItsResponse(t *testing.T) {
	src := frontendFile(t, "reader.js")
	i := strings.Index(src, "'/action/set-chapter-progress'")
	if i < 0 {
		t.Fatal("progress post not found in reader.js")
	}
	window := src[i : i+900]
	if strings.Contains(window, ".then(") && !strings.Contains(window, "X-GoIsekai-Reload") {
		t.Error("reportProgress reads its response but ignores the reload hint")
	}
}
