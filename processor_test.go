package main

import (
	"log"
	"os"
	"testing"
)

const (
	badContentFile  = "testdata/bad_content.json"
	urlPatternsFile = "testdata/url_patterns.json"
)

func TestRewriteUrl(t *testing.T) {
	cfg := Config{
		UrlPatternsFile: urlPatternsFile,
		Logger:          log.New(os.Stderr, "", log.LstdFlags),
	}
	p := newProcessor(cfg)

	for _, tc := range []struct {
		OrigUrl string
		NewUrl  string
	}{
		{"http://m.example.com/index.html?r=1", "http://example.com/index.html"},
	} {
		if url, err := p.rewriteUrl(tc.OrigUrl); err != nil {
			t.Errorf("got error when rewriting %q: %v", tc.OrigUrl, err)
		} else if url != tc.NewUrl {
			t.Errorf("didn't rewrite %q correctly:\nexpected: %q\n  actual: %q", tc.OrigUrl, tc.NewUrl, url)
		}
	}
}

func TestCheckContent(t *testing.T) {
	cfg := Config{
		BadContentFile: badContentFile,
		Logger:         log.New(os.Stderr, "", log.LstdFlags),
	}
	p := newProcessor(cfg)

	for _, tc := range []struct {
		Url     string
		Content string
		Okay    bool
	}{
		{"http://www.example.com/good.html", "<html><body><h1>Hi!</h1></body></html>", true},
		{"http://www.example.com/bad.html", "<html><body><h1>Go away.</h1></body></html>", false},
		{"http://www.example.net/bad.html", "<html><body><h1>Go away.</h1></body></html>", true},
		{"http://www.example.net/really_bad.html", "<html><body><h1>Really go away.</h1></body></html>", false},
	} {
		err := p.checkContent(PageInfo{OriginalUrl: tc.Url}, tc.Content)
		if tc.Okay && err != nil {
			t.Errorf("got error for %q: %v", tc.Url, err)
		} else if !tc.Okay && err == nil {
			t.Errorf("didn't get expected error for %q", tc.Url)
		}
	}
}
