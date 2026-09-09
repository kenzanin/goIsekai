//go:build wasip1

package main

import (
	"encoding/json"
	"goisekai/pkg/types"
	"math"
	"net/url"
	"strconv"

	"github.com/extism/go-pdk"
)

// GetChapterList returns all chapters for a manga (paginated feed).
//
//go:wasmexport GetChapterList
func GetChapterList() int32 {
	var mangaID string
	_ = json.Unmarshal(pdk.Input(), &mangaID)
	if mangaID == "" {
		b, _ := json.Marshal([]types.Chapter{})
		pdk.Output(b)
		return 0
	}

	var all []types.Chapter
	offset := 0
	for {
		q := url.Values{}
		q.Set("limit", "500")
		q.Set("offset", strconv.Itoa(offset))
		q.Add("translatedLanguage[]", lang)
		q.Add("order[volume]", "asc")
		q.Add("order[chapter]", "asc")
		q.Add("includes[]", "scanlation_group")
		q.Set("includeEmptyPages", "0")
		contentRatingQuery(q)

		resp, err := doFetch(apiURL + "/manga/" + mangaID + "/feed?" + q.Encode())
		if err != nil || resp.Status < 200 || resp.Status >= 300 {
			break
		}

		var list chapterListResp
		if err := json.Unmarshal([]byte(resp.Body), &list); err != nil {
			break
		}

		for _, cd := range list.Data {
			if cd.Attributes.ExternalURL != "" {
				continue
			}

			chNum := parseFloat64(cd.Attributes.Chapter)
			if math.IsNaN(chNum) {
				chNum = 0
			}

			volNum := parseFloat64(cd.Attributes.Volume)
			if math.IsNaN(volNum) {
				volNum = 0
			}

			chTitle := "Chapter " + cd.Attributes.Chapter
			if cd.Attributes.Title != "" {
				chTitle = cd.Attributes.Title
			}
			if cd.Attributes.Chapter == "" && cd.Attributes.Title == "" {
				chTitle = "Oneshot"
			}

			all = append(all, types.Chapter{
				ID:         cd.ID,
				MangaID:    mangaID,
				Title:      chTitle,
				ChapterNum: chNum,
				VolumeNum:  volNum,
				ReleasedAt: parseTime(cd.Attributes.PublishAt),
				URL:        "https://mangadex.org/chapter/" + cd.ID,
			})
		}

		offset += list.Limit
		if offset >= list.Total {
			break
		}
	}
	b, _ := json.Marshal(all)
	pdk.Output(b)
	return 0
}
