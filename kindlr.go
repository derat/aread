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
	processor *Processor
	password  string
}

func (h handler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	if len(h.password) > 0 && r.FormValue("p") != h.password {
		h.processor.Logger.Printf("Got request with invalid password from %v\n", r.RemoteAddr)
		rw.Write([]byte("Nope."))
		return
	}

	_, err := h.processor.ProcessUrl(r.FormValue("u"))
	if err != nil {
		h.processor.Logger.Println(err)
		rw.Write([]byte("Got an error. :-("))
	} else {
		rw.Write([]byte("Done!"))
	}
}

type config struct {
	ApiToken       string
	OutputDir      string
	MailServer     string
	Recipient      string
	Sender         string
	Password       string
	DownloadImages bool
}

func readConfig(configPath string) config {
	c := config{OutputDir: "/tmp", DownloadImages: true}
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
	p := NewProcessor()
	p.ApiToken = c.ApiToken
	p.BaseOutputDir = c.OutputDir
	p.MailServer = c.MailServer
	p.Recipient = c.Recipient
	p.Sender = c.Sender
	p.DownloadImages = c.DownloadImages

	if len(flag.Args()) > 0 {
		for i := range flag.Args() {
			url := flag.Args()[i]
			outputDir, err := p.ProcessUrl(url)
			if err != nil {
				log.Println(err)
			} else {
				log.Printf("%v -> %v\n", url, outputDir)
			}
		}
	} else {
		var err error
		if p.Logger, err = syslog.NewLogger(syslog.LOG_INFO|syslog.LOG_DAEMON, log.LstdFlags); err != nil {
			log.Fatalf("Unable to connect to syslog: %v\n", err)
		}
		h := handler{processor: p, password: c.Password}
		fcgi.Serve(nil, h)
	}
}
