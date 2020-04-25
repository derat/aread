package main

import (
	"flag"
	"fmt"
	"log"
	"log/syslog"
	"net/http/fcgi"
	"os"
	"path/filepath"

	"github.com/derat/aread/common"
	"github.com/derat/aread/db"
	"github.com/derat/aread/proc"
)

func main() {
	var configPath string
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [option]... <url>\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.StringVar(&configPath, "config", filepath.Join(os.Getenv("HOME"), ".aread.json"), "Path to JSON config file")
	flag.Parse()

	var logger *log.Logger
	daemon := len(flag.Args()) == 0
	if daemon {
		var err error
		if logger, err = syslog.NewLogger(syslog.LOG_INFO|syslog.LOG_DAEMON, log.LstdFlags); err != nil {
			log.Fatalf("Unable to connect to syslog: %v\n", err)
		}
	} else {
		logger = log.New(os.Stderr, "", log.LstdFlags)
	}

	cfg, err := common.ReadConfig(configPath, logger)
	if err != nil {
		logger.Fatalf("Unable to read config from %v: %v\n", configPath, err)
	}

	if !daemon {
		cfg.Verbose = true
	}

	p := proc.New(cfg)

	if daemon {
		db, err := db.New(cfg.Database)
		if err != nil {
			logger.Fatalln(err)
		}
		logger.Println("Accepting connections")
		fcgi.Serve(nil, newHandler(cfg, p, db))
	} else {
		for i := range flag.Args() {
			url := flag.Args()[i]
			if pi, err := p.ProcessURL(url, false); err == nil {
				logger.Printf("Processed %v (%v)\n", url, pi.Title)
			} else {
				logger.Println(err)
			}
		}
	}
}
