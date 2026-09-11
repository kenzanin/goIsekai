package main

import "github.com/foo/bar"

func Search(arg string) (string, error) {
	return bar.Echo(arg), nil
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