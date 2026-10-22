package main

import (
	"net/http"
	"time"
)

// healthy reports whether the server answers its library page. Any answer
// counts as up: a non-200 still means the process is listening, and that is
// what the menu entry is for.
func healthy(url string) bool {
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get(url)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return true
}
