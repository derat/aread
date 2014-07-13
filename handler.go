package main

import (
	"fmt"
	"html/template"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	addUrlParam    = "u"
	addKindleParam = "k"
)

type Handler struct {
	cfg           Config
	processor     *Processor
	db            *Database
	staticHandler http.Handler
	pageHandler   http.Handler
}

func NewHandler(cfg Config, p *Processor, d *Database) Handler {
	return Handler{
		cfg:           cfg,
		processor:     p,
		db:            d,
		staticHandler: http.StripPrefix(cfg.GetPath(staticUrlPath), http.FileServer(http.Dir(cfg.StaticDir))),
		pageHandler:   http.StripPrefix(cfg.GetPath(pagesUrlPath), http.FileServer(http.Dir(cfg.PageDir))),
	}
}

func (h Handler) getAddToken() string {
	return getSha1String(h.cfg.Username + "|" + h.cfg.Password)
}

func (h Handler) makeBookmarklet(kindle bool) string {
	getCurUrl := "encodeURIComponent(document.URL)"
	addUrl := path.Join(h.cfg.BaseUrl, addUrlPath) + "?" + addUrlParam + "=\"+" + getCurUrl + "+\"&" + tokenParam + "=" + h.getAddToken()
	if kindle {
		addUrl += "&" + addKindleParam + "=1"
	}
	return "javascript:{window.location.href=\"" + addUrl + "\";};void(0);"
}

func (h Handler) serveTemplate(w http.ResponseWriter, t string, d interface{}, fm template.FuncMap) {
	if err := writeTemplate(w, h.cfg, t, d, fm); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h Handler) isAuthenticated(r *http.Request) bool {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}
	isAuth, err := h.db.IsValidSession(c.Value)
	if err != nil {
		h.cfg.Logger.Println(err)
		return false
	}
	return isAuth
}

func (h Handler) handleAdd(w http.ResponseWriter, r *http.Request) {
	u := r.FormValue(addUrlParam)
	if len(u) > 0 {
		if r.FormValue(tokenParam) != h.getAddToken() {
			h.cfg.Logger.Printf("Bad or missing token in add request from %v\n", r.RemoteAddr)
			http.Error(w, "Invalid token", http.StatusForbidden)
			return
		}

		pi, err := h.processor.ProcessUrl(u)
		if err != nil {
			h.cfg.Logger.Println(err)
			http.Error(w, "Failed to process page", http.StatusInternalServerError)
			return
		}
		if err = h.db.AddPage(pi); err != nil {
			h.cfg.Logger.Println(err)
			http.Error(w, "Failed to add to database", http.StatusInternalServerError)
			return
		}
		if r.FormValue(addKindleParam) == "1" {
			if err = h.processor.SendToKindle(pi.Id); err != nil {
				h.cfg.Logger.Println(err)
				http.Error(w, "Failed to send to Kindle", http.StatusInternalServerError)
				return
			}
		}

		http.Redirect(w, r, h.cfg.GetPath(pagesUrlPath, pi.Id), http.StatusFound)
		return
	}

	writeHeader(w, h.cfg, "Add", "", "")
	h.serveTemplate(w, `
  <body>
    <form method="post">
      <table>
        <input type="hidden" name="t" value={{.Token}}>
        <tr><td>URL</td><td><input type="text" name="u" id="add-url"></td></tr>
        <tr><td><input type="submit" value="Add"></td></tr>
	  </table>
    </form>
  </body>
</html>`, struct{ Token string }{Token: h.getAddToken()}, template.FuncMap{})
}

