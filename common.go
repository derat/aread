package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"net/url"
)

const (
	sessionCookieName = "session"
	addUrlPath        = "add"
	archiveUrlPath    = "archive"
	authUrlPath       = "auth"
	kindleUrlPath     = "kindle"
	staticUrlPath     = "static"
	pagesUrlPath      = "pages"
	cssFile           = "base.css"
	faviconFile       = "favicon.ico"
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

func getSha1String(input string) string {
	h := sha1.New()
	io.WriteString(h, input)
	return fmt.Sprintf("%x", h.Sum(nil))
}
