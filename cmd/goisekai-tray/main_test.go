package main

import "testing"

func TestResolveURL(t *testing.T) {
	for _, tc := range []struct {
		host string
		port int
		want string
	}{
		{"127.0.0.1", 8080, "http://127.0.0.1:8080/"},
		{"", 0, "http://127.0.0.1:8080/"},           // defaults
		{"0.0.0.0", 9000, "http://127.0.0.1:9000/"}, // bind-all loops back
		{"::", 8080, "http://127.0.0.1:8080/"},
		{"::1", 8080, "http://[::1]:8080/"}, // real IPv6 keeps brackets
	} {
		if got := resolveURL(tc.host, tc.port); got != tc.want {
			t.Errorf("resolveURL(%q, %d) = %q, want %q", tc.host, tc.port, got, tc.want)
		}
	}
}

func TestResolveDir(t *testing.T) {
	if got := resolveDir("app_data", "/srv/goisekai"); got != "/srv/goisekai/app_data" {
		t.Errorf("relative = %q", got)
	}
	if got := resolveDir("/var/log/x.log", "/srv/goisekai"); got != "/var/log/x.log" {
		t.Errorf("absolute = %q", got)
	}
}
