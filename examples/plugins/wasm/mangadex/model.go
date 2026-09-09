//go:build wasip1

package main

type mangaListResp struct {
	Result   string      `json:"result"`
	Response string      `json:"response"`
	Data     []mangaData `json:"data"`
	Limit    int         `json:"limit"`
	Offset   int         `json:"offset"`
	Total    int         `json:"total"`
}

type singleMangaResp struct {
	Result   string    `json:"result"`
	Response string    `json:"response"`
	Data     mangaData `json:"data"`
}

type mangaData struct {
	ID            string         `json:"id"`
	Type          string         `json:"type"`
	Attributes    mangaAttrs     `json:"attributes"`
	Relationships []relationship `json:"relationships"`
}

type mangaAttrs struct {
	Title       map[string]string   `json:"title"`
	AltTitles   []map[string]string `json:"altTitles"`
	Description map[string]string   `json:"description"`
	Status      string              `json:"status"`
	Tags        []mangaTag          `json:"tags"`
}

type mangaTag struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		Name  map[string]string `json:"name"`
		Group string            `json:"group"`
	} `json:"attributes"`
}

type relationship struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes *struct {
		FileName string `json:"fileName"`
		Name     string `json:"name"`
	} `json:"attributes"`
}

type chapterListResp struct {
	Result string        `json:"result"`
	Data   []chapterData `json:"data"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
	Total  int           `json:"total"`
}

type chapterData struct {
	ID            string         `json:"id"`
	Attributes    chapterAttrs   `json:"attributes"`
	Relationships []relationship `json:"relationships"`
}

type chapterAttrs struct {
	Chapter            string `json:"chapter"`
	Volume             string `json:"volume"`
	Title              string `json:"title"`
	TranslatedLanguage string `json:"translatedLanguage"`
	PublishAt          string `json:"publishAt"`
	ExternalURL        string `json:"externalURL"`
}

type atHomeResp struct {
	BaseURL string `json:"baseUrl"`
	Chapter struct {
		Hash string   `json:"hash"`
		Data []string `json:"data"`
	} `json:"chapter"`
}
