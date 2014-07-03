package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/smtp"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

const (
	MaxLineLength = 80
	MimeMarker    = "HEREISTHEMIMEMARKER"
	TempDocFile   = "out.mobi"
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

// Based on https://gist.github.com/rmulley/6603544.
func sendMail(sender, recipient, docPath string) error {
	data, err := ioutil.ReadFile(docPath)
	if err != nil {
		return err
	}
	encoded := base64.StdEncoding.EncodeToString(data)

	var buf bytes.Buffer
	numLines := len(encoded) / MaxLineLength
	for i := 0; i < numLines; i++ {
		buf.WriteString(encoded[i*MaxLineLength:(i+1)*MaxLineLength] + "\n")
	}
	buf.WriteString(encoded[numLines*MaxLineLength:])

	// You so crazy, gofmt.
	body := fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: kindle document\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: application/x-mobipocket-ebook\r\n"+
			"Content-Transfer-Encoding:base64\r\n"+
			"Content-Disposition: attachment; filename=\"%s\";\r\n"+
			"\r\n"+
			"%s\r\n", sender, recipient, filepath.Base(docPath), buf.String())

	c, err := smtp.Dial("localhost:25")
	if err != nil {
		return err
	}
	c.Mail(sender)
	c.Rcpt(recipient)
	w, err := c.Data()
	if err != nil {
		return err
	}
	defer w.Close()
	if _, err = w.Write([]byte(body)); err != nil {
		return err
	}
	return nil
}

func main() {
	var downloadImages bool
	var baseTempDir, recipient, sender, token string
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [option]... <url>\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.StringVar(&baseTempDir, "temp-dir", "/tmp", "Base temp directory")
	flag.BoolVar(&downloadImages, "download-images", true, "Download and write local copies of images")
	flag.StringVar(&recipient, "recipient", "", "Recipient email address")
	flag.StringVar(&sender, "sender", "", "Sender email address")
	flag.StringVar(&token, "token", "", "Readability.com Parser API token")
	flag.Parse()

	if len(flag.Args()) != 1 {
		log.Fatalln("One URL must be supplied")
	}
	if len(recipient) == 0 || len(sender) == 0 {
		log.Fatalln("Missing recipient or sender")
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

	docPath := filepath.Join(tempDir, "doc.mobi")
	if err = buildDoc(contentPath, docPath); err != nil {
		log.Fatalf("Unable to build doc: %v\n", err)
	}
	if err = sendMail(sender, recipient, docPath); err != nil {
		log.Fatalf("Unable to send mail: %v\n", err)
	}
}
