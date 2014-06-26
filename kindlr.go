package main

import (
	"code.google.com/p/go.net/html"
	"crypto/sha1"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	ContentFile           = "index.html"
	DefaultImageExtension = ".jpg"
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
	token                string
	ShouldDownloadImages bool
}

func NewContentFetcher(token string) *contentFetcher {
	f := contentFetcher{}
	f.token = token
	return &f
}

func (f *contentFetcher) processContent(input string) (content string, imageUrls map[string]string, err error) {
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

func (f *contentFetcher) downloadImages(urls map[string]string, dir string) error {
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
			return fmt.Errorf("Unable to open %v for image %v: %v\n", path, url, err)
			continue
		}
		defer file.Close()

		if _, err = io.Copy(file, body); err != nil {
			log.Printf("Unable to write image %v to %v: %v\n", url, path, err)
		}
	}

	return nil
}

func (f *contentFetcher) GetContent(contentUrl, destDir string) error {
	url := fmt.Sprintf("https://www.readability.com/api/content/v1/parser?url=%s&token=%s", url.QueryEscape(contentUrl), f.token)
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

	title, err := getStringValue(&o, "title")
	if err != nil {
		return fmt.Errorf("Unable to get title from %v: %v", url, err)
	}
	content, err := getStringValue(&o, "content")
	if err != nil {
		return fmt.Errorf("Unable to get content from %v: %v", url, err)
	}

	content, imageUrls, err := f.processContent(content)
	if err != nil {
		return fmt.Errorf("Unable to process content: %v", err)
	}

	if err = os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("Unable to create %v: %v", destDir, err)
	}

	contentPath := filepath.Join(destDir, ContentFile)
	contentFile, err := os.Create(contentPath)
	if err != nil {
		return err
	}
	defer contentFile.Close()
	template := `
<!DOCTYPE html>
<html>
  <head>
	<meta charset="utf-8">
    <title>%s</title>
  </head>
  <body>
    %s
  </body>
</html>
`
	if _, err := contentFile.WriteString(fmt.Sprintf(template, html.EscapeString(title), content)); err != nil {
		return fmt.Errorf("Failed to write content to %v: %v", contentPath, err)
	}
	if f.ShouldDownloadImages {
		if err = f.downloadImages(imageUrls, destDir); err != nil {
			return fmt.Errorf("Unable to download images: %v", err)
		}
	}
	return nil
}

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
