package common

import (
	"log"
	"net/url"
	"path"
)

type Config struct {
	ParserPath        string `json:"parserPath"`
	BaseURL           string `json:"baseUrl"`
	StaticDir         string `json:"staticDir"`
	PageDir           string `json:"pageDir"`
	URLPatternsFile   string `json:"urlPatternsFile"`
	BadContentFile    string `json:"badContentFile"`
	HiddenTagsFile    string `json:"hiddenTagsFile"`
	Database          string `json:"database"`
	MailServer        string `json:"mailServer"`
	Recipient         string `json:"recipient"`
	Sender            string `json:"sender"`
	Username          string `json:"username"`
	Password          string `json:"password"`
	FriendBaseURL     string `json:"friendBaseUrl"`
	FriendRemoteToken string `json:"friendRemoteToken"`
	FriendLocalToken  string `json:"friendLocalToken"`
	FriendTitlePrefix string `json:"friendTitlePrefix"`
	DownloadImages    bool   `json:"downloadImages"`
	MaxImageWidth     int    `json:"maxImageWidth"`
	MaxImageHeight    int    `json:"maxImageHeight"`
	MaxImageBytes     int64  `json:"maxImageBytes"`
	JPEGQuality       int    `json:"jpegQuality"`
	MaxImageProcs     int    `json:"maxImageProcs"`
	DownloadFavicons  bool   `json:"downloadFavicons"`
	MaxListSize       int    `json:"maxListSize"`
	Verbose           bool   `json:"verbose"`
	Logger            *log.Logger
}

func ReadConfig(p string, lg *log.Logger) (*Config, error) {
	cfg := Config{
		Logger:         lg,
		PageDir:        "/tmp",
		DownloadImages: true,
		MaxImageWidth:  1024,
		MaxImageHeight: 768,
		MaxImageBytes:  1 * 1024 * 1024,
		JPEGQuality:    85,
		MaxImageProcs:  3,
		MaxListSize:    50,
	}

	if err := ReadJSONFile(p, &cfg); err != nil {
		return nil, err
	}

	if cfg.BaseURL[len(cfg.BaseURL)-1] == '/' {
		cfg.BaseURL = cfg.BaseURL[:len(cfg.BaseURL)-1]
	}
	return &cfg, nil
}

func (cfg *Config) GetPath(p ...string) string {
	u, err := url.Parse(cfg.BaseURL)
	if err != nil {
		cfg.Logger.Fatalf("Unable to parse base URL %v: %v\n", cfg.BaseURL, err)
	}

	p = append(p, "")
	copy(p[1:], p[0:])
	p[0] = u.Path
	return path.Join(p...)
}
