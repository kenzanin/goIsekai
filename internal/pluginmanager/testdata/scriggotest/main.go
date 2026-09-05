package main

func Search(arg string) (string, error) {
	return `[{"id":"S1","title":"Scriggo test"}]`, nil
}

func GetMangaDetail(arg string) (string, error) {
	return `{"id":"m1","title":"Detail m1"}`, nil
}

func GetChapterList(arg string) (string, error) {
	return `[{"id":"c1","manga_id":"m1","title":"Chapter 1","chapter_num":1.0}]`, nil
}

func GetPageList(arg string) (string, error) {
	return `[{"index":0,"url":"https://example.com/img/1.png"}]`, nil
}
