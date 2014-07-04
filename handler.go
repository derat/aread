package main

import (
	"html/template"
	"log"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

type Handler struct {
	Password         string
	BookmarkletToken string
	MaxListSize      int

	processor     *Processor
	db            *Database
	logger        *log.Logger
	baseUrl       *url.URL
	staticHandler http.Handler
	pageHandler   http.Handler
}

func NewHandler(p *Processor, d *Database, l *log.Logger, baseUrl *url.URL, staticDir, pageDir string) *Handler {
	return &Handler{
		MaxListSize:   50,
		processor:     p,
		db:            d,
		logger:        l,
		baseUrl:       baseUrl,
		staticHandler: http.StripPrefix(baseUrl.Path+"/"+staticUrlPath, http.FileServer(http.Dir(staticDir))),
		pageHandler:   http.StripPrefix(baseUrl.Path+"/"+pagesUrlPath, http.FileServer(http.Dir(pageDir))),
	}
}

func (h Handler) handleAdd(w http.ResponseWriter, r *http.Request) {
	if len(h.BookmarkletToken) > 0 && r.FormValue("t") != h.BookmarkletToken {
		h.logger.Printf("Tokenless request from %v\n", r.RemoteAddr)
		http.Error(w, "Invalid token", http.StatusForbidden)
		return
	}

	u := r.FormValue("u")
	if len(u) == 0 {
		http.Error(w, "Missing URL", http.StatusBadRequest)
		return
	}

	sendToKindle := r.FormValue("k") == "1"
	pi, err := h.processor.ProcessUrl(u, sendToKindle)
	if len(pi.Id) > 0 {
		h.db.AddPage(pi)
	}
	if err != nil {
		h.logger.Println(err)
		http.Error(w, "Failed to add page", http.StatusInternalServerError)
		return
	}
	pagePath := path.Join(h.baseUrl.Path, pagesUrlPath, pi.Id)
	http.Redirect(w, r, pagePath, http.StatusFound)
}

func (h Handler) makeBookmarklet(kindle bool) string {
	getCurUrl := "encodeURIComponent(document.URL)"
	addUrl := path.Join(h.baseUrl.String(), addUrlPath) + "?u=\"+" + getCurUrl + "+\"&t=" + h.BookmarkletToken
	if kindle {
		addUrl += "&k=1"
	}
	return "javascript:{window.location.href=\"" + addUrl + "\";};void(0);"
}

func (h Handler) handleList(w http.ResponseWriter, r *http.Request) {
	type templateData struct {
		Pages                 []PageInfo
		PagesPath             string
		StylesheetPath        string
		ReadBookmarkletHref   template.HTMLAttr
		KindleBookmarkletHref template.HTMLAttr
	}
	d := &templateData{
		PagesPath:             path.Join(h.baseUrl.Path, pagesUrlPath),
		StylesheetPath:        path.Join(h.baseUrl.Path, staticUrlPath, cssFile),
		ReadBookmarkletHref:   template.HTMLAttr("href=" + h.makeBookmarklet(false)),
		KindleBookmarkletHref: template.HTMLAttr("href=" + h.makeBookmarklet(true)),
	}

	var err error
	if d.Pages, err = h.db.GetPages(h.MaxListSize); err != nil {
		h.logger.Printf("Unable to get pages: %v\n", err)
		http.Error(w, "Unable to get page list", http.StatusInternalServerError)
		return
	}

	fm := template.FuncMap{
		"host": getHost,
		"time": func(t int64) string { return time.Unix(t, 0).Format("Monday, January 2 at 15:04:05") },
	}
	tmpl, err := template.New("list").Funcs(fm).Parse(`
<!DOCTYPE html>
<html>
  <head>
    <meta content="text/html; charset=utf-8" http-equiv="Content-Type"/>
    <title>Reading List</title>
    <link href="{{.StylesheetPath}}" rel="stylesheet" type="text/css"/>
  </head>
  <body>
    {{ range .Pages }}
    <div class="list-entry">
      <div class="title"><a href="{{$.PagesPath}}/{{.Id}}/">{{.Title}}</a></div>
      <div class="orig"><a href="{{.OriginalUrl}}">{{host .OriginalUrl}}</a></div>
      <div class="time">Added {{time .TimeAdded}}</div>
    </div>
    {{ end }}
    <div>
      <span class="bookmarklet"><a {{.ReadBookmarkletHref}}>Add to list</a></span>
      <span class="bookmarklet"><a {{.KindleBookmarkletHref}}>Send to Kindle</a></span>
    </div>
  </body>
</html>`)
	if err != nil {
		h.logger.Printf("Unable to parse template: %v\n", err)
		http.Error(w, "Unable to parse template", http.StatusInternalServerError)
		return
	}
	if err = tmpl.Execute(w, d); err != nil {
		h.logger.Printf("Unable to execute template: %v\n", err)
		http.Error(w, "Unable to execute template", http.StatusInternalServerError)
		return
	}
}

func (h Handler) handleAuth(w http.ResponseWriter, r *http.Request) {
	if len(r.FormValue("p")) > 0 {
		if len(h.Password) > 0 && r.FormValue("p") == h.Password {
			id := getSha1String(h.Password + "|" + strconv.FormatInt(time.Now().UnixNano(), 10))
			if err := h.db.AddSession(id); err != nil {
				h.logger.Printf("Unable to insert session: %v\n", err)
				http.Error(w, "Unable to insert session", http.StatusInternalServerError)
				return
			}
			h.logger.Printf("Successful authentication attempt from %v\n", r.RemoteAddr)
			w.Header()["Set-Cookie"] = []string{sessionCookie + "=" + id}
			http.Redirect(w, r, r.FormValue("r"), http.StatusFound)
			return
		} else {
			h.logger.Printf("Bad authentication attempt from %v\n", r.RemoteAddr)
		}
	}

	type templateData struct {
		Redirect       string
		StylesheetPath string
	}
	d := templateData{
		Redirect:       r.FormValue("r"),
		StylesheetPath: path.Join(h.baseUrl.Path, staticUrlPath, cssFile),
	}
	tmpl, err := template.New("auth").Parse(`
<!DOCTYPE html>
<html>
  <head>
    <meta content="text/html; charset=utf-8" http-equiv="Content-Type"/>
    <title>Auth</title>
    <link href="{{.StylesheetPath}}" rel="stylesheet" type="text/css"/>
  </head>
  <body>
    <form method="post">
      Password: <input type="password" name="p"><br>
	  <input type="hidden" name="r" value={{.Redirect}}>
      <input type="submit" value="Submit">
    </form>
  </body>
</html>`)
	if err != nil {
		h.logger.Printf("Unable to parse template: %v\n", err)
		http.Error(w, "Unable to parse template", http.StatusInternalServerError)
		return
	}
	if err = tmpl.Execute(w, d); err != nil {
		h.logger.Printf("Unable to execute template: %v\n", err)
		http.Error(w, "Unable to execute template", http.StatusInternalServerError)
		return
	}
}

func (h Handler) isAuthenticated(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	isAuth, err := h.db.IsValidSession(c.Value)
	if err != nil {
		h.logger.Println(err)
		return false
	}
	return isAuth
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, h.baseUrl.Path) {
		h.logger.Printf("Got request with unexpected path \"%v\"", r.URL.Path)
		http.Error(w, "Unexpected path", http.StatusInternalServerError)
		return
	}
	reqPath := r.URL.Path[len(h.baseUrl.Path):]
	if strings.HasPrefix(reqPath, "/") {
		reqPath = reqPath[1:]
	}

	if strings.HasPrefix(reqPath, staticUrlPath+"/") {
		h.staticHandler.ServeHTTP(w, r)
		return
	}
	if reqPath == authUrlPath {
		h.handleAuth(w, r)
		return
	}

	// Everything else requires authentication.
	if !h.isAuthenticated(r) {
		h.logger.Printf("Unauthenticated request from %v\n", r.RemoteAddr)
		http.Redirect(w, r, path.Join(h.baseUrl.Path, authUrlPath+"?r="+r.URL.Path), http.StatusFound)
		return
	}

	if len(reqPath) == 0 {
		h.handleList(w, r)
	} else if reqPath == addUrlPath {
		h.handleAdd(w, r)
	} else if strings.HasPrefix(reqPath, pagesUrlPath+"/") {
		h.pageHandler.ServeHTTP(w, r)
	} else {
		http.Error(w, "Bogus request", http.StatusBadRequest)
	}
}
