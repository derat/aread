// Copyright 2020 Daniel Erat.
// All rights reserved.

package main

import "testing"

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
			t.Errorf("joinURLAndPath(%q, %q) = %q; want %q", tc.url, tc.path, out, tc.output)
		}
	}
}
