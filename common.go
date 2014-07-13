package main

import (
	"crypto/sha1"
	"fmt"
	"html/template"
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

func writeTemplate(w io.Writer, c Config, t string, d interface{}, fm template.FuncMap) error {
	tmpl, err := template.New("").Funcs(fm).Parse(t)
	if err != nil {
		c.Logger.Printf("Unable to parse template: %v\n", err)
		return err
	}
	if err = tmpl.Execute(w, d); err != nil {
		c.Logger.Printf("Unable to execute template: %v\n", err)
		return err
	}
	return nil
}

// Writes everything up to the closing </head> tag.
func writePageHeader(w io.Writer, c Config, title, faviconPath string) {
	d := struct {
		Title          string
		StylesheetPath string
		FaviconPath    string
	}{
		Title:          title,
		StylesheetPath: c.GetPath(staticUrlPath, cssFile),
		FaviconPath:    c.GetPath(staticUrlPath, faviconFile),
	}

	if len(faviconPath) > 0 {
		d.FaviconPath = faviconPath
	}

	t := `
<!DOCTYPE html>
<html>
  <head>
    <meta content="text/html; charset=utf-8" http-equiv="Content-Type"/>
    <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
    <title>{{.Title}}</title>
    <link rel="stylesheet" href="{{.StylesheetPath}}"/>
	<link rel="icon" href="{{.FaviconPath}}"/>
  </head>
`
	if err := writeTemplate(w, c, t, d, template.FuncMap{}); err != nil {
		c.Logger.Println(err)
		panic(err)
	}
}
