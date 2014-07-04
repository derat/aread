package main

import (
	"bytes"
	"code.google.com/p/go.net/html"
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
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	defaultImageExtension = ".jpg"
	maxLineLength         = 80
	indexFile             = "index.html"
	docFile               = "out.mobi"
)

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

type Processor struct {
	ApiToken       string
	MailServer     string
	Sender         string
	Recipient      string
	BaseOutputDir  string
	BaseUrlPath    string
	DownloadImages bool
	Logger         *log.Logger
}

func NewProcessor() *Processor {
	return &Processor{
		BaseOutputDir:  "/tmp",
		DownloadImages: true,
		Logger:         log.New(os.Stderr, "", log.LstdFlags),
	}
}

func (p *Processor) rewriteContent(input string) (content string, imageUrls map[string]string, err error) {
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
		if p.DownloadImages && t.Type == html.StartTagToken && t.Data == "img" {
			for i := range t.Attr {
				if t.Attr[i].Key == "src" {
					url := t.Attr[i].Val
					ext := filepath.Ext(url)
					if len(ext) == 0 {
						ext = defaultImageExtension
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

func (p *Processor) downloadImages(urls map[string]string, dir string) (totalBytes int64, err error) {
	for filename, url := range urls {
		body, err := openUrl(url)
		if err != nil {
			p.Logger.Printf("Failed to download image %v: %v\n", url, err)
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
			p.Logger.Printf("Unable to write image %v to %v: %v\n", url, path, err)
		}
		totalBytes += numBytes
	}
	return totalBytes, nil
}

func (p *Processor) downloadContent(contentUrl, dir string) (title string, err error) {
	apiUrl := fmt.Sprintf("https://www.readability.com/api/content/v1/parser?url=%s&token=%s", url.QueryEscape(contentUrl), p.ApiToken)
	body, err := openUrl(apiUrl)
	if err != nil {
		return title, err
	}
	defer body.Close()
	var b []byte
	if b, err = ioutil.ReadAll(body); err != nil {
		return title, fmt.Errorf("Unable to read %s: %v", apiUrl, err)
	}
	o := make(map[string]interface{})
	if err = json.Unmarshal(b, &o); err != nil {
		return title, fmt.Errorf("Unable to unmarshal JSON from %v: %v", apiUrl, err)
	}

	type templateData struct {
		Content        template.HTML
		Url            string
		Host           string
		Title          string
		Author         string
		PubDate        string
		StylesheetPath string
		ListPath       string
	}
	d := &templateData{
		Url:            contentUrl,
		Host:           getHost(contentUrl),
		StylesheetPath: path.Join(p.BaseUrlPath, staticUrlPath, cssFile),
		ListPath:       p.BaseUrlPath,
	}

	content, err := getStringValue(&o, "content")
	if err != nil {
		return title, fmt.Errorf("Unable to get content from %v: %v", apiUrl, err)
	}

	title, _ = getStringValue(&o, "title")
	if len(title) == 0 {
		title = contentUrl
	}
	d.Title = title
	d.Author, _ = getStringValue(&o, "author")
	d.PubDate, _ = getStringValue(&o, "date_published")

	var imageUrls map[string]string
	content, imageUrls, err = p.rewriteContent(content)
	if err != nil {
		return title, fmt.Errorf("Unable to process content: %v", err)
	}
	d.Content = template.HTML(content)

	contentFile, err := os.Create(filepath.Join(dir, "index.html"))
	if err != nil {
		return title, err
	}
	defer contentFile.Close()

	tmpl, err := template.New("doc").Parse(`
<!DOCTYPE html>
<html>
  <head>
    <meta content="text/html; charset=utf-8" http-equiv="Content-Type"/>
    {{if .Author}}<meta content="{{.Author}}" name="author"/>{{end}}
    <title>{{.Title}}</title>
	<link href="{{.StylesheetPath}}" rel="stylesheet" type="text/css"/>
  </head>
  <body>
    <h3>{{.Title}}</h3>
    <p>
      <a href="{{.Url}}">{{.Host}}</a><br>
      {{if .Author}}<b>By {{.Author}}</b><br>{{end}}
      {{if .PubDate}}<em>Published {{.PubDate}}</em>{{end}}
    </p>
    {{.Content}}
    <a href="{{.ListPath}}">Back to reading list</a>
  </body>
</html>`)

	if err = tmpl.Execute(contentFile, d); err != nil {
		return title, fmt.Errorf("Failed to execute template: %v", err)
	}

	if p.DownloadImages && len(imageUrls) > 0 {
		totalBytes, err := p.downloadImages(imageUrls, dir)
		if err != nil {
			return title, fmt.Errorf("Unable to download images: %v", err)
		}
		p.Logger.Printf("Downloaded %v image(s) totalling %v byte(s)\n", len(imageUrls), totalBytes)
	}

	return title, nil
}

func (p *Processor) buildDoc(dir string) error {
	c := exec.Command("docker", "run", "-v", dir+":/source", "jagregory/kindlegen", indexFile, "-o", docFile)
	o, err := c.CombinedOutput()
	p.Logger.Printf("kindlegen output:%s", strings.Replace("\n"+string(o), "\n", "\n  ", -1))
	if err != nil {
		// kindlegen returns 1 for warnings and 2 for fatal errors.
		if status, ok := err.(*exec.ExitError); !ok || status.Sys().(syscall.WaitStatus).ExitStatus() != 1 {
			return fmt.Errorf("Failed to build doc: %v", err)
		}
	}
	return nil
}

// Based on https://gist.github.com/rmulley/6603544.
func (p *Processor) sendMail(docPath string) error {
	data, err := ioutil.ReadFile(docPath)
	if err != nil {
		return err
	}
	encoded := base64.StdEncoding.EncodeToString(data)

	var buf bytes.Buffer
	numLines := len(encoded) / maxLineLength
	for i := 0; i < numLines; i++ {
		buf.WriteString(encoded[i*maxLineLength:(i+1)*maxLineLength] + "\n")
	}
	buf.WriteString(encoded[numLines*maxLineLength:])

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
			"%s\r\n", p.Sender, p.Recipient, filepath.Base(docPath), buf.String())
	p.Logger.Printf("Sending %v-byte message to %v\n", len(body), p.Recipient)

	c, err := smtp.Dial(p.MailServer)
	if err != nil {
		return err
	}
	c.Mail(p.Sender)
	c.Rcpt(p.Recipient)
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

func (p *Processor) ProcessUrl(contentUrl string, sendToKindle bool) (pi PageInfo, err error) {
	pi.OriginalUrl = contentUrl
	pi.TimeAdded = time.Now().Unix()

	id := getSha1String(contentUrl)
	outDir := filepath.Join(p.BaseOutputDir, id)
	p.Logger.Printf("Processing %v in %v\n", contentUrl, outDir)

	if _, err = os.Stat(outDir); err == nil {
		p.Logger.Printf("Deleting existing %v directory\n", outDir)
		if err = os.RemoveAll(outDir); err != nil {
			return pi, err
		}
	}

	if err = os.MkdirAll(outDir, 0755); err != nil {
		return pi, err
	}
	if pi.Title, err = p.downloadContent(contentUrl, outDir); err != nil {
		return pi, err
	}

	// Okay, we got the page.
	pi.Id = id

	if sendToKindle {
		if err = p.buildDoc(outDir); err != nil {
			return pi, err
		}
		// Leave the .mobi file lying around if we're not sending email.
		if len(p.Recipient) == 0 || len(p.Sender) == 0 {
			p.Logger.Println("Empty recipient or sender; not sending email")
			return pi, nil
		}
		docPath := filepath.Join(outDir, docFile)
		if err = p.sendMail(docPath); err != nil {
			return pi, fmt.Errorf("Unable to send mail: %v\n", err)
		}
		if err = os.Remove(docPath); err != nil {
			return pi, err
		}
	}

	return pi, nil
}
