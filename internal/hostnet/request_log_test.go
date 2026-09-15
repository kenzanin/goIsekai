package hostnet

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goisekai/internal/logger"
)

// TestHandleRequestLogsFailures pins the contract plugins now rely on: the host,
// not each plugin, reports a failed upstream call. Every plugin used to carry its
// own copy of this logging; if it disappears here, failures go unreported.
func TestHandleRequestLogsFailures(t *testing.T) {
	if err := logger.Init("debug"); err != nil {
		t.Fatalf("logger.Init: %v", err)
	}

	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	}))
	defer srv.Close()

	p := NewProxy()
	call := func() string {
		logger.Clear()
		if _, err := p.HandleRequest("plugin-x", fmt.Sprintf(`{"method":"GET","url":%q}`, srv.URL)); err != nil {
			t.Fatalf("HandleRequest: %v", err)
		}
		return strings.Join(logger.GetLines(), "\n")
	}

	status = http.StatusInternalServerError
	got := call()
	if !strings.Contains(got, "non-2xx") {
		t.Errorf("non-2xx response not logged:\n%s", got)
	}
	if !strings.Contains(got, "plugin=plugin-x") || !strings.Contains(got, "status=500") {
		t.Errorf("log line missing plugin/status context:\n%s", got)
	}

	status = http.StatusOK
	got = call()
	if strings.Contains(got, "non-2xx") {
		t.Errorf("successful response should not log a failure:\n%s", got)
	}
}
