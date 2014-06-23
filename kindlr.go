package main

import (
	"code.google.com/p/go.net/html"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
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

type contentFetcher struct {
	token string
}

func newContentFetcher(token string) *contentFetcher {
	f := contentFetcher{}
	f.token = token
	return &f
}

func (f *contentFetcher) processContent(data string) (string, error) {
	var s string
	z := html.NewTokenizer(strings.NewReader(data))
	for {
		if z.Next() == html.ErrorToken {
			if z.Err() == io.EOF {
				return s, nil
			}
			return "", z.Err()
		}
		t := z.Token()
		if t.Type == html.StartTagToken && t.Data == "img" {
			for i := range t.Attr {
				if t.Attr[i].Key == "src" {
					t.Attr[i].Val = "foo"
				}
			}
		}
		s += t.String()
	}
}

func (f *contentFetcher) getContent(contentUrl string) (string, error) {
	url := fmt.Sprintf("https://www.readability.com/api/content/v1/parser?url=%s&token=%s", url.QueryEscape(contentUrl), f.token)
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("Fetching %s failed: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Fetching %s returned %d", url, resp.StatusCode)
	}
	var b []byte
	if b, err = ioutil.ReadAll(resp.Body); err != nil {
		return "", fmt.Errorf("Unable to read %s: %v", url, err)
	}
	o := make(map[string]interface{})
	if err = json.Unmarshal(b, &o); err != nil {
		return "", fmt.Errorf("Unable to unmarshal JSON from %v: %v", url, err)
	}

	title, err := getStringValue(&o, "title")
	if err != nil {
		return "", fmt.Errorf("Unable to get title from %v: %v", url, err)
	}
	content, err := getStringValue(&o, "content")
	if err != nil {
		return "", fmt.Errorf("Unable to get content from %v: %v", url, err)
	}
	content, err = f.processContent(content)
	if err != nil {
		return "", fmt.Errorf("Unable to process content: %v", err)
	}

	return fmt.Sprintf("<html><head><title>%s</title></head><body>%s</body></html>", html.EscapeString(title), content), nil
}

func main() {
	var token string
	flag.StringVar(&token, "token", "", "Readability.com Parser API token")
	flag.Parse()

	if len(flag.Args()) != 1 {
		log.Fatalln("Exactly one URL must be passed on command line")
	}

	cf := newContentFetcher(token)
	c, err := cf.getContent(flag.Args()[0])
	if err != nil {
		log.Fatalf("Unable to get content: %v\n", err)
	}
	fmt.Println(c)
}
