//go:build wasip1

package main

import (
	"encoding/json"
	"goisekai/pkg/types"
	"strconv"
	"time"

	"github.com/extism/go-pdk"
)

//go:wasmexport GetChapterList
func GetChapterList() int32 {
	var hid string
	_ = json.Unmarshal(pdk.Input(), &hid)

	chapters := []types.Chapter{}
	page := 1
	for {
		params := map[string]string{
			"language": "en",
			"limit":    "200",
			"order":    "desc",
			"page":     strconv.Itoa(page),
			"sort":     "number",
		}
		var resp struct {
			Items []struct {
				ID        int     `json:"id"`
				Number    float64 `json:"number"`
				Name      string  `json:"name"`
				CreatedAt int64   `json:"created_at"`
			} `json:"items"`
			Meta struct {
				LastPage int  `json:"last_page"`
				HasNext  bool `json:"has_next"`
			} `json:"meta"`
		}
		u := vrfURL("/titles/"+hid+"/chapters", params)
		if err := fetchJSON(u, &resp); err != nil {
			break
		}
		for _, c := range resp.Items {
			chapters = append(chapters, types.Chapter{
				ID:         strconv.Itoa(c.ID),
				MangaID:    hid,
				ChapterNum: c.Number,
				Title:      c.Name,
				ReleasedAt: time.Unix(c.CreatedAt, 0).UTC(),
			})
		}
		if page >= resp.Meta.LastPage || !resp.Meta.HasNext || page >= 3 {
			break
		}
		page++
	}
	// Descending page order => newest first, matching the host convention.
	b, _ := json.Marshal(chapters)
	pdk.Output(b)
	return 0
}