func (h Handler) handleArchive(w http.ResponseWriter, r *http.Request) {
	pi, err := h.db.GetPage(r.FormValue(idParam))
	if err != nil {
		http.Error(w, "Unable to find page", http.StatusBadRequest)
		return
	}
	if len(pi.Token) > 0 && r.FormValue(tokenParam) != pi.Token {
		h.cfg.Logger.Printf("Bad or missing token in archive request from %v\n", r.RemoteAddr)
		http.Error(w, "Invalid token", http.StatusBadRequest)
		return
	}
	if err := h.db.TogglePageArchived(pi.Id); err != nil {
		h.cfg.Logger.Println(err)
		http.Error(w, "Failed to toggle archived state", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, r.FormValue(redirectParam), http.StatusFound)
}

func (h Handler) handleKindle(w http.ResponseWriter, r *http.Request) {
	pi, err := h.db.GetPage(r.FormValue(idParam))
	if err != nil {
		http.Error(w, "Unable to find page", http.StatusBadRequest)
		return
	}
	if len(pi.Token) > 0 && r.FormValue(tokenParam) != pi.Token {
		h.cfg.Logger.Printf("Bad or missing token in kindle request from %v\n", r.RemoteAddr)
		http.Error(w, "Invalid token", http.StatusBadRequest)
		return
	}
	if err := h.processor.SendToKindle(pi.Id); err != nil {
		h.cfg.Logger.Println(err)
		http.Error(w, "Failed to send to Kindle", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, r.FormValue(redirectParam), http.StatusFound)
}

func (h Handler) handleList(w http.ResponseWriter, r *http.Request) {
	d := struct {
		Pages                 []PageInfo
		PagesPath             string
		TogglePagePath        string
		TogglePageString      string
		ToggleListPath        string
		ToggleListString      string
		AddPath               string
		ReadBookmarkletHref   template.HTMLAttr
		KindleBookmarkletHref template.HTMLAttr
	}{
		PagesPath:             h.cfg.GetPath(pagesUrlPath),
		AddPath:               h.cfg.GetPath(addUrlPath),
		ReadBookmarkletHref:   template.HTMLAttr("href=" + h.makeBookmarklet(false)),
		KindleBookmarkletHref: template.HTMLAttr("href=" + h.makeBookmarklet(true)),
	}

	archived := r.FormValue("a") == "1"
	archivedListPath := h.cfg.GetPath() + "?a=1"
	unarchivedListPath := h.cfg.GetPath()
	if archived {
		d.TogglePageString = "Unarchive"
		d.ToggleListString = "View unarchived pages"
		d.ToggleListPath = unarchivedListPath
	} else {
		d.TogglePageString = "Archive"
		d.ToggleListString = "View archived pages"
		d.ToggleListPath = archivedListPath
	}

	var err error
	if d.Pages, err = h.db.GetAllPages(archived, h.cfg.MaxListSize); err != nil {
		h.cfg.Logger.Printf("Unable to get pages: %v\n", err)
		http.Error(w, "Unable to get page list", http.StatusInternalServerError)
		return
	}

	fm := template.FuncMap{
		"host": getHost,
		"time": func(t int64) string { return time.Unix(t, 0).Format("Monday, January 2 at 15:04:05") },
		"toggleUrl": func(id, token string) string {
			listPath := unarchivedListPath
			if archived {
				listPath = archivedListPath
			}
			return fmt.Sprintf("%s?%s=%s&%s=%s&%s=%s", h.cfg.GetPath(archiveUrlPath), idParam, id, tokenParam, token, redirectParam, listPath)
		},
	}

	writeHeader(w, h.cfg, "aread", "", "")
	h.serveTemplate(w, `
<!DOCTYPE html>
  <body>
    <p><a href="{{.ToggleListPath}}">{{.ToggleListString}}</a> - <a href="{{.AddPath}}">Add URL</a></p>
    {{ range .Pages }}
    <div class="list-entry">
      <div class="title"><a href="{{$.PagesPath}}/{{.Id}}/">{{.Title}}</a></div>
      <div class="orig"><a href="{{.OriginalUrl}}">{{host .OriginalUrl}}</a></div>
      <div class="details">
        <a href="{{toggleUrl .Id .Token}}">{{$.TogglePageString}}</a> - <span class="time">Added {{time .TimeAdded}}</span>
      </div>
    </div>
    {{ end }}
    <div>
      <span class="bookmarklets-label">Bookmarklets:</span>
      <div class="bookmarklet"><a {{.ReadBookmarkletHref}}>Add to list</a></div>
      <div class="bookmarklet"><a {{.KindleBookmarkletHref}}>Send to Kindle</a></div>
    </div>
  </body>
</html>`, d, fm)
}

func (h Handler) handleAuth(w http.ResponseWriter, r *http.Request) {
	if len(r.FormValue("p")) > 0 {
		if r.FormValue("u") == h.cfg.Username && r.FormValue("p") == h.cfg.Password {
			id := getSha1String(h.cfg.Username + "|" + h.cfg.Password + "|" + strconv.FormatInt(time.Now().UnixNano(), 10))
			if err := h.db.AddSession(id, r.RemoteAddr); err != nil {
				h.cfg.Logger.Printf("Unable to insert session: %v\n", err)
				http.Error(w, "Unable to insert session", http.StatusInternalServerError)
				return
			}
			h.cfg.Logger.Printf("Successful authentication attempt from %v\n", r.RemoteAddr)
			cookie := fmt.Sprintf("%s=%s;Path=%s;Max-Age=%d;Secure;HttpOnly", sessionCookieName, id, h.cfg.GetPath(), 86400*365*100)
			w.Header()["Set-Cookie"] = []string{cookie}
			http.Redirect(w, r, r.FormValue("r"), http.StatusFound)
			return
		} else {
			h.cfg.Logger.Printf("Bad authentication attempt from %v\n", r.RemoteAddr)
		}
	}

	writeHeader(w, h.cfg, "Auth", "", "")
	h.serveTemplate(w, `
  <body>
    <form method="post">
      <input type="hidden" name="r" value={{.Redirect}}>
      <table class="auth">
        <tr><td>Username</td><td><input type="text" name="u"></td></tr>
        <tr><td>Password</td><td><input type="password" name="p"></td></tr>
        <tr><td><input type="submit" value="Submit"></td></tr>
      </table>
    </form>
  </body>
</html>`, struct{ Redirect string }{Redirect: r.FormValue("r")}, template.FuncMap{})
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, h.cfg.GetPath()) {
		h.cfg.Logger.Printf("Got request with unexpected path \"%v\"", r.URL.Path)
		http.Error(w, "Unexpected path", http.StatusInternalServerError)
		return
	}
	reqPath := r.URL.Path[len(h.cfg.GetPath()):]
	if strings.HasPrefix(reqPath, "/") {
		reqPath = reqPath[1:]
	}

	if strings.HasPrefix(reqPath, staticUrlPath+"/") {
		h.staticHandler.ServeHTTP(w, r)
		return
	}
	if reqPath == "favicon.ico" {
		http.Redirect(w, r, h.cfg.GetPath(staticUrlPath, "favicon.ico"), http.StatusFound)
		return
	}
	if reqPath == authUrlPath {
		h.handleAuth(w, r)
		return
	}

	// Everything else requires authentication.
	if !h.isAuthenticated(r) {
		h.cfg.Logger.Printf("Unauthenticated request from %v\n", r.RemoteAddr)
		http.Redirect(w, r, h.cfg.GetPath(authUrlPath+"?"+redirectParam+"="+r.URL.Path), http.StatusFound)
		return
	}

	if len(reqPath) == 0 {
		h.handleList(w, r)
	} else if reqPath == addUrlPath {
		h.handleAdd(w, r)
	} else if reqPath == archiveUrlPath {
		h.handleArchive(w, r)
	} else if reqPath == kindleUrlPath {
		h.handleKindle(w, r)
	} else if strings.HasPrefix(reqPath, pagesUrlPath+"/") {
		h.pageHandler.ServeHTTP(w, r)
	} else {
		http.Error(w, "Bogus request", http.StatusBadRequest)
	}
}
