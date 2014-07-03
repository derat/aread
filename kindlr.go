package main

import (
	"flag"
	"fmt"
	"log"
	"log/syslog"
	"net/http"
	"net/http/fcgi"
	"os"
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

func main() {
	f := NewContentFetcher()
	var password string

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [option]... <url>\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.StringVar(&f.BaseTempDir, "temp-dir", "/tmp", "Base temp directory")
	flag.BoolVar(&f.DownloadImages, "download-images", true, "Download and write local copies of images")
	flag.BoolVar(&f.KeepTempFiles, "keep-temp-files", false, "Keep temporary files")
	flag.StringVar(&f.MailServer, "mail-server", "localhost:25", "SMTP server host:port")
	flag.StringVar(&password, "password", "", "Password required for web requests")
	flag.StringVar(&f.Recipient, "recipient", "", "Recipient email address")
	flag.StringVar(&f.Sender, "sender", "", "Sender email address")
	flag.StringVar(&f.ApiToken, "token", "", "Readability.com Parser API token")
	flag.Parse()

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
		h := handler{fetcher: f, password: password}
		fcgi.Serve(nil, h)
	}
}
