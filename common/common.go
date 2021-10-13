// Copyright 2020 Daniel Erat.
// All rights reserved.

// Package common contains constants and functions shared by other packages.
package common

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
	faviconFile     = "favicon.ico"
	defaultImageExt = ".jpg"
)

var supportedImageExts = map[string]struct{}{
	".bmp":  struct{}{},
	".gif":  struct{}{},
	".jpeg": struct{}{},
	".jpg":  struct{}{},
	".png":  struct{}{},
	".svg":  struct{}{},
}

type PageInfo struct {
	Id          string
	OriginalURL string
	Title       string
	TimeAdded   int64 // time_t
	Token       string
	FromFriend  bool
}

func GetHost(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	return u.Host
}

func SHA1String(input string) string {
	h := sha1.New()
	io.WriteString(h, input)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func WriteTemplate(w io.Writer, cfg *Config, t string, d interface{}, fm template.FuncMap) error {
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

// WriteHeader writes everything up to the closing </head> tag.
func WriteHeader(w io.Writer, cfg *Config, stylesheets []string, title, favicon, author string) {
	d := struct {
		Title       string
		Stylesheets []string
		Favicon     string
		Author      string
	}{
		Title:       title,
		Stylesheets: stylesheets,
		Favicon:     cfg.GetPath(StaticURLPath, faviconFile),
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
    <title>{{.Title}}</title>
    <meta name="DCTERMS.title" content="{{.Title}}"/>
    {{if .Author -}}
    <meta name="author" content="{{.Author}}"/>
    <meta name="DCTERMS.creator" content="{{.Author}}"/>
    {{- end}}
    {{range .Stylesheets}}<link rel="stylesheet" href="{{.}}"/>{{end}}
    <link rel="icon" href="{{.Favicon}}"/>
  </head>
`
	if err := WriteTemplate(w, cfg, t, d, template.FuncMap{}); err != nil {
		cfg.Logger.Println(err)
		panic(err)
	}
}

func LocalImageFilename(url string) string {
	// kindlegen seems to be confused by image files without extensions.
	ext := strings.ToLower(filepath.Ext(strings.Split(url, "?")[0]))
	if _, ok := supportedImageExts[ext]; !ok {
		ext = defaultImageExt
	}
	return SHA1String(url) + ext
}

func ReadJSONFile(path string, out interface{}) error {
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
