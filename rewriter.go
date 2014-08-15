package main

import (
	"code.google.com/p/go.net/html"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// element -> class -> true
type hiddenTagsMap map[string]map[string]bool

func getAttrValue(token html.Token, name string) string {
	for i := range token.Attr {
		if token.Attr[i].Key == name {
			return token.Attr[i].Val
		}
	}
	return ""
}

type Rewriter struct {
	cfg Config
}

// readHiddenTagsFile returns a map containing the tags that should be hidden for url.
func (r *Rewriter) readHiddenTagsFile(url string) (*hiddenTagsMap, error) {
	tags := make(hiddenTagsMap)
	if len(r.cfg.HiddenTagsFile) == 0 {
		return &tags, nil
	}

	// host -> [element.class, element.class, ...]
	f, err := os.Open(r.cfg.HiddenTagsFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data := make(map[string][]string)
	d := json.NewDecoder(f)
	if err = d.Decode(&data); err != nil {
		return nil, err
	}

	urlHost := getHost(url)
	for host, entries := range data {
		if host != urlHost && !strings.HasSuffix(urlHost, "."+host) {
			continue
		}

		for _, entry := range entries {
			parts := strings.Split(entry, ".")
			if len(parts) != 2 {
				return nil, fmt.Errorf("Expected element.class in %q", entry)
			}
			if _, ok := tags[parts[0]]; !ok {
				tags[parts[0]] = make(map[string]bool)
			}
			tags[parts[0]][parts[1]] = true
		}
	}
	return &tags, nil
}

func (r *Rewriter) shouldHideToken(t html.Token, tags *hiddenTagsMap) bool {
	if classes, ok := (*tags)[t.Data]; ok {
		for _, c := range strings.Fields(getAttrValue(t, "class")) {
			if _, ok := classes[c]; ok {
				return true
			}
		}
	}
	return false
}

func (r *Rewriter) RewriteContent(input, url string) (content string, imageUrls map[string]string, err error) {
	hiddenTags, err := r.readHiddenTagsFile(url)
	if err != nil {
		return "", nil, err
	}

	imageUrls = make(map[string]string)
	hideDepth := 0

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

		// Check if we're nested within a hidden element.
		if hideDepth > 0 {
			if isEnd {
				hideDepth--
			} else if isStart {
				hideDepth++
			}
			continue
		}

		if r.shouldHideToken(t, hiddenTags) {
			r.cfg.Logger.Printf("Hiding <%v> token with class %q\n", t.Data, getAttrValue(t, "class"))
			if isStart {
				hideDepth = 1
			}
			continue
		}

		if r.cfg.DownloadImages && isStart && t.Data == "img" {
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
		} else if (isStart || isEnd) && (t.Data == "h4" || t.Data == "h5" || t.Data == "h6") {
			// <h6> seems to mainly be used by people who don't know what they're doing. Upgrade <h4>, <h5>, and <h6> to <h3>.
			t.Data = "h3"
		} else if isStart && t.Data == "iframe" {
			// Readability puts YouTube videos into iframes but kindlegen doesn't know what to do with them.
			continue
		} else if (isStart || isEnd) && t.Data == "noscript" {
			// Tell the tokenizer to interpret nested elements. This handles the non-JS tags for lazily-loaded images on theverge.com.
			if isStart {
				z.NextIsNotRawText()
			}
			// Keep kindlegen from complaining about <noscript>.
			continue
		} else if (isStart || isEnd) && t.Data == "body" {
			// Why does Readability leave body tags within the content sometimes?
			// See e.g. http://kirtimukha.com/surfings/Cogitation/wisdom_of_insecurity_by_alan_wat.htm
			continue
		}
		content += t.String()
	}
	return content, imageUrls, nil
}