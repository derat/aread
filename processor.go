package main

import (
	"bytes"
	// Handle Comodo certs: http://bridge.grumpy-troll.org/2014/05/golang-tls-comodo/
	_ "crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/smtp"
	"net/textproto"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"
)

const (
	maxLineLength    = 80
	indexFile        = "index.html"
	kindleFile       = "kindle.html"
	docFile          = "out.mobi"
	maxPageRetries   = 1
	httpRetryDelayMs = 1000
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

func getFaviconUrl(origUrl string) (string, error) {
	u, err := url.Parse(origUrl)
	if err != nil {
		return "", err
	}
	u.Path = "/favicon.ico"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

type Processor struct {
	cfg Config
}

func (p *Processor) rewriteUrl(origUrl string) (newUrl string, err error) {
	if len(p.cfg.UrlPatternsFile) == 0 {
		return origUrl, nil
	}

	pats := make([][]string, 0)
	if err = readJsonFile(p.cfg.UrlPatternsFile, &pats); err != nil {
		return
	}
	newUrl = origUrl
	for i, entry := range pats {
		if len(entry) != 2 {
			return "", fmt.Errorf("Entry %v had %v element(s); should be [regexp, repl]", i, len(entry))
		}
		re, err := regexp.Compile(entry[0])
		if err != nil {
			return "", fmt.Errorf("Failed to compile regexp %q: %v", entry[0], err)
		}
		newUrl = re.ReplaceAllString(newUrl, entry[1])
	}
	if newUrl != origUrl {
		p.cfg.Logger.Printf("Rewrote %v to %v\n", origUrl, newUrl)
	}
	return
}

func (p *Processor) openUrl(url string, maxRetries int) (io.ReadCloser, error) {
	for i := 0; ; i++ {
		var transientError bool
		resp, err := http.Get(url)
		if err != nil {
			transientError = true
		} else if resp.StatusCode != 200 {
			resp.Body.Close()
			err = fmt.Errorf("Received status code %d", resp.StatusCode)
			transientError = resp.StatusCode >= 500 && resp.StatusCode < 600
		} else {
			return resp.Body, nil
		}

		if transientError && i < maxRetries {
			p.cfg.Logger.Printf("Got transient error for %v: %v\n", url, err)
			time.Sleep(time.Duration(httpRetryDelayMs) * time.Millisecond)
		} else {
			return nil, fmt.Errorf("Unable to get %v: %v", url, err)
		}
	}
}

func (p *Processor) downloadImages(urls map[string]string, dir string) (totalBytes int64) {
	ic := newImageCleaner(p.cfg)
	c := make(chan int64)
	for filename, url := range urls {
		go func(filename, url string) {
			var bytes int64 = 0
			defer func() { c <- bytes }()

			body, err := p.openUrl(url, 0)
			if err != nil {
				p.cfg.Logger.Printf("Failed to download image %v: %v\n", url, err)
				return
			}
			defer body.Close()

			path := filepath.Join(dir, filename)
			file, err := os.Create(path)
			if err != nil {
				p.cfg.Logger.Printf("Unable to open %v for image %v: %v\n", path, url, err)
				return
			}
			defer file.Close()

			bytes, err = io.Copy(file, body)
			if err != nil {
				p.cfg.Logger.Printf("Unable to write image %v to %v: %v\n", url, path, err)
				return
			}
			if err = ic.ProcessImage(path); err != nil {
				p.cfg.Logger.Printf("Unable to process image %v: %v\n", path, err)
			}
		}(filename, url)
	}

	for i := 0; i < len(urls); i++ {
		totalBytes += <-c
	}
	close(c)
	runtime.GC()
	return totalBytes
}

func (p *Processor) checkContent(pi PageInfo, content string) error {
	if len(p.cfg.BadContentFile) == 0 {
		return nil
	}

	pats := make([][]string, 0)
	if err := readJsonFile(p.cfg.BadContentFile, &pats); err != nil {
		return err
	}
	for i, entry := range pats {
		if len(entry) != 2 {
			return fmt.Errorf("Entry %v had %v element(s); should be [url_regexp, content_regexp]", i, len(entry))
		}
		urlRegexp, err := regexp.Compile(entry[0])
		if err != nil {
			return fmt.Errorf("Failed to compile URL regexp %q: %v", entry[0], err)
		}
		contentRegexp, err := regexp.Compile(entry[1])
		if err != nil {
			return fmt.Errorf("Failed to compile content regexp %q: %v", entry[1], err)
		}
		if urlRegexp.MatchString(pi.OriginalUrl) && contentRegexp.MatchString(content) {
			return fmt.Errorf("Matched %q", entry[1])
		}
	}
	return nil
}

func (p *Processor) downloadContent(pi PageInfo, dir string) (title string, err error) {
	apiUrl := fmt.Sprintf("https://www.readability.com/api/content/v1/parser?url=%s&token=%s", url.QueryEscape(pi.OriginalUrl), p.cfg.ApiToken)
	body, err := p.openUrl(apiUrl, maxPageRetries)
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

	queryParams := fmt.Sprintf("?%s=%s&%s=%s&%s=%s", idParam, pi.Id, tokenParam, pi.Token, redirectParam, url.QueryEscape(p.cfg.GetPath()))
	d := struct {
		Content     template.HTML
		ForWeb      bool
		Url         string
		Host        string
		Title       string
		Author      string
		PubDate     string
		ArchivePath string
		KindlePath  string
		ListPath    string
	}{
		Url:         pi.OriginalUrl,
		Host:        getHost(pi.OriginalUrl),
		ArchivePath: p.cfg.GetPath(archiveUrlPath + queryParams),
		KindlePath:  p.cfg.GetPath(kindleUrlPath + queryParams),
		ListPath:    p.cfg.GetPath(),
	}

	content, err := getStringValue(&o, "content")
	if err != nil {
		return title, fmt.Errorf("Unable to get content from %v: %v", apiUrl, err)
	}
	if err = p.checkContent(pi, content); err != nil {
		return title, fmt.Errorf("Bad content: %v", err)
	}

	title, _ = getStringValue(&o, "title")
	if len(title) == 0 {
		title = pi.OriginalUrl
	}
	d.Title = title
	d.Author, _ = getStringValue(&o, "author")

	rawDate, _ := getStringValue(&o, "date_published")
	if len(rawDate) > 0 {
		if parsedDate, err := time.Parse("2006-01-02 15:04:05", rawDate); err == nil {
			d.PubDate = parsedDate.Format("Monday, January 2, 2006")
		}
	}

	// filename -> URL
	var imageUrls map[string]string
	rewriter := Rewriter{p.cfg}
	content, imageUrls, err = rewriter.RewriteContent(content, pi.OriginalUrl)
	if err != nil {
		return title, fmt.Errorf("Unable to process content: %v", err)
	}
	d.Content = template.HTML(content)

	var faviconFilename string
	if p.cfg.DownloadFavicons {
		if faviconUrl, err := getFaviconUrl(pi.OriginalUrl); err != nil {
			p.cfg.Logger.Printf("Unable to generate favicon URL for %v: %v", pi.OriginalUrl, err)
		} else {
			faviconFilename = getLocalImageFilename(faviconUrl)
			imageUrls[faviconFilename] = faviconUrl
		}
	}

	if p.cfg.DownloadImages && len(imageUrls) > 0 {
		totalBytes := p.downloadImages(imageUrls, dir)
		p.cfg.Logger.Printf("Downloaded %v image(s) totalling %v byte(s)\n", len(imageUrls), totalBytes)
	}
	if len(faviconFilename) > 0 {
		if _, err := os.Stat(filepath.Join(dir, faviconFilename)); err != nil {
			faviconFilename = ""
		}
	}

	cssFiles := []string{commonCssFile, pageCssFile}
	for _, file := range cssFiles {
		if err = copyFile(filepath.Join(dir, file), filepath.Join(p.cfg.StaticDir, file)); err != nil {
			return title, err
		}
	}

	for _, filename := range []string{indexFile, kindleFile} {
		contentFile, err := os.Create(filepath.Join(dir, filename))
		if err != nil {
			return title, err
		}
		defer contentFile.Close()

		writeHeader(contentFile, p.cfg, cssFiles, title, faviconFilename, d.Author)
		t := `
  <body>
    <h1 id="title-header">{{.Title}}</h1>
    <a href="{{.Url}}">{{.Host}}</a><br/>
    {{if .Author}}<b>By {{.Author}}</b><br/>{{end}}
    {{if .PubDate}}<em>Published {{.PubDate}}</em><br/>{{end}}
	{{if .ForWeb}}<span id="top-links">
      <a href="#end-paragraph">Jump to bottom</a> -
      <a href="{{.KindlePath}}">Send to Kindle</a>
    </span>{{end}}
    <div class="content">
      {{.Content}}
    </div>
	{{if .ForWeb}}<p id="end-paragraph">
      <a href="{{.ArchivePath}}">Toggle archived</a> -
      <a href="#title-header">Jump to top</a> -
      <a href="{{.ListPath}}">Back to list</a>
    </p>{{end}}
  </body>
</html>`
		d.ForWeb = filename != kindleFile
		if err := writeTemplate(contentFile, p.cfg, t, d, template.FuncMap{}); err != nil {
			return title, fmt.Errorf("Failed to execute page template: %v", err)
		}
	}

	return title, nil
}

func (p *Processor) buildDoc(dir string) error {
	c := exec.Command("docker", "run", "-v", dir+":/source", "jagregory/kindlegen", kindleFile, "-o", docFile)
	o, err := c.CombinedOutput()
	p.cfg.Logger.Printf("kindlegen output:%s", strings.Replace("\n"+string(o), "\n", "\n  ", -1))
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

	c, err := smtp.Dial(p.cfg.MailServer)
	if err != nil {
		return err
	}
	c.Mail(p.cfg.Sender)
	c.Rcpt(p.cfg.Recipient)
	w, err := c.Data()
	if err != nil {
		return err
	}
	defer w.Close()

	mw := multipart.NewWriter(w)
	if _, err = w.Write([]byte(fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: kindle document\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: multipart/mixed; boundary=%s\r\n"+
			"\r\n",
		p.cfg.Sender, p.cfg.Recipient, mw.Boundary()))); err != nil {
		return fmt.Errorf("Failed to write header: %v", err)
	}

	thead := make(textproto.MIMEHeader)
	thead.Add("Content-Type", "text/plain; charset=UTF-8")
	if pw, err := mw.CreatePart(thead); err != nil {
		return fmt.Errorf("Failed to create text part: %v", err)
	} else if _, err = pw.Write([]byte("Nothing to see here.")); err != nil {
		return fmt.Errorf("Failed to write text part: %v", err)
	}

	basename := filepath.Base(docPath)
	ahead := make(textproto.MIMEHeader)
	ahead.Add("Content-Type", "application/x-mobipocket-ebook; name=\""+basename+"\"")
	ahead.Add("Content-Disposition", "attachment; filename=\""+basename+"\"")
	ahead.Add("Content-Transfer-Encoding", "base64")
	ahead.Add("X-Attachment-Id", basename)
	if pw, err := mw.CreatePart(ahead); err != nil {
		return fmt.Errorf("Failed to create attachment part: %v", err)
	} else if _, err = pw.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("Failed to write attachment part: %v", err)
	}

	if err = mw.Close(); err != nil {
		return err
	}

	p.cfg.Logger.Printf("Sent message with %v-byte attachment to %v\n", buf.Len(), p.cfg.Recipient)
	return nil
}

