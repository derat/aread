package main

import (
	"testing"
)

func TestJoinURLAndPath(t *testing.T) {
	for _, tc := range []struct {
		url    string
		path   string
		output string
	}{
		{"https://www.example.com", "page.html", "https://www.example.com/page.html"},
		{"https://www.example.com/", "page.html", "https://www.example.com/page.html"},
		{"https://www.example.com", "/page.html", "https://www.example.com/page.html"},
		{"https://www.example.com/", "/page.html", "https://www.example.com/page.html"},
	} {
		out := joinURLAndPath(tc.url, tc.path)
		if out != tc.output {
			t.Errorf("joined %q and %q to %q instead of %q", tc.url, tc.path, out, tc.output)
		}
	}
}

func TestGetLocalImageFilename(t *testing.T) {
	for _, tc := range []struct {
		url            string
		expectedSuffix string
	}{
		{"http://a.com/img.png", ".png"},
		{"http://a.com/img.gif", ".gif"},
		{"http://a.com/img.jpg", ".jpg"},
		{"http://a.com/img.svg", ".svg"},
		{"http://a.com/img.jpeg", ".jpeg"},
		// Extensions preceding query strings should be found.
		{"http://a.com/img.png?q=foo", ".png"},
		// Missing or unknown extensions should use a default.
		{"http://a.com/img", ".jpg"},
		{"http://a.com/img.foo", ".jpg"},
	} {
		expected := getSha1String(tc.url) + tc.expectedSuffix
		actual := getLocalImageFilename(tc.url)
		if actual != expected {
			t.Errorf("got local filename %q for %v, expected %v", actual, tc.url, expected)
		}
	}
}
