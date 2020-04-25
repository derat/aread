package main

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	addURLParam    = "u"
	addKindleParam = "k"
)

type handler struct {
	cfg           config
	proc          *processor
	db            *Database
	staticHandler http.Handler
	pageHandler   http.Handler
}

func newHandler(cfg config, proc *processor, db *Database) handler {
	return handler{
		cfg:           cfg,
		proc:          proc,
		db:            db,
		staticHandler: http.StripPrefix(cfg.GetPath(staticURLPath), http.FileServer(http.Dir(cfg.StaticDir))),
		pageHandler:   http.StripPrefix(cfg.GetPath(pagesURLPath), http.FileServer(http.Dir(cfg.PageDir))),
	}
}

func (h handler) getStylesheets() []string {
	return []string{h.cfg.GetPath(staticURLPath, commonCssFile), h.cfg.GetPath(staticURLPath, appCssFile)}
}

func (h handler) getAddToken() string {
	return getSHA1String(h.cfg.Username + "|" + h.cfg.Password)
}

func (h handler) makeBookmarklet(baseURL string, token string, kindle bool) string {
	getCurURL := "encodeURIComponent(document.URL)"
	addURL := joinURLAndPath(baseURL, addURLPath) +
		fmt.Sprintf("?%s=\"+%s+\"&%s=%s", addURLParam, getCurURL, tokenParam, token)
	if kindle {
		addURL += fmt.Sprintf("&%s=1", addKindleParam)
	}
	return "javascript:{window.location.href=\"" + addURL + "\";};void(0);"
}

func (h handler) serveTemplate(w http.ResponseWriter, t string, d interface{}, fm template.FuncMap) {
	if err := writeTemplate(w, h.cfg, t, d, fm); err != nil {
		http.Error(w, fmt.Sprintf("Template error: %v", err), http.StatusInternalServerError)
	}
}

