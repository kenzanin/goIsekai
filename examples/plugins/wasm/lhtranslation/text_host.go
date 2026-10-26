//go:build wasip1

package main

import "encoding/json"

// chapterNum delegates to the host's text.chapter_num helper.
func chapterNum(s string) float64 {
	var f float64
	if json.Unmarshal([]byte(hostCall("text.chapter_num", s)), &f) != nil {
		return 0
	}
	return f
}

// dateToISO delegates to the host's text.date_to_iso helper.
func dateToISO(s string) string {
	return hostString("text.date_to_iso", s)
}
