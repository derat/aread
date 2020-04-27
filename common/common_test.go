// Copyright 2020 Daniel Erat.
// All rights reserved.

package common

import (
	"testing"
)

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
		expected := SHA1String(tc.url) + tc.expectedSuffix
		actual := LocalImageFilename(tc.url)
		if actual != expected {
			t.Errorf("LocalImageFilename(%q) = %q; want %q", tc.url, actual, expected)
		}
	}
}
