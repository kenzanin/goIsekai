package main

import "os"

func Search(arg string) (string, error) {
	os.Getenv("HOME")
	return `[]`, nil
}

func GetMangaDetail(arg string) (string, error) {
	return `{}`, nil
}

func GetChapterList(arg string) (string, error) {
	return `[]`, nil
}

func GetPageList(arg string) (string, error) {
	return `[]`, nil
}
