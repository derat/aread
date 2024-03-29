// Copyright 2020 Daniel Erat.
// All rights reserved.

package main

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/derat/aread/common"
	"github.com/derat/aread/db"
	"github.com/derat/aread/proc"
)

const sessionCookieName = "session"

type handler struct {
	cfg           *common.Config
	proc          *proc.Processor
	db            *db.Database
	staticHandler http.Handler
	pageHandler   http.Handler
}

func newHandler(cfg *common.Config, proc *proc.Processor, db *db.Database) handler {
	return handler{
		cfg:  cfg,
		proc: proc,
		db:   db,
		staticHandler: http.StripPrefix(cfg.GetPath(common.StaticURLPath),
			http.FileServer(http.Dir(cfg.StaticDir))),
		pageHandler: http.StripPrefix(cfg.GetPath(common.PagesURLPath),
			http.FileServer(http.Dir(cfg.PageDir))),
	}
}

func (h handler) getStylesheets() []string {
	return []string{h.cfg.GetPath(common.StaticURLPath, common.CommonCSSFile),
		h.cfg.GetPath(common.StaticURLPath, common.AppCSSFile)}
}

func (h handler) getAddToken() string {
	return common.SHA1String(h.cfg.Username + "|" + h.cfg.Password)
}

type bookmarkletFlags uint32

const (
	sendToKindle bookmarkletFlags = 1 << iota
	archive
)

func (h handler) makeBookmarklet(baseURL string, token string, flags bookmarkletFlags) string {
	addURL := joinURLAndPath(baseURL, common.AddURLPath) +
		fmt.Sprintf(`?%s="+encodeURIComponent(window.location.href)+"&%s=%s`,
			common.AddURLParam, common.TokenParam, token)
	if flags&sendToKindle != 0 {
		addURL += fmt.Sprintf("&%s=1", common.AddKindleParam)
	}
	if flags&archive != 0 {
		addURL += fmt.Sprintf("&%s=1", common.ArchiveParam)
	}
	return `javascript:{window.location.href="` + addURL + `";};void(0);`
}

func (h handler) serveTemplate(w http.ResponseWriter, t string, d interface{}, fm template.FuncMap) {
	if err := common.WriteTemplate(w, h.cfg, t, d, fm); err != nil {
		http.Error(w, fmt.Sprintf("Template error: %v", err), http.StatusInternalServerError)
	}
}

func (h handler) isAuthenticated(r *http.Request) bool {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}
	isAuth, err := h.db.ValidSession(c.Value)
	if err != nil {
		h.cfg.Logger.Println(err)
		return false
	}
	return isAuth
}

func (h handler) isFriend(r *http.Request) bool {
	return len(h.cfg.FriendLocalToken) > 0 && r.FormValue(common.TokenParam) == h.cfg.FriendLocalToken
}

