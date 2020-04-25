package main

import (
	"encoding/json"
	"log"
	"net/url"
	"os"
	"path"
)

type config struct {
	ParserPath        string
	BaseURL           string
	StaticDir         string
	PageDir           string
	URLPatternsFile   string
	BadContentFile    string
	HiddenTagsFile    string
	Database          string
	MailServer        string
	Recipient         string
	Sender            string
	Username          string
	Password          string
	FriendBaseURL     string
	FriendRemoteToken string
	FriendLocalToken  string
	FriendTitlePrefix string
	DownloadImages    bool
	MaxImageWidth     int
	MaxImageHeight    int
	MaxImageBytes     int64
	JpegQuality       int
	MaxImageProcs     int
	DownloadFavicons  bool
	MaxListSize       int
	Verbose           bool
	Logger            *log.Logger
}

func readConfig(p string, lg *log.Logger) (config, error) {
	cfg := config{
		Logger:         lg,
		PageDir:        "/tmp",
		DownloadImages: true,
		MaxImageWidth:  1024,
		MaxImageHeight: 768,
		MaxImageBytes:  1 * 1024 * 1024,
		JpegQuality:    85,
		MaxImageProcs:  3,
		MaxListSize:    50,
	}

	f, err := os.Open(p)
	if err != nil {
		return cfg, err
	}
	defer f.Close()

	d := json.NewDecoder(f)
	if err = d.Decode(&cfg); err != nil {
		return cfg, err
	}

	if cfg.BaseURL[len(cfg.BaseURL)-1] == '/' {
		cfg.BaseURL = cfg.BaseURL[:len(cfg.BaseURL)-1]
	}
	return cfg, nil
}

func (cfg *config) GetPath(p ...string) string {
	u, err := url.Parse(cfg.BaseURL)
	if err != nil {
		cfg.Logger.Fatalf("Unable to parse base URL %v: %v\n", cfg.BaseURL, err)
	}

	p = append(p, "")
	copy(p[1:], p[0:])
	p[0] = u.Path
	return path.Join(p...)
}
