package common

import (
	"log"
	"net/url"
	"path"
)

type Config struct {
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

func ReadConfig(p string, lg *log.Logger) (*Config, error) {
	cfg := Config{
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
