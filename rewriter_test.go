package main

import (
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"testing"
)

const (
	inputPath      = "testdata/input.html"
	inputURL       = "http://www.example.com/test.html"
	hiddenTagsPath = "testdata/hidden_tags.json"
	outputPath     = "testdata/output.html"
)

var expectedImages []string = []string{
	"http://www.example.com/img.png",
	"http://assets.bwbx.io/images/i6vlZjCDxVKs/v1/488x-1.jpg",
	"http://cdn.arstechnica.net/wp-content/uploads/2016/01/Screen-Shot-2016-01-30-at-11.30.32-PM-1280x562.png",
	"http://a.com/drop-srcset.png",
}

func TestBasic(t *testing.T) {
	rw := rewriter{config{
		HiddenTagsFile: hiddenTagsPath,
		Logger:         log.New(os.Stderr, "", log.LstdFlags),
		DownloadImages: true,
	}}

	input, err := ioutil.ReadFile(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	output, imageURLs, err := rw.RewriteContent(string(input), inputURL)

	// Whitespace is a pain. Ignore empty lines.
	emptyLineRegexp := regexp.MustCompile("\n\\s*\n")
	output = emptyLineRegexp.ReplaceAllLiteralString(output, "\n")

	expectedOutput, err := ioutil.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if output != string(expectedOutput) {
		t.Errorf("actual output differed from expected output\n\nexpected:\n-----\n%v\n-----\nactual:\n-----\n%v\n-----\n", string(expectedOutput), output)
	}

	if len(imageURLs) != len(expectedImages) {
		t.Errorf("got %v image(s) instead of %v", len(imageURLs), len(expectedImages))
	}
	for _, ei := range expectedImages {
		fn := getLocalImageFilename(ei)
		u, ok := imageURLs[fn]
		if !ok {
			t.Errorf("missing file %v for image %v", fn, ei)
		} else if u != ei {
			t.Errorf("file %v maps to %v, expected %v", fn, u, ei)
		}
	}
}
