package main

import (
	"net/url"
)

const (
	staticUrlPath = "static"
	pagesUrlPath  = "pages"
	cssFile       = "base.css"
)

type PageInfo struct {
	Id          string
	OriginalUrl string
	Title       string
	TimeAdded   int64 // time_t
}

func getHost(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	return u.Host
}
