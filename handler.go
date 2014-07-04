package main

import (
	"html/template"
	"log"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

type Handler struct {
	Password    string
	MaxListSize int

	processor     *Processor
	db            *Database
	logger        *log.Logger
	baseUrlPath   string
	staticHandler http.Handler
	pageHandler   http.Handler
}

func NewHandler(p *Processor, d *Database, l *log.Logger, baseUrlPath, staticDir, pageDir string) *Handler {
	return &Handler{
		MaxListSize:   50,
		processor:     p,
		db:            d,
		logger:        l,
		baseUrlPath:   baseUrlPath,
		staticHandler: http.StripPrefix(baseUrlPath+"/"+staticUrlPath, http.FileServer(http.Dir(staticDir))),
		pageHandler:   http.StripPrefix(baseUrlPath+"/"+pagesUrlPath, http.FileServer(http.Dir(pageDir))),
	}
}

func (h Handler) handleAdd(rw http.ResponseWriter, r *http.Request) {
	url := r.FormValue("u")
	sendToKindle := r.FormValue("k") == "1"
	pi, err := h.processor.ProcessUrl(url, sendToKindle)
	if len(pi.Id) > 0 {
		h.db.AddPage(pi)
	}
	if err != nil {
		h.logger.Println(err)
		rw.Write([]byte("Got an error. :-(")) // FIXME
	} else {
		pagePath := path.Join(h.baseUrlPath, pagesUrlPath, pi.Id)
		http.Redirect(rw, r, pagePath, http.StatusFound)
	}
}

func (h Handler) handleList(rw http.ResponseWriter, r *http.Request) {
	type templateData struct {
		Pages          []PageInfo
		PagesPath      string
		StylesheetPath string
	}
	d := &templateData{
		PagesPath:      path.Join(h.baseUrlPath, pagesUrlPath),
		StylesheetPath: path.Join(h.baseUrlPath, staticUrlPath, cssFile),
	}

	var err error
	if d.Pages, err = h.db.GetPages(h.MaxListSize); err != nil {
		h.logger.Printf("Unable to get pages: %v\n", err)
		return // FIXME
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
  </body>
</html>`)
	if err != nil {
		h.logger.Printf("Unable to parse template: %v\n", err)
		return // FIXME
	}
	if err = tmpl.Execute(rw, d); err != nil {
		h.logger.Printf("Unable to execute template: %v\n", err)
		return // FIXME
	}
}

func (h Handler) handleAuth(rw http.ResponseWriter, r *http.Request) {
	if len(r.FormValue("p")) > 0 {
		if len(h.Password) > 0 && r.FormValue("p") == h.Password {
			id := getSha1String(h.Password + "|" + strconv.FormatInt(time.Now().UnixNano(), 10))
			if err := h.db.AddSession(id); err != nil {
				h.logger.Printf("Unable to insert session: %v\n", err)
				return // FIXME
			}
			h.logger.Printf("Successful authentication attempt from %v\n", r.RemoteAddr)
			rw.Header()["Set-Cookie"] = []string{sessionCookie + "=" + id}
			http.Redirect(rw, r, r.FormValue("r"), http.StatusFound)
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
		StylesheetPath: path.Join(h.baseUrlPath, staticUrlPath, cssFile),
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
		return // FIXME
	}
	if err = tmpl.Execute(rw, d); err != nil {
		h.logger.Printf("Unable to execute template: %v\n", err)
		return // FIXME
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

func (h Handler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, h.baseUrlPath) {
		h.logger.Printf("Got request with unexpected path \"%v\"", r.URL.Path)
		return
	}
	reqPath := r.URL.Path[len(h.baseUrlPath):]
	if strings.HasPrefix(reqPath, "/") {
		reqPath = reqPath[1:]
	}

	if strings.HasPrefix(reqPath, staticUrlPath+"/") {
		h.staticHandler.ServeHTTP(rw, r)
		return
	}
	if reqPath == authUrlPath {
		h.handleAuth(rw, r)
		return
	}

	if !h.isAuthenticated(r) {
		h.logger.Printf("Unauthenticated request from %v\n", r.RemoteAddr)
		http.Redirect(rw, r, path.Join(h.baseUrlPath, authUrlPath+"?r="+r.URL.Path), http.StatusFound)
		return
	}

	if len(reqPath) == 0 {
		if len(r.FormValue("u")) > 0 {
			h.handleAdd(rw, r)
		} else {
			h.handleList(rw, r)
		}
	} else if strings.HasPrefix(reqPath, pagesUrlPath+"/") {
		h.pageHandler.ServeHTTP(rw, r)
	} else {
		// FIXME: 404
	}
}
