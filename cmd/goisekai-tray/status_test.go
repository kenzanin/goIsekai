package main

import "testing"

func TestHealthyRejectsDeadServer(t *testing.T) {
	// Port 1 is not listening in any test environment.
	if healthy("http://127.0.0.1:1/") {
		t.Fatal("healthy() reported up for a dead port")
	}
}
