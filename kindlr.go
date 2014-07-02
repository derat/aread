package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	var downloadImages bool
	var token string
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [option] ... <url> <dest-dir>\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.BoolVar(&downloadImages, "download-images", true, "Download and write local copies of images")
	flag.StringVar(&token, "token", "", "Readability.com Parser API token")
	flag.Parse()

	if len(flag.Args()) != 2 {
		log.Fatalln("One URL and dest dir must be passed on command line")
	}

	cf := NewContentFetcher(token)
	cf.ShouldDownloadImages = downloadImages
	if err := cf.GetContent(flag.Args()[0], flag.Args()[1]); err != nil {
		log.Fatalf("Unable to get content: %v\n", err)
	}
}
