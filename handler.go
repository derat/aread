package main

import (
	"log"
	"net/http"
	"path"
	"strings"
)

type Handler struct {
	Password string

	processor     *Processor
	db            *Database
	logger        *log.Logger
	baseUrlPath   string
	staticHandler http.Handler
	pageHandler   http.Handler
}

func NewHandler(p *Processor, d *Database, l *log.Logger, baseUrlPath, staticDir, pageDir string) *Handler {
	return &Handler{
		processor:     p,
		db:            d,
		logger:        l,
		baseUrlPath:   baseUrlPath,
		staticHandler: http.StripPrefix(baseUrlPath+"/"+staticUrlPath, http.FileServer(http.Dir(staticDir))),
		pageHandler:   http.StripPrefix(baseUrlPath+"/"+pagesUrlPath, http.FileServer(http.Dir(pageDir))),
	}
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

	if len(reqPath) == 0 {
		if len(h.Password) > 0 && r.FormValue("p") != h.Password {
			h.logger.Printf("Got request with invalid password from %v\n", r.RemoteAddr)
			rw.Write([]byte("Nope."))
			return
		}

		if len(r.FormValue("u")) > 0 {
			url := r.FormValue("u")
			sendToKindle := r.FormValue("k") == "1"
			pi, err := h.processor.ProcessUrl(url, sendToKindle)
			if len(pi.Id) > 0 {
				h.db.AddPage(pi)
			}

			if err != nil {
				h.logger.Println(err)
				rw.Write([]byte("Got an error. :-("))
			} else {
				pagePath := path.Join(h.baseUrlPath, pagesUrlPath, pi.Id)
				http.Redirect(rw, r, pagePath, http.StatusFound)
			}
			return
		}

		// FIXME: serve doc list
	} else if strings.HasPrefix(reqPath, staticUrlPath+"/") {
		h.staticHandler.ServeHTTP(rw, r)
	} else if strings.HasPrefix(reqPath, pagesUrlPath+"/") {
		h.pageHandler.ServeHTTP(rw, r)
	} else {
		// FIXME: 404
	}
}
