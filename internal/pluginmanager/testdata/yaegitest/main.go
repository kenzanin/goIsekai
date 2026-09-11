package main

func Search(arg string) (string, error) {
	return `[{"id":"S1","title":"Yaegi test"}]`, nil
}

func GetMangaDetail(arg string) (string, error) {
	return `{"id":"m1","title":"Detail m1"}`, nil
}

func GetChapterList(arg string) (string, error) {
	return `[{"chapter_num":1,"title":"Ch 1","source_chapter_id":"c1","manga_id":"m1"}]`, nil
}

func GetPageList(arg string) (string, error) {
	return `[{"page_num":1,"url":"https://example.com/img/1.png"}]`, nil
}
