package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	sessionCookieName = "session"
	addURLPath        = "add"
	archiveURLPath    = "archive"
	authURLPath       = "auth"
	kindleURLPath     = "kindle"
	staticURLPath     = "static"
	pagesURLPath      = "pages"
	appCSSFile        = "app.css"
	commonCSSFile     = "common.css"
	pageCSSFile       = "page.css"
	faviconFile       = "favicon.ico"

	// Query parameter names for HTTP requests.
	idParam       = "i"
	tokenParam    = "t"
	redirectParam = "r"

	defaultImageExtension = ".jpg"
)

var supportedImageExtensions map[string]bool = map[string]bool{
	".bmp":  true,
	".gif":  true,
	".jpeg": true,
	".jpg":  true,
	".png":  true,
	".svg":  true,
}

type PageInfo struct {
	Id          string
	OriginalURL string
	Title       string
	TimeAdded   int64 // time_t
	Token       string
	FromFriend  bool
}

func getHost(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	return u.Host
}

func getSHA1String(input string) string {
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

func writeTemplate(w io.Writer, cfg config, t string, d interface{}, fm template.FuncMap) error {
	tmpl, err := template.New("").Funcs(fm).Parse(t)
	if err != nil {
		cfg.Logger.Printf("Unable to parse template: %v\n", err)
		return err
	}
	if err = tmpl.Execute(w, d); err != nil {
		cfg.Logger.Printf("Unable to execute template: %v\n", err)
		return err
	}
	return nil
}

// Writes everything up to the closing </head> tag.
func writeHeader(w io.Writer, cfg config, stylesheets []string, title, favicon, author string) {
	d := struct {
		Title       string
		Stylesheets []string
		Favicon     string
		Author      string
	}{
		Title:       title,
		Stylesheets: stylesheets,
		Favicon:     cfg.GetPath(staticURLPath, faviconFile),
		Author:      author,
	}

	if len(favicon) > 0 {
		d.Favicon = favicon
	}

	t := `<!DOCTYPE html>
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
	if err := writeTemplate(w, cfg, t, d, template.FuncMap{}); err != nil {
		cfg.Logger.Println(err)
		panic(err)
	}
}

func getLocalImageFilename(url string) string {
	// kindlegen seems to be confused by image files without extensions.
	ext := strings.ToLower(filepath.Ext(strings.Split(url, "?")[0]))
	if _, ok := supportedImageExtensions[ext]; !ok {
		ext = defaultImageExtension
	}
	return getSHA1String(url) + ext
}

func readJSONFile(path string, out interface{}) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	d := json.NewDecoder(f)
	if err = d.Decode(&out); err != nil {
		return err
	}
	return nil
}

func joinURLAndPath(url, path string) string {
	// Can't use path.Join, as it changes e.g. "https://" to "https:/".
	if strings.HasSuffix(url, "/") {
		url = url[0 : len(url)-1]
	}
	if strings.HasPrefix(path, "/") {
		path = path[1:len(path)]
	}
	return url + "/" + path
}
