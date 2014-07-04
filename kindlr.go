package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"log/syslog"
	"net/http/fcgi"
	"os"
	"path/filepath"
)

type config struct {
	ApiToken       string
	BaseUrlPath    string
	StaticDir      string
	PageDir        string
	Database       string
	MailServer     string
	Recipient      string
	Sender         string
	Password       string
	DownloadImages bool
}

func readConfig(configPath string) config {
	c := config{PageDir: "/tmp", DownloadImages: true}
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
	p.BaseOutputDir = c.PageDir
	p.BaseUrlPath = c.BaseUrlPath
	p.MailServer = c.MailServer
	p.Recipient = c.Recipient
	p.Sender = c.Sender
	p.DownloadImages = c.DownloadImages

	if len(flag.Args()) > 0 {
		for i := range flag.Args() {
			url := flag.Args()[i]
			pi, err := p.ProcessUrl(url, false)
			if err != nil {
				log.Println(err)
			} else {
				log.Printf("Processed %v (%v)\n", url, pi.Title)
			}
		}
	} else {
		logger, err := syslog.NewLogger(syslog.LOG_INFO|syslog.LOG_DAEMON, log.LstdFlags)
		if err != nil {
			log.Fatalf("Unable to connect to syslog: %v\n", err)
		}
		p.Logger = logger

		d, err := NewDatabase(c.Database)
		if err != nil {
			log.Fatalln(err)
		}

		h := NewHandler(p, d, logger, c.BaseUrlPath, c.StaticDir, c.PageDir)
		h.Password = c.Password
		fcgi.Serve(nil, *h)
	}
}
