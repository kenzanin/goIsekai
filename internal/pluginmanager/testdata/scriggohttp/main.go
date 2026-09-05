package main

import "hostnet"

func Search(arg string) (string, error) {
	body, err := hostnet.Get(arg)
	if err != nil {
		return "", err
	}
	return `"` + body + `"`, nil
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
