package main

import (
	"crypto/sha1"
	"fmt"
	"html/template"
	"io"
	"net/url"
	"os"
)

const (
	sessionCookieName = "session"
	addUrlPath        = "add"
	archiveUrlPath    = "archive"
	authUrlPath       = "auth"
	kindleUrlPath     = "kindle"
	staticUrlPath     = "static"
	pagesUrlPath      = "pages"
	appCssFile        = "app.css"
	commonCssFile     = "common.css"
	pageCssFile       = "page.css"
	faviconFile       = "favicon.ico"

	// Query parameter names for HTTP requests.
	idParam       = "i"
	tokenParam    = "t"
	redirectParam = "r"
)

type PageInfo struct {
	Id          string
	OriginalUrl string
	Title       string
	TimeAdded   int64 // time_t
	Token       string
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

func copyFile(dest, src string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	d, err := os.Create(dest)
	if err != nil {
		return err
	}

	if _, err := io.Copy(d, s); err != nil {
		d.Close()
		return err
	}
	return d.Close()
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
func writeHeader(w io.Writer, c Config, stylesheets []string, title, favicon, author string) {
	d := struct {
		Title       string
		Stylesheets []string
		Favicon     string
		Author      string
	}{
		Title:       title,
		Stylesheets: stylesheets,
		Favicon:     c.GetPath(staticUrlPath, faviconFile),
		Author:      author,
	}

	if len(favicon) > 0 {
		d.Favicon = favicon
	}

	t := `
<!DOCTYPE html>
<html>
  <head>
    <meta content="text/html; charset=utf-8" http-equiv="Content-Type"/>
    <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
    {{if .Author}}<meta content="{{.Author}}" name="author"/>{{end}}
    <title>{{.Title}}</title>
    {{range .Stylesheets}}<link rel="stylesheet" href="{{.}}"/>{{end}}
    <link rel="icon" href="{{.Favicon}}"/>
  </head>
`
	if err := writeTemplate(w, c, t, d, template.FuncMap{}); err != nil {
		c.Logger.Println(err)
		panic(err)
	}
}
