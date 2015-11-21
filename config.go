package main

import (
	"encoding/json"
	"log"
	"net/url"
	"os"
	"path"
)

type Config struct {
	ApiToken         string
	BaseUrl          string
	StaticDir        string
	PageDir          string
	UrlPatternsFile  string
	BadContentFile   string
	HiddenTagsFile   string
	Database         string
	MailServer       string
	Recipient        string
	Sender           string
	Username         string
	Password         string
	DownloadImages   bool
	MaxImageWidth    int
	MaxImageHeight   int
	MaxImageBytes    int64
	JpegQuality      int
	MaxImageProcs    int
	DownloadFavicons bool
	MaxListSize      int
	Verbose          bool
	Logger           *log.Logger
}

func readConfig(configPath string, logger *log.Logger) (cfg Config, err error) {
	cfg.Logger = logger
	cfg.PageDir = "/tmp"
	cfg.DownloadImages = true
	cfg.MaxImageWidth = 1024
	cfg.MaxImageHeight = 768
	cfg.MaxImageBytes = 1 * 1024 * 1024
	cfg.JpegQuality = 85
	cfg.MaxImageProcs = 3
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

	if cfg.BaseUrl[len(cfg.BaseUrl)-1] == '/' {
		cfg.BaseUrl = cfg.BaseUrl[:len(cfg.BaseUrl)-1]
	}

	return cfg, nil
}

func (c *Config) GetPath(p ...string) string {
	u, err := url.Parse(c.BaseUrl)
	if err != nil {
		c.Logger.Fatalf("Unable to parse base URL %v: %v\n", c.BaseUrl, err)
	}

	p = append(p, "")
	copy(p[1:], p[0:])
	p[0] = u.Path
	return path.Join(p...)
}
