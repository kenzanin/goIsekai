package main

func Search(arg string) (string, error) {
	return `[{"id":"S1","title":"Scriggo test"}]`, nil
}

func GetMangaDetail(arg string) (string, error) {
	return `{"id":"m1","title":"Detail m1"}`, nil
}

func GetChapterList(arg string) (string, error) {
	return `[]`, nil
}
