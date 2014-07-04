package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"log/syslog"
	"net/http"
	"net/http/fcgi"
	"os"
	"path/filepath"
)

type handler struct {
	fetcher  *contentFetcher
	password string
}

func (h handler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	if len(h.password) > 0 && r.FormValue("p") != h.password {
		h.fetcher.Logger.Printf("Got request with invalid password from %v\n", r.RemoteAddr)
		rw.Write([]byte("Nope."))
	} else if err := h.fetcher.ProcessUrl(r.FormValue("u")); err != nil {
		h.fetcher.Logger.Println(err)
		rw.Write([]byte("Got an error. :-("))
	} else {
		rw.Write([]byte("Done!"))
	}
}

type config struct {
	ApiToken       string
	MailServer     string
	Recipient      string
	Sender         string
	Password       string
	DownloadImages bool
}

func readConfig(configPath string) config {
	c := config{DownloadImages: true}
	f, err := os.Open(configPath)
	if err != nil {
		log.Fatalf("Unable to open config file %v: %v\n", configPath, err)
	}
	defer f.Close()
	d := json.NewDecoder(f)
	if err = d.Decode(&c); err != nil {
		log.Fatalf("Unable to read JSON from %v: %v\n", configPath, err)
	}
	return c
}

func main() {
	var configPath string
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [option]... <url>\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.StringVar(&configPath, "config", filepath.Join(os.Getenv("HOME"), ".kindlr"), "Path to JSON config file")
	flag.Parse()

	c := readConfig(configPath)
	f := NewContentFetcher()
	f.ApiToken = c.ApiToken
	f.MailServer = c.MailServer
	f.Recipient = c.Recipient
	f.Sender = c.Sender
	f.DownloadImages = c.DownloadImages

	if len(flag.Args()) > 0 {
		for i := range flag.Args() {
			if err := f.ProcessUrl(flag.Args()[i]); err != nil {
				log.Println(err)
			}
		}
	} else {
		var err error
		if f.Logger, err = syslog.NewLogger(syslog.LOG_INFO|syslog.LOG_DAEMON, log.LstdFlags); err != nil {
			log.Fatalf("Unable to connect to syslog: %v\n", err)
		}
		h := handler{fetcher: f, password: c.Password}
		fcgi.Serve(nil, h)
	}
}