func (h handler) isAuthenticated(r *http.Request) bool {
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

func (h handler) isFriend(r *http.Request) bool {
	return len(h.cfg.FriendLocalToken) > 0 && r.FormValue(tokenParam) == h.cfg.FriendLocalToken
}

func (h handler) handleAdd(w http.ResponseWriter, r *http.Request) {
	u := r.FormValue(addURLParam)
	if len(u) > 0 {
		isFriend := h.isFriend(r)
		if !isFriend && r.FormValue(tokenParam) != h.getAddToken() {
			h.cfg.Logger.Printf("Bad or missing token in add request from %v\n", r.RemoteAddr)
			http.Error(w, "Invalid token", http.StatusForbidden)
			return
		}

		pi, err := h.proc.ProcessURL(u, isFriend)
		if err != nil {
			h.cfg.Logger.Println(err)
			http.Error(w, fmt.Sprintf("Failed to process %v: %v", u, err), http.StatusInternalServerError)
			return
		}
		if err = h.db.AddPage(pi); err != nil {
			h.cfg.Logger.Println(err)
			http.Error(w, fmt.Sprintf("Failed to add to database: %v", err), http.StatusInternalServerError)
			return
		}
		if r.FormValue(addKindleParam) == "1" {
			if err = h.proc.SendToKindle(pi.Id); err != nil {
				h.cfg.Logger.Println(err)
				http.Error(w, fmt.Sprintf("Failed to send to Kindle: %v", err), http.StatusInternalServerError)
				return
			}
		}

		if isFriend {
			writeHeader(w, h.cfg, h.getStylesheets(), "Added page", "", "")
			h.serveTemplate(w, `
  <body>
	<p>Successfully added {{.URL}}!
  </body>
</html>`, struct{ URL string }{URL: u}, template.FuncMap{})
		} else {
			http.Redirect(w, r, h.cfg.GetPath(pagesURLPath, pi.Id), http.StatusFound)
		}
		return
	}

	writeHeader(w, h.cfg, h.getStylesheets(), "Add", "", "")
	h.serveTemplate(w, `
  <body>
    <form method="post">
      <table>
        <input type="hidden" name="t" value={{.Token}}>
        <tr>
          <td>URL</td>
          <td><input type="text" autofocus name="u" id="add-url"></td>
        </tr>
        <tr><td><input type="submit" value="Add"></td></tr>
      </table>
    </form>
  </body>
</html>`, struct{ Token string }{Token: h.getAddToken()}, template.FuncMap{})
}

func (h handler) handleArchive(w http.ResponseWriter, r *http.Request) {
	pi, err := h.db.GetPage(r.FormValue(idParam))
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to find page: %v", err), http.StatusBadRequest)
		return
	}
	if len(pi.Token) > 0 && r.FormValue(tokenParam) != pi.Token {
		h.cfg.Logger.Printf("Bad or missing token in archive request from %v\n", r.RemoteAddr)
		http.Error(w, "Invalid token", http.StatusBadRequest)
		return
	}
	if err := h.db.TogglePageArchived(pi.Id); err != nil {
		h.cfg.Logger.Println(err)
		http.Error(w, fmt.Sprintf("Failed to toggle archived state: %v", err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, r.FormValue(redirectParam), http.StatusFound)
}

func (h handler) handleKindle(w http.ResponseWriter, r *http.Request) {
	pi, err := h.db.GetPage(r.FormValue(idParam))
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to find page: %v", err), http.StatusBadRequest)
		return
	}
	if len(pi.Token) > 0 && r.FormValue(tokenParam) != pi.Token {
		h.cfg.Logger.Printf("Bad or missing token in kindle request from %v\n", r.RemoteAddr)
		http.Error(w, "Invalid token", http.StatusBadRequest)
		return
	}
	if err := h.proc.SendToKindle(pi.Id); err != nil {
		h.cfg.Logger.Println(err)
		http.Error(w, fmt.Sprintf("Failed to send to Kindle: %v", err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, r.FormValue(redirectParam), http.StatusFound)
}

func (h handler) handleList(w http.ResponseWriter, r *http.Request) {
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
		FriendBookmarkletHref template.HTMLAttr
	}{
		PagesPath:             h.cfg.GetPath(pagesURLPath),
		AddPath:               h.cfg.GetPath(addURLPath),
		ReadBookmarkletHref:   template.HTMLAttr("href=" + h.makeBookmarklet(h.cfg.BaseURL, h.getAddToken(), false)),
		KindleBookmarkletHref: template.HTMLAttr("href=" + h.makeBookmarklet(h.cfg.BaseURL, h.getAddToken(), true)),
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

	if len(h.cfg.FriendBaseURL) > 0 && len(h.cfg.FriendRemoteToken) > 0 {
		d.FriendBookmarkletHref = template.HTMLAttr("href=" + h.makeBookmarklet(h.cfg.FriendBaseURL, h.cfg.FriendRemoteToken, true))
	}

	var err error
	if d.Pages, err = h.db.GetAllPages(archived, h.cfg.MaxListSize); err != nil {
		h.cfg.Logger.Printf("Unable to get pages: %v\n", err)
		http.Error(w, fmt.Sprintf("Unable to get page list: %v", err), http.StatusInternalServerError)
		return
	}

	fm := template.FuncMap{
		"host": getHost,
		"time": func(t int64) string { return time.Unix(t, 0).Format("Monday, Jan 2 at 15:04") },
		"toggleURL": func(id, token string) string {
			listPath := unarchivedListPath
			if archived {
				listPath = archivedListPath
			}
			return fmt.Sprintf("%s?%s=%s&%s=%s&%s=%s", h.cfg.GetPath(archiveURLPath), idParam, id, tokenParam, token, redirectParam, listPath)
		},
	}

	writeHeader(w, h.cfg, h.getStylesheets(), "aread", "", "")
	h.serveTemplate(w, `
  <body>
    <p><a href="{{.ToggleListPath}}">{{.ToggleListString}}</a> - <a href="{{.AddPath}}">Add URL</a></p>
    {{ range .Pages }}
    <div class="list-entry">
      <div class="title"><a href="{{$.PagesPath}}/{{.Id}}/">{{.Title}}</a></div>
      <div class="orig"><a href="{{.OriginalURL}}">{{host .OriginalURL}}</a></div>
      <div class="details">
        <a href="{{toggleURL .Id .Token}}">{{$.TogglePageString}}</a> - <span class="time">Added {{time .TimeAdded}}</span>
      </div>
    </div>
    {{ end }}
    <div>
      <span class="bookmarklets-label">Bookmarklets:</span>
      <div class="bookmarklet"><a {{.ReadBookmarkletHref}}>Add to list</a></div>
      <div class="bookmarklet"><a {{.KindleBookmarkletHref}}>Send to Kindle</a></div>
	  {{if .FriendBookmarkletHref}}<div class="bookmarklet"><a {{.FriendBookmarkletHref}}>Send to Friend's Kindle</a></div>{{end}}
    </div>
  </body>
</html>`, d, fm)
}

func (h handler) handleAuth(w http.ResponseWriter, r *http.Request) {
	if len(r.FormValue("p")) > 0 {
		if r.FormValue("u") == h.cfg.Username && r.FormValue("p") == h.cfg.Password {
			id := getSHA1String(h.cfg.Username + "|" + h.cfg.Password + "|" + strconv.FormatInt(time.Now().UnixNano(), 10))
			if err := h.db.AddSession(id, r.RemoteAddr); err != nil {
				h.cfg.Logger.Printf("Unable to insert session: %v\n", err)
				http.Error(w, fmt.Sprintf("Unable to insert session: %v", err), http.StatusInternalServerError)
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

	writeHeader(w, h.cfg, h.getStylesheets(), "Auth", "", "")
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

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, h.cfg.GetPath()) {
		h.cfg.Logger.Printf("Got request with unexpected path \"%v\"", r.URL.Path)
		http.Error(w, "Unexpected path", http.StatusInternalServerError)
		return
	}
	reqPath := r.URL.Path[len(h.cfg.GetPath()):]
	if strings.HasPrefix(reqPath, "/") {
		reqPath = reqPath[1:]
	}

	if strings.HasPrefix(reqPath, staticURLPath+"/") {
		h.staticHandler.ServeHTTP(w, r)
		return
	}
	if reqPath == "favicon.ico" {
		http.Redirect(w, r, h.cfg.GetPath(staticURLPath, "favicon.ico"), http.StatusFound)
		return
	}
	if reqPath == authURLPath {
		h.handleAuth(w, r)
		return
	}

	// Everything else requires authentication.
	if !h.isAuthenticated(r) && !(reqPath == addURLPath && h.isFriend(r)) {
		h.cfg.Logger.Printf("Unauthenticated request from %v\n", r.RemoteAddr)
		http.Redirect(w, r, h.cfg.GetPath(authURLPath+"?"+redirectParam+"="+r.URL.Path), http.StatusFound)
		return
	}

	if len(reqPath) == 0 {
		h.handleList(w, r)
	} else if reqPath == addURLPath {
		h.handleAdd(w, r)
	} else if reqPath == archiveURLPath {
		h.handleArchive(w, r)
	} else if reqPath == kindleURLPath {
		h.handleKindle(w, r)
	} else if strings.HasPrefix(reqPath, pagesURLPath+"/") {
		h.pageHandler.ServeHTTP(w, r)
	} else {
		http.Error(w, "Bogus request", http.StatusBadRequest)
	}
}