func (h handler) handleAdd(w http.ResponseWriter, r *http.Request) {
	u := r.FormValue(common.AddURLParam)
	if len(u) > 0 {
		isFriend := h.isFriend(r)
		if !isFriend && r.FormValue(common.TokenParam) != h.getAddToken() {
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
		if err := h.db.AddPage(pi); err != nil {
			h.cfg.Logger.Println(err)
			http.Error(w, fmt.Sprintf("Failed to add to database: %v", err), http.StatusInternalServerError)
			return
		}
		if r.FormValue(common.ArchiveParam) == "1" {
			if err := h.db.TogglePageArchived(pi.Id); err != nil {
				h.cfg.Logger.Println(err)
				http.Error(w, fmt.Sprintf("Failed to archive page: %v", err), http.StatusInternalServerError)
				return
			}
		}
		if r.FormValue(common.AddKindleParam) == "1" {
			if err = h.proc.SendToKindle(pi.Id); err != nil {
				h.cfg.Logger.Println(err)
				http.Error(w, fmt.Sprintf("Failed to send to Kindle: %v", err), http.StatusInternalServerError)
				return
			}
		}

		if isFriend {
			common.WriteHeader(w, h.cfg, h.getStylesheets(), "Added page", "", "")
			h.serveTemplate(w, `
  <body>
	<p>Successfully added {{.URL}}!
  </body>
</html>`, struct{ URL string }{URL: u}, template.FuncMap{})
		} else {
			http.Redirect(w, r, h.cfg.GetPath(common.PagesURLPath, pi.Id), http.StatusFound)
		}
		return
	}

	common.WriteHeader(w, h.cfg, h.getStylesheets(), "Add", "", "")
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
	pi, err := h.db.GetPage(r.FormValue(common.IDParam))
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to find page: %v", err), http.StatusBadRequest)
		return
	}
	if len(pi.Token) > 0 && r.FormValue(common.TokenParam) != pi.Token {
		h.cfg.Logger.Printf("Bad or missing token in archive request from %v\n", r.RemoteAddr)
		http.Error(w, "Invalid token", http.StatusBadRequest)
		return
	}
	if err := h.db.TogglePageArchived(pi.Id); err != nil {
		h.cfg.Logger.Println(err)
		http.Error(w, fmt.Sprintf("Failed to toggle archived state: %v", err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, r.FormValue(common.RedirectParam), http.StatusFound)
}

func (h handler) handleKindle(w http.ResponseWriter, r *http.Request) {
	pi, err := h.db.GetPage(r.FormValue(common.IDParam))
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to find page: %v", err), http.StatusBadRequest)
		return
	}
	if len(pi.Token) > 0 && r.FormValue(common.TokenParam) != pi.Token {
		h.cfg.Logger.Printf("Bad or missing token in kindle request from %v\n", r.RemoteAddr)
		http.Error(w, "Invalid token", http.StatusBadRequest)
		return
	}
	if err := h.proc.SendToKindle(pi.Id); err != nil {
		h.cfg.Logger.Println(err)
		http.Error(w, fmt.Sprintf("Failed to send to Kindle: %v", err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, r.FormValue(common.RedirectParam), http.StatusFound)
}

func (h handler) handleList(w http.ResponseWriter, r *http.Request) {
	d := struct {
		Pages                 []common.PageInfo
		PagesPath             string
		TogglePagePath        string
		TogglePageString      string
		ToggleListPath        string
		ToggleListString      string
		AddPath               string
		ReadBookmarkletHref   template.HTMLAttr
		SaveBookmarkletHref   template.HTMLAttr
		KindleBookmarkletHref template.HTMLAttr
		FriendBookmarkletHref template.HTMLAttr
	}{
		PagesPath:             h.cfg.GetPath(common.PagesURLPath),
		AddPath:               h.cfg.GetPath(common.AddURLPath),
		ReadBookmarkletHref:   template.HTMLAttr("href=" + h.makeBookmarklet(h.cfg.BaseURL, h.getAddToken(), 0)),
		SaveBookmarkletHref:   template.HTMLAttr("href=" + h.makeBookmarklet(h.cfg.BaseURL, h.getAddToken(), archive)),
		KindleBookmarkletHref: template.HTMLAttr("href=" + h.makeBookmarklet(h.cfg.BaseURL, h.getAddToken(), sendToKindle)),
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
		d.FriendBookmarkletHref =
			template.HTMLAttr("href=" + h.makeBookmarklet(h.cfg.FriendBaseURL, h.cfg.FriendRemoteToken, sendToKindle))
	}

	var err error
	if d.Pages, err = h.db.GetAllPages(archived, h.cfg.MaxListSize); err != nil {
		h.cfg.Logger.Printf("Unable to get pages: %v\n", err)
		http.Error(w, fmt.Sprintf("Unable to get page list: %v", err), http.StatusInternalServerError)
		return
	}

	fm := template.FuncMap{
		"host": common.GetHost,
		"time": func(t int64) string { return time.Unix(t, 0).Format("Monday, Jan 2 at 15:04") },
		"toggleURL": func(id, token string) string {
			listPath := unarchivedListPath
			if archived {
				listPath = archivedListPath
			}
			return fmt.Sprintf("%s?%s=%s&%s=%s&%s=%s", h.cfg.GetPath(common.ArchiveURLPath),
				common.IDParam, id, common.TokenParam, token, common.RedirectParam, listPath)
		},
	}

	common.WriteHeader(w, h.cfg, h.getStylesheets(), "aread", "", "")
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
      <div class="bookmarklet"><a {{.ReadBookmarkletHref}}>Add</a></div>
      <div class="bookmarklet"><a {{.SaveBookmarkletHref}}>Save</a></div>
      <div class="bookmarklet"><a {{.KindleBookmarkletHref}}>Kindle</a></div>
	  {{if .FriendBookmarkletHref}}<div class="bookmarklet"><a {{.FriendBookmarkletHref}}>Friend's Kindle</a></div>{{end}}
    </div>
  </body>
</html>`, d, fm)
}

func (h handler) handleAuth(w http.ResponseWriter, r *http.Request) {
	if len(r.FormValue("p")) > 0 {
		if r.FormValue("u") == h.cfg.Username && r.FormValue("p") == h.cfg.Password {
			id := common.SHA1String(fmt.Sprintf("%s|%s|%d", h.cfg.Username, h.cfg.Password, time.Now().UnixNano()))
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

	common.WriteHeader(w, h.cfg, h.getStylesheets(), "Auth", "", "")
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

	if strings.HasPrefix(reqPath, common.StaticURLPath+"/") {
		h.staticHandler.ServeHTTP(w, r)
		return
	}
	if reqPath == "favicon.ico" {
		http.Redirect(w, r, h.cfg.GetPath(common.StaticURLPath, "favicon.ico"), http.StatusFound)
		return
	}
	if reqPath == common.AuthURLPath {
		h.handleAuth(w, r)
		return
	}

	// Everything else requires authentication.
	if !h.isAuthenticated(r) && !(reqPath == common.AddURLPath && h.isFriend(r)) {
		h.cfg.Logger.Printf("Unauthenticated request from %v\n", r.RemoteAddr)
		path := h.cfg.GetPath(fmt.Sprintf("%s?%s=%s", common.AuthURLPath, common.RedirectParam, r.URL.Path))
		http.Redirect(w, r, path, http.StatusFound)
		return
	}

	if len(reqPath) == 0 {
		h.handleList(w, r)
	} else if reqPath == common.AddURLPath {
		h.handleAdd(w, r)
	} else if reqPath == common.ArchiveURLPath {
		h.handleArchive(w, r)
	} else if reqPath == common.KindleURLPath {
		h.handleKindle(w, r)
	} else if strings.HasPrefix(reqPath, common.PagesURLPath+"/") {
		h.pageHandler.ServeHTTP(w, r)
	} else {
		http.Error(w, "Bogus request", http.StatusBadRequest)
	}
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
