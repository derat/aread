package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	f := NewContentFetcher()

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [option]... <url>\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.StringVar(&f.BaseTempDir, "temp-dir", "/tmp", "Base temp directory")
	flag.BoolVar(&f.ShouldDownloadImages, "download-images", true, "Download and write local copies of images")
	flag.StringVar(&f.MailServer, "mail-server", "localhost:25", "SMTP server host:port")
	flag.StringVar(&f.Recipient, "recipient", "", "Recipient email address")
	flag.StringVar(&f.Sender, "sender", "", "Sender email address")
	flag.StringVar(&f.ApiToken, "token", "", "Readability.com Parser API token")
	flag.Parse()

	if len(flag.Args()) != 1 {
		log.Fatalln("One URL must be supplied")
	}
	if err := f.ProcessUrl(flag.Args()[0]); err != nil {
		log.Fatalln(err)
	}
}
