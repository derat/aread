// Copyright 2020 Daniel Erat.
// All rights reserved.

package common

import (
	"log"
	"net/url"
	"path"
)

// Config contains the server's configuration.
type Config struct {
	// ParserPath contains the path to the mercury-parser executable,
	// e.g. "/home/user/.node/bin/mercury-parser". See installation instructions
	// at https://github.com/postlight/mercury-parser.
	ParserPath string `json:"parserPath"`
	// KindlegenPath contains the path to the kindlegen executable,
	// e.g. "/usr/local/bin/kindlegen". kindlegen is available from
	// https://www.amazon.com/gp/feature.html?ie=UTF8&docId=1000765211.
	KindlegenPath string `json:"kindlegenPath"`
	// BaseURL contains the base URL at which the site is served,
	// e.g. "https://example.org/aread".
	BaseURL string `json:"baseUrl"`
	// StaticDir is the path to this repository's static/ directory,
	// e.g. "/home/user/aread/static".
	StaticDir string `json:"staticDir"`
	// PageDir is the directory under which pages will be saved,
	// e.g. "/var/lib/aread/pages".
	PageDir string `json:"pageDir"`
	// URLPatternsFile is the path to a file containing URL rewrite patterns,
	// e.g. "/var/lib/aread/url_patterns.json". The file consists of a JSON
	// array containing 2-element arrays with the source regular expression and
	// replacement pattern. For example:
	//
	//   [
	//     ["^(https?://)mobile\\.nytimes\\.com/", "${1}nytimes.com/"],
	//     ["([./]nytimes\\.com/.*)\\?.*$", "$1"]
	//   ]
	URLPatternsFile string `json:"urlPatternsFile"`
	// BadContentFile is the path to a file containing patterns used to
	// detect bad (e.g. paywalled) content. The file consists of a JSON array
	// containing 2-element arrays with a URL regular expresion and a string to
	// search for. For example:
	//
	//   [
	//     ["[./]nytimes\\.com/", "To save articles or get newsletters, alerts or recommendations"],
	//     ["[./]forbes\\.com/", "<span class=\"dynamic-css\">false</span>"],
	//     ["[./]time\\.com/", "One of the main ways we cover our costs is through advertising"]
	//   ]
	BadContentFile string `json:"badContentFile"`
	// HiddenTagsFile is the path to a file listing tags to strip out from
	// pages. The file consists of a JSON object, where keys are URL wildcards
	// and values are arrays of element.class patterns. For example:
	//   {
	//     "*": [
	//       "*.jp-relatedposts-headline",
	//       "div.articleOptions",
	//       "div.sharedaddy",
	//       "p.jp-relatedposts",
	//       "span.print-link"
	//     ],
	//     "adage.com": [
	//       "figcaption.*"
	//     ]
	//   }
	HiddenTagsFile string `json:"hiddenTagsFile"`
	// Database is the path to the SQLite database containing page information,
	// e.g. "/var/lib/aread/data/aread.db".
	Database string `json:"database"`
	// MailServer contains the SMTP server used to mail documents to Kindle
	// devices as "hostname:port", e.g. "localhost:25".
	MailServer string `json:"mailServer"`
	// Recipient contains the email address where documents should be mailed,
	// e.g. "my-name_123@kindle.com".
	Recipient string `json:"recipient"`
	// Sender contains the sender email address used when mailing documents,
	// e.g. "user@example.org".
	Sender string `json:"sender"`
	// Username contains a basic HTTP authentication username.
	Username string `json:"username"`
	// Password contains a basic HTTP authentication password.
	Password          string `json:"password"`
	FriendBaseURL     string `json:"friendBaseUrl"`
	FriendRemoteToken string `json:"friendRemoteToken"`
	FriendLocalToken  string `json:"friendLocalToken"`
	FriendTitlePrefix string `json:"friendTitlePrefix"`
	// DownloadImages controls whether a page's images should be downloaded.
	// It is true by default.
	DownloadImages bool `json:"downloadImages"`
	// MaxImageWidth contains the maximum width in pixels of downloaded images.
	// It defaults to 1024.
	MaxImageWidth int `json:"maxImageWidth"`
	// MaxImageHeight contains the maximum height in pixels of downloaded images.
	// It defaults to 768.
	MaxImageHeight int `json:"maxImageHeight"`
	// MaxImageBytes contains the maximum size of downloaded images.
	// Images larger than this after resizing will be deleted.
	MaxImageBytes int64 `json:"maxImageBytes"`
	// JPEGQuality contains the quality (up to 100) to use when saving JPEG
	// images after resizing them. It defaults to 85.
	JPEGQuality int `json:"jpegQuality"`
	// MaxImageProcs contains the maximum number of images to process
	// simultaneously. It defaults to 3.
	MaxImageProcs int `json:"maxImageProcs"`
	// DownloadFavicons controls whether pages' favicon images are saved.
	// It defaults to false.
	DownloadFavicons bool `json:"downloadFavicons"`
	// MaxListSize contains the maximum number of pages to list on the website.
	// It defaults to 50.
	MaxListSize int `json:"maxListSize"`
	// Verbose controls whether verbose logs are written.
	Verbose bool `json:"verbose"`
	// Logger is used to log messages.
	Logger *log.Logger `json:"-"`
}

// ReadConfig returns a new Config based on the JSON file at p.
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
