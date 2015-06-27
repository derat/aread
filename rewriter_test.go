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
	inputUrl       = "http://www.example.com/test.html"
	hiddenTagsPath = "testdata/hidden_tags.json"
	outputPath     = "testdata/output.html"
)

func TestBasic(t *testing.T) {
	cfg := Config{
		HiddenTagsFile: hiddenTagsPath,
		Logger:         log.New(os.Stderr, "", log.LstdFlags),
		DownloadImages: true,
	}
	rw := Rewriter{cfg}

	input, err := ioutil.ReadFile(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	output, imageUrls, err := rw.RewriteContent(string(input), inputUrl)

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
	if len(imageUrls) != 1 {
		t.Errorf("got %v image(s) instead of 1", len(imageUrls))
	}
	// TODO: Actually check the image mapping.
}
