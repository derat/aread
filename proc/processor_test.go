// Copyright 2020 Daniel Erat.
// All rights reserved.

package proc

import (
	"log"
	"os"
	"testing"

	"github.com/derat/aread/common"
)

const (
	badContentFile  = "testdata/bad_content.json"
	urlPatternsFile = "testdata/url_patterns.json"
)

func TestProcessor_RewriteURL(t *testing.T) {
	p := New(&common.Config{
		URLPatternsFile: urlPatternsFile,
		Logger:          log.New(os.Stderr, "", log.LstdFlags),
	})

	for _, tc := range []struct {
		OrigURL string
		NewURL  string
	}{
		{"http://m.example.com/index.html?r=1", "http://example.com/index.html"},
	} {
		if url, err := p.rewriteURL(tc.OrigURL); err != nil {
			t.Errorf("got error when rewriting %q: %v", tc.OrigURL, err)
		} else if url != tc.NewURL {
			t.Errorf("didn't rewrite %q correctly:\nexpected: %q\n  actual: %q", tc.OrigURL, tc.NewURL, url)
		}
	}
}

func TestProcessor_CheckContent(t *testing.T) {
	p := New(&common.Config{
		BadContentFile: badContentFile,
		Logger:         log.New(os.Stderr, "", log.LstdFlags),
	})

	for _, tc := range []struct {
		URL     string
		Content string
		Okay    bool
	}{
		{"http://www.example.com/good.html", "<html><body><h1>Hi!</h1></body></html>", true},
		{"http://www.example.com/bad.html", "<html><body><h1>Go away.</h1></body></html>", false},
		{"http://www.example.net/bad.html", "<html><body><h1>Go away.</h1></body></html>", true},
		{"http://www.example.net/really_bad.html", "<html><body><h1>Really go away.</h1></body></html>", false},
	} {
		err := p.checkContent(common.PageInfo{OriginalURL: tc.URL}, tc.Content)
		if tc.Okay && err != nil {
			t.Errorf("got error for %q: %v", tc.URL, err)
		} else if !tc.Okay && err == nil {
			t.Errorf("didn't get expected error for %q", tc.URL)
		}
	}
}
