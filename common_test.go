package main

import (
	"testing"
)

func TestJoinUrlAndPath(t *testing.T) {
	for _, tc := range []struct {
		Url    string
		Path   string
		Output string
	}{
		{"https://www.example.com", "page.html", "https://www.example.com/page.html"},
		{"https://www.example.com/", "page.html", "https://www.example.com/page.html"},
		{"https://www.example.com", "/page.html", "https://www.example.com/page.html"},
		{"https://www.example.com/", "/page.html", "https://www.example.com/page.html"},
	} {
		out := joinUrlAndPath(tc.Url, tc.Path)
		if out != tc.Output {
			t.Errorf("joined %q and %q to %q instead of %q", tc.Url, tc.Path, out, tc.Output)
		}
	}
}
