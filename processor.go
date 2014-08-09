package main

import (
	"bytes"
	"code.google.com/p/go.net/html"
	"code.google.com/p/graphics-go/graphics"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

func getLocalImageFilename(url string) string {
	// kindlegen seems to be confused by image files without extensions.
	ext := filepath.Ext(strings.Split(url, "?")[0])
	if len(ext) == 0 {
		ext = defaultImageExtension
	}
	return getSha1String(url) + ext
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
		isStart := t.Type == html.StartTagToken
		isEnd := t.Type == html.EndTagToken

		if p.cfg.DownloadImages && isStart && t.Data == "img" {
			hasSrc := false
			for i := range t.Attr {
				if t.Attr[i].Key == "src" && len(t.Attr[i].Val) > 0 {
					url := t.Attr[i].Val
					filename := getLocalImageFilename(url)
					imageUrls[filename] = url
					t.Attr[i].Val = filename
					hasSrc = true
					break
				}
			}
			if !hasSrc {
				// kindlegen barfs on empty <img> tags. One appears in
				// http://online.wsj.com/articles/google-to-collect-data-to-define-healthy-human-1406246214.
				continue
			}
		} else if (isStart || isEnd) && t.Data == "h1" {
			// Downgrade <h1> to <h2>.
			t.Data = "h2"
		} else if isStart && t.Data == "iframe" {
			// Readability puts YouTube videos into iframes but kindlegen doesn't know what to do with them.
			continue
		} else if (isStart || isEnd) && t.Data == "noscript" {
			// Keep kindlegen from complaining about <noscript>, and also tell the tokenizer to interpret nested
			// elements. This handles the non-JS tags for lazily-loaded images on theverge.com.
			z.NextIsNotRawText()
			continue
		} else if (isStart || isEnd) && t.Data == "body" {
			// Why does Readability leave body tags within the content sometimes?
			// See e.g. http://kirtimukha.com/surfings/Cogitation/wisdom_of_insecurity_by_alan_wat.htm
			continue
		}
		content += t.String()
	}
}

func (p *Processor) resizeImage(origImg image.Image, imgFmt, filename string) error {
	origWidth := origImg.Bounds().Max.X - origImg.Bounds().Min.X
	origHeight := origImg.Bounds().Max.Y - origImg.Bounds().Min.Y
	if origWidth <= p.cfg.MaxImageWidth && origHeight <= p.cfg.MaxImageHeight {
		return nil
	}

	widthRatio := float64(origWidth) / float64(p.cfg.MaxImageWidth)
	heightRatio := float64(origHeight) / float64(p.cfg.MaxImageHeight)
	var newWidth, newHeight int
	if widthRatio > heightRatio {
		newWidth = p.cfg.MaxImageWidth
		newHeight = int(float64(origHeight)/widthRatio + 0.5)
	} else {
		newWidth = int(float64(origWidth)/heightRatio + 0.5)
		newHeight = p.cfg.MaxImageHeight
	}

	p.cfg.Logger.Printf("Scaling %v from %vx%v to %vx%v\n", filename, origWidth, origHeight, newWidth, newHeight)
	newImg := image.NewRGBA(image.Rectangle{image.Point{0, 0}, image.Point{newWidth, newHeight}})
	if err := graphics.Scale(newImg, origImg); err != nil {
		return err
	}

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	switch imgFmt {
	case "png":
		err = png.Encode(f, newImg)
	case "jpeg":
		err = jpeg.Encode(f, newImg, &jpeg.Options{Quality: p.cfg.JpegQuality})
	default:
		p.cfg.Logger.Fatalf("Unhandled image format %v for %v", imgFmt, filename)
	}
	return err
}

func (p *Processor) processImage(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	origInfo, err := f.Stat()
	if err != nil {
		return err
	}

	img, imgFmt, err := image.Decode(f)
	if err != nil {
		p.cfg.Logger.Printf("Unable to decode %v\n", filename)
	} else {
		if err = p.resizeImage(img, imgFmt, filename); err != nil {
			return err
		}
	}

	newInfo, err := f.Stat()
	if err != nil {
		return err
	}
	if origInfo.Size() != newInfo.Size() {
		p.cfg.Logger.Printf("Resized %v from %v bytes to %v bytes\n", filename, origInfo.Size(), newInfo.Size())
	}
	if newInfo.Size() > p.cfg.MaxImageBytes {
		p.cfg.Logger.Printf("Deleting %v-byte file %v\n", newInfo.Size(), filename)
		if err = os.Remove(filename); err != nil {
			return err
		}
	}
	return nil
}

func (p *Processor) downloadImages(urls map[string]string, dir string) (totalBytes int64) {
	c := make(chan int64)
	for filename, url := range urls {
		go func(filename, url string) {
			var bytes int64 = 0
			defer func() { c <- bytes }()

			body, err := openUrl(url)
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
			if err = p.processImage(path); err != nil {
				p.cfg.Logger.Printf("Unable to process image %v: %v\n", path, err)
			}
		}(filename, url)
	}

	for i := 0; i < len(urls); i++ {
		totalBytes += <-c
	}
	close(c)
	return totalBytes
}

func (p *Processor) downloadContent(pi PageInfo, dir string) (title string, err error) {
	apiUrl := fmt.Sprintf("https://www.readability.com/api/content/v1/parser?url=%s&token=%s", url.QueryEscape(pi.OriginalUrl), p.cfg.ApiToken)
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

	queryParams := fmt.Sprintf("?%s=%s&%s=%s&%s=%s", idParam, pi.Id, tokenParam, pi.Token, redirectParam, url.QueryEscape(p.cfg.GetPath()))
	d := struct {
		Content     template.HTML
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
	content, imageUrls, err = p.rewriteContent(content)
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

	contentFile, err := os.Create(filepath.Join(dir, "index.html"))
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
    <span id="top-links">
      <a href="#end-paragraph">Jump to bottom</a> -
      <a href="{{.KindlePath}}">Send to Kindle</a>
    </span>
    <div class="content">
      {{.Content}}
    </div>
    <p id="end-paragraph">
      <a href="{{.ArchivePath}}">Toggle archived</a> -
      <a href="#title-header">Jump to top</a> -
      <a href="{{.ListPath}}">Back to list</a>
    </p>
  </body>
</html>`
	if err := writeTemplate(contentFile, p.cfg, t, d, template.FuncMap{}); err != nil {
		return title, fmt.Errorf("Failed to execute page template: %v", err)
	}

	return title, nil
}

func (p *Processor) buildDoc(dir string) error {
	c := exec.Command("docker", "run", "-v", dir+":/source", "jagregory/kindlegen", indexFile, "-o", docFile)
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
			"%s\r\n", p.cfg.Sender, p.cfg.Recipient, filepath.Base(docPath), buf.String())
	p.cfg.Logger.Printf("Sending %v-byte message to %v\n", len(body), p.cfg.Recipient)

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
	if _, err = w.Write([]byte(body)); err != nil {
		return err
	}
	return nil
}

func (p *Processor) ProcessUrl(contentUrl string) (pi PageInfo, err error) {
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
