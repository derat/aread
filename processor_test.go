package main

import (
	"log"
	"os"
	"testing"
)

const (
	urlPatternsFile = "testdata/url_patterns.json"
)

func TestRewriteUrl(t *testing.T) {
	cfg := Config{
		UrlPatternsFile: urlPatternsFile,
		Logger:          log.New(os.Stderr, "", log.LstdFlags),
	}
	p := Processor{cfg}

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
