package main

import (
	"strconv"
	"strings"

	"hostnet"
)

const sampleHTML = `<div class="header"><h1 class="title">   Solo Leveling   </h1></div>
<div class="chapters"><a href="/c/1">Chapter 1</a><a href="/c/2">Chapter 2</a></div>
<img src="/img1.jpg"><img alt="x"><img src="/img2.jpg">`

func encodeList(vals []string) string {
	return `["` + strings.Join(vals, `","`) + `"]`
}

// Search runs the same eight lookups the Lua and JS fixtures run, and returns
// them as one JSON document so the three runtimes can be compared field by
// field. The argument "bogus" switches to the invalid-selector path.
func Search(arg string) (string, error) {
	doc, err := hostnet.Parse(sampleHTML)
	if err != nil {
		return "", err
	}

	// The argument arrives JSON-encoded, hence the quoted comparison.
	if arg == `"bogus"` {
		_, err := hostnet.FindText(doc, "{invalid")
		if err == nil {
			return `{"error":""}`, nil
		}
		return `{"error":` + strconv.Quote(err.Error()) + `}`, nil
	}

	title, err := hostnet.FindText(doc, "h1.title")
	if err != nil {
		return "", err
	}
	href, err := hostnet.FindAttr(doc, "a", "href")
	if err != nil {
		return "", err
	}
	texts, err := hostnet.FindListText(doc, "a")
	if err != nil {
		return "", err
	}
	srcs, err := hostnet.FindListAttr(doc, "img", "src")
	if err != nil {
		return "", err
	}
	xtitle, err := hostnet.XPathText(doc, "//h1[@class='title']")
	if err != nil {
		return "", err
	}
	xhref, err := hostnet.XPathAttr(doc, "//a", "href")
	if err != nil {
		return "", err
	}
	xtexts, err := hostnet.XPathListText(doc, "//a")
	if err != nil {
		return "", err
	}
	xsrcs, err := hostnet.XPathListAttr(doc, "//img", "src")
	if err != nil {
		return "", err
	}

	return `{"title":"` + title + `","href":"` + href + `","texts":` + encodeList(texts) +
		`,"srcs":` + encodeList(srcs) + `,"xtitle":"` + xtitle + `","xhref":"` + xhref +
		`","xtexts":` + encodeList(xtexts) + `,"xsrcs":` + encodeList(xsrcs) + `}`, nil
}

func GetMangaDetail(arg string) (string, error) {
	return `{"id":"m1","title":"Detail m1"}`, nil
}

func GetChapterList(arg string) (string, error) {
	return `[]`, nil
}

func GetPageList(arg string) (string, error) {
	return `[]`, nil
}
