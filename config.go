package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
)

type Config struct {
	ApiToken         string
	BaseUrl          string
	StaticDir        string
	PageDir          string
	Database         string
	MailServer       string
	Recipient        string
	Sender           string
	Username         string
	Password         string
	BookmarkletToken string
	DownloadImages   bool
	MaxListSize      int

	// Automatically derived values.
	BaseUrlPath string
	Logger      *log.Logger
}

func readConfig(configPath string, logger *log.Logger) (cfg Config, err error) {
	cfg.Logger = logger
	cfg.PageDir = "/tmp"
	cfg.DownloadImages = true
	cfg.MaxListSize = 50
	f, err := os.Open(configPath)
	if err != nil {
		return cfg, err
	}
	defer f.Close()
	d := json.NewDecoder(f)
	if err = d.Decode(&cfg); err != nil {
		return cfg, err
	}

	u, err := url.Parse(cfg.BaseUrl)
	if err != nil {
		return cfg, fmt.Errorf("Unable to parse base URL %v: %v\n", cfg.BaseUrl, err)
	}
	cfg.BaseUrlPath = u.Path

	return cfg, nil
}