func (p *Processor) ProcessUrl(contentUrl string) (pi PageInfo, err error) {
	if contentUrl, err = p.rewriteUrl(contentUrl); err != nil {
		return pi, fmt.Errorf("Failed rewriting URL: %v", err)
	}

	pi.Id = getSha1String(contentUrl)
	pi.OriginalUrl = contentUrl
	pi.TimeAdded = time.Now().Unix()
	pi.Token = getSha1String(fmt.Sprintf("%s|%s|%s", p.cfg.Username, p.cfg.Password, contentUrl))

	outDir := filepath.Join(p.cfg.PageDir, pi.Id)
	p.cfg.Logger.Printf("Processing %v in %v\n", contentUrl, outDir)

	if _, err = os.Stat(outDir); err == nil {
		p.cfg.Logger.Printf("Deleting existing %v directory\n", outDir)
		if err = os.RemoveAll(outDir); err != nil {
			return pi, err
		}
	}

	if err = os.MkdirAll(outDir, 0755); err != nil {
		return pi, err
	}
	if pi.Title, err = p.downloadContent(pi, outDir); err != nil {
		return pi, err
	}
	return pi, nil
}

func (p *Processor) SendToKindle(id string) error {
	if matched, err := regexp.Match("^[a-f0-9]+$", []byte(id)); err != nil {
		return err
	} else if !matched {
		return fmt.Errorf("Invalid ID")
	}

	outDir := filepath.Join(p.cfg.PageDir, id)
	if _, err := os.Stat(outDir); err != nil {
		return fmt.Errorf("Nonexistent directory")
	}

	if err := p.buildDoc(outDir); err != nil {
		return err
	}
	// Leave the .mobi file lying around if we're not sending email.
	if len(p.cfg.Recipient) == 0 || len(p.cfg.Sender) == 0 {
		p.cfg.Logger.Println("Empty recipient or sender; not sending email")
		return nil
	}
	docPath := filepath.Join(outDir, docFile)
	if err := p.sendMail(docPath); err != nil {
		return fmt.Errorf("Unable to send mail: %v", err)
	}
	if err := os.Remove(docPath); err != nil {
		return err
	}
	return nil
}
