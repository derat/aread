package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

const (
	TempDocFile = "out.mobi"
)

func buildDoc(htmlPath string, destPath string) error {
	inputDir := filepath.Dir(htmlPath)
	c := exec.Command("docker", "run", "-v", inputDir+":/source", "jagregory/kindlegen", filepath.Base(htmlPath), "-o", TempDocFile)
	o, err := c.CombinedOutput()
	log.Printf("kindlegen output: %s", o)
	if err != nil {
		// kindlegen returns 1 for warnings and 2 for fatal errors.
		if status, ok := err.(*exec.ExitError); !ok || status.Sys().(syscall.WaitStatus).ExitStatus() != 1 {
			return fmt.Errorf("Failed to build doc: %v", err)
		}
	}
	srcPath := filepath.Join(inputDir, TempDocFile)
	err = os.Rename(srcPath, destPath)
	if err != nil {
		return fmt.Errorf("Unable to move %v to %v: %v", srcPath, destPath, err)
	}
	return nil
}

func main() {
	var downloadImages bool
	var baseTempDir, token string
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [option] ... <url> <dest-file>\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.StringVar(&baseTempDir, "temp-dir", "/tmp", "Base temp directory")
	flag.BoolVar(&downloadImages, "download-images", true, "Download and write local copies of images")
	flag.StringVar(&token, "token", "", "Readability.com Parser API token")
	flag.Parse()

	if len(flag.Args()) != 2 {
		log.Fatalln("One URL and dest file must be passed on command line")
	}

	tempDir, err := ioutil.TempDir(baseTempDir, "kindlr.")
	if err != nil {
		log.Fatalf("Unable to create temp dir under %v: %v\n", tempDir, err)
	}

	cf := NewContentFetcher(token)
	cf.ShouldDownloadImages = downloadImages
	contentPath := filepath.Join(tempDir, "index.html")
	if err := cf.GetContent(flag.Args()[0], contentPath); err != nil {
		log.Fatalf("Unable to get content: %v\n", err)
	}

	if err = buildDoc(contentPath, flag.Args()[1]); err != nil {
		log.Fatalf("Unable to build doc: %v\n", err)
	}
}
