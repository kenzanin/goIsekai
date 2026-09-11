package main

import "hostnet"

func Search(arg string) (string, error) {
	url := "http://localhost:" + arg
	body, err := hostnet.Get(url)
	if err != nil {
		return "", err
	}
	return body, nil
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
