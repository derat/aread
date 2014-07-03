package main

import (
	"bytes"
	"code.google.com/p/go.net/html"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	DefaultImageExtension = ".jpg"
	MaxLineLength         = 80
	IndexFile             = "index.html"
	DocFile               = "out.mobi"
)

func getSha1String(input string) string {
	h := sha1.New()
	io.WriteString(h, input)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func getStringValue(object *map[string]interface{}, name string) (string, error) {
	data, ok := (*object)[name]
	if !ok {
		return "", fmt.Errorf("No property \"%v\" in object", name)
	}
	s, ok := data.(string)
	if !ok {
		return "", fmt.Errorf("Property \"%v\" is not a string", name)
	}
	return s, nil
}

func openUrl(url string) (io.ReadCloser, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("Fetching %s failed: %v", url, err)
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, fmt.Errorf("Fetching %s returned %d", url, resp.StatusCode)
	}
	return resp.Body, nil
}

type contentFetcher struct {
	ApiToken             string
	MailServer           string
	Sender               string
	Recipient            string
	BaseTempDir          string
	ShouldDownloadImages bool
}

func NewContentFetcher() *contentFetcher {
	f := contentFetcher{}
	return &f
}

func (f *contentFetcher) rewriteContent(input string) (content string, imageUrls map[string]string, err error) {
	imageUrls = make(map[string]string)
	z := html.NewTokenizer(strings.NewReader(input))
	for {
		if z.Next() == html.ErrorToken {
			if z.Err() == io.EOF {
				return content, imageUrls, nil
			}
			return "", nil, z.Err()
		}
		t := z.Token()
		if f.ShouldDownloadImages && t.Type == html.StartTagToken && t.Data == "img" {
			for i := range t.Attr {
				if t.Attr[i].Key == "src" {
					url := t.Attr[i].Val
					ext := filepath.Ext(url)
					if len(ext) == 0 {
						ext = DefaultImageExtension
					}
					name := getSha1String(url) + ext
					imageUrls[name] = url
					t.Attr[i].Val = name
				}
			}
		} else if t.Type == html.StartTagToken && t.Data == "iframe" {
			// Readability puts YouTube videos into iframes but kindlegen doesn't know what to do with them.
			continue
		}
		content += t.String()
	}
}

func (f *contentFetcher) downloadImages(urls map[string]string, dir string) (totalBytes int64, err error) {
	for filename, url := range urls {
		body, err := openUrl(url)
		if err != nil {
			log.Printf("Failed to download image %v: %v\n", url, err)
			continue
		}
		defer body.Close()

		path := filepath.Join(dir, filename)
		file, err := os.Create(path)
		if err != nil {
			return totalBytes, fmt.Errorf("Unable to open %v for image %v: %v\n", path, url, err)
		}
		defer file.Close()

		numBytes, err := io.Copy(file, body)
		if err != nil {
			log.Printf("Unable to write image %v to %v: %v\n", url, path, err)
		}
		totalBytes += numBytes
	}
	return totalBytes, nil
}

func (f *contentFetcher) downloadContent(contentUrl, dir string) error {
	url := fmt.Sprintf("https://www.readability.com/api/content/v1/parser?url=%s&token=%s", url.QueryEscape(contentUrl), f.ApiToken)
	body, err := openUrl(url)
	if err != nil {
		return err
	}
	defer body.Close()
	var b []byte
	if b, err = ioutil.ReadAll(body); err != nil {
		return fmt.Errorf("Unable to read %s: %v", url, err)
	}
	o := make(map[string]interface{})
	if err = json.Unmarshal(b, &o); err != nil {
		return fmt.Errorf("Unable to unmarshal JSON from %v: %v", url, err)
	}

	type templateData struct {
		Content template.HTML
		Title   string
		Author  string
		PubDate string
	}
	d := &templateData{}

	content, err := getStringValue(&o, "content")
	if err != nil {
		return fmt.Errorf("Unable to get content from %v: %v", url, err)
	}

	d.Title, _ = getStringValue(&o, "title")
	if len(d.Title) == 0 {
		d.Title = contentUrl
	}
	d.Author, _ = getStringValue(&o, "author")
	d.PubDate, _ = getStringValue(&o, "date_published")

	var imageUrls map[string]string
	content, imageUrls, err = f.rewriteContent(content)
	if err != nil {
		return fmt.Errorf("Unable to process content: %v", err)
	}
	d.Content = template.HTML(content)

	contentFile, err := os.Create(filepath.Join(dir, "index.html"))
	if err != nil {
		return err
	}
	defer contentFile.Close()

	tmpl, err := template.New("doc").Parse(`
<html>
  <head>
    <meta content="text/html; charset=utf-8" http-equiv="Content-Type"/>
    {{if .Author}}<meta content="{{.Author}}" name="author"/>{{end}}
    <title>{{.Title}}</title>
  </head>
  <body>
    <h2>{{.Title}}</h2>
    <p>
      {{if .Author}}<b>By {{.Author}}</b><br>{{end}}
      {{if .PubDate}}<em>Published {{.PubDate}}</em>{{end}}
    </p>
    {{.Content}}
  </body>
</html>`)

	if err = tmpl.Execute(contentFile, d); err != nil {
		return fmt.Errorf("Failed to execute template: %v", err)
	}

	if f.ShouldDownloadImages && len(imageUrls) > 0 {
		totalBytes, err := f.downloadImages(imageUrls, dir)
		if err != nil {
			return fmt.Errorf("Unable to download images: %v", err)
		}
		log.Printf("Downloaded %v image(s) totalling %v byte(s)\n", len(imageUrls), totalBytes)
	}

	return nil
}

func (f *contentFetcher) buildDoc(dir string) error {
	c := exec.Command("docker", "run", "-v", dir+":/source", "jagregory/kindlegen", IndexFile, "-o", DocFile)
	o, err := c.CombinedOutput()
	log.Printf("kindlegen output:%s", strings.Replace("\n"+string(o), "\n", "\n  ", -1))
	if err != nil {
		// kindlegen returns 1 for warnings and 2 for fatal errors.
		if status, ok := err.(*exec.ExitError); !ok || status.Sys().(syscall.WaitStatus).ExitStatus() != 1 {
			return fmt.Errorf("Failed to build doc: %v", err)
		}
	}
	return nil
}

// Based on https://gist.github.com/rmulley/6603544.
func (f *contentFetcher) sendMail(docPath string) error {
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
			"%s\r\n", f.Sender, f.Recipient, filepath.Base(docPath), buf.String())
	log.Printf("Sending %v-byte message to %v\n", len(body), f.Recipient)

	c, err := smtp.Dial(f.MailServer)
	if err != nil {
		return err
	}
	c.Mail(f.Sender)
	c.Rcpt(f.Recipient)
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

func (f *contentFetcher) ProcessUrl(contentUrl string) error {
	tempDir, err := ioutil.TempDir(f.BaseTempDir, "kindlr.")
	if err != nil {
		return err
	}
	log.Printf("Processing %v in %v\n", contentUrl, tempDir)
	if err = f.downloadContent(contentUrl, tempDir); err != nil {
		return err
	}
	if err = f.buildDoc(tempDir); err != nil {
		return err
	}

	if len(f.Recipient) == 0 || len(f.Sender) == 0 {
		log.Println("Empty recipient or sender; not sending email")
	} else if err = f.sendMail(filepath.Join(tempDir, DocFile)); err != nil {
		return fmt.Errorf("Unable to send mail: %v\n", err)
	}
	return nil
}
