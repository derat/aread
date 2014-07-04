package main

import (
	"log"
	"net/http"
)

type Handler struct {
	Password string

	processor     *Processor
	logger        *log.Logger
	basePath      string
	staticHandler http.Handler
}

func newHandler(p *Processor, l *log.Logger, basePath, staticDir string) *Handler {
	return &Handler{
		processor:     p,
		logger:        l,
		basePath:      basePath,
		staticHandler: http.StripPrefix(basePath, http.FileServer(http.Dir(staticDir))),
	}
}

func (h Handler) checkPassword(rw http.ResponseWriter, r *http.Request) bool {
	if len(h.Password) > 0 && r.FormValue("p") != h.Password {
		h.logger.Printf("Got request with invalid password from %v\n", r.RemoteAddr)
		rw.Write([]byte("Nope."))
		return false
	}
	return true
}

func (h Handler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	if len(r.FormValue("u")) > 0 {
		if !h.checkPassword(rw, r) {
			return
		}
		url := r.FormValue("u")
		sendToKindle := r.FormValue("k") == "1"
		_, err := h.processor.ProcessUrl(url, sendToKindle)
		if err != nil {
			h.logger.Println(err)
			rw.Write([]byte("Got an error. :-("))
		} else {
			rw.Write([]byte("Done!"))
		}
	} else if r.URL.Path == h.basePath || r.URL.Path == h.basePath+"/" {
		if !h.checkPassword(rw, r) {
			return
		}
		// FIXME: serve doc list
	} else {
		h.staticHandler.ServeHTTP(rw, r)
	}
}
