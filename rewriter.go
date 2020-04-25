package main

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// id -> true
type hiddenIdsMap map[string]bool

// element -> class -> true
type hiddenTagsMap map[string]map[string]bool

// Stolen from go.net/html's render.go.
var voidElements = map[string]bool{
	"area":    true,
	"base":    true,
	"br":      true,
	"col":     true,
	"command": true,
	"embed":   true,
	"hr":      true,
	"img":     true,
	"input":   true,
	"keygen":  true,
	"link":    true,
	"meta":    true,
	"param":   true,
	"source":  true,
	"track":   true,
	"wbr":     true,
}

func getAttrValue(token *html.Token, name string) string {
	for i := range token.Attr {
		if token.Attr[i].Key == name {
			return token.Attr[i].Val
		}
	}
	return ""
}

type rewriter struct {
	cfg config
}

// readHiddenTagsFile returns maps containing the tags that should be hidden for url.
func (rw *rewriter) readHiddenTagsFile(url string) (*hiddenIdsMap, *hiddenTagsMap, error) {
	ids := make(hiddenIdsMap)
	tags := make(hiddenTagsMap)
	if len(rw.cfg.HiddenTagsFile) == 0 {
		return &ids, &tags, nil
	}

	// host -> [element.class, element.class, ...]
	// "div.class" matches all divs with class "class".
	// "div.*" or just "div" matches all divs.
	// "*.class" matches all elements with class "class".
	// "#id" matches the element with ID "id".
	data := make(map[string][]string)
	if err := readJSONFile(rw.cfg.HiddenTagsFile, &data); err != nil {
		return nil, nil, err
	}

	urlHost := getHost(url)
	for host, entries := range data {
		if host != "*" && host != urlHost && !strings.HasSuffix(urlHost, "."+host) {
			continue
		}

		for _, entry := range entries {
			parts := strings.Split(entry, ".")
			if len(parts) == 1 && len(parts[0]) > 1 && parts[0][0] == '#' {
				ids[parts[0][1:]] = true
			} else if len(parts) == 1 || len(parts) == 2 {
				element := parts[0]
				class := "*"
				if len(parts) == 2 {
					class = parts[1]
				}
				if _, ok := tags[element]; !ok {
					tags[element] = make(map[string]bool)
				}
				tags[element][class] = true
			} else {
				return nil, nil, fmt.Errorf("expected #id, element, or element.class in %q", entry)
			}
		}
	}
	return &ids, &tags, nil
}

func (rw *rewriter) shouldHideToken(t *html.Token, ids *hiddenIdsMap, tags *hiddenTagsMap) bool {
	id := getAttrValue(t, "id")
	if len(id) > 0 && (*ids)[id] {
		return true
	}
	if classes, ok := (*tags)[t.Data]; ok {
		if _, ok := classes["*"]; ok {
			return true
		}
		for _, c := range strings.Fields(getAttrValue(t, "class")) {
			if _, ok := classes[c]; ok {
				return true
			}
		}
	}
	if wildcardClasses, ok := (*tags)["*"]; ok {
		for _, tc := range strings.Fields(getAttrValue(t, "class")) {
			for wc := range wildcardClasses {
				if tc == wc {
					return true
				}
			}
		}
	}
	return false
}

// fixImageURL fixes up <img> elements that Readability decided to break because
// they had srcset attributes. Specifically, an <img> element with both src and
// srcset attributes seems to end up with just a src attribute containing a
// URL-escaped copy of the srcset value. That makes absolutely no sense.
func (rw *rewriter) fixImageURL(url string) string {
	if m, _ := regexp.MatchString("%20\\d+[wx](,|$)", url); !m {
		return url
	}
	// Just chop off the first space and everything after it.
	index := strings.Index(url, "%20")
	newURL := url[0:index]
	rw.cfg.Logger.Printf("Rewrote broken-looking image URL %q to %q\n", url, newURL)
	return newURL
}

// RewriteContent rewrites HTML that is passed to it. imageURLs maps from local
// filename to the original remote image URL.
func (rw *rewriter) RewriteContent(input, url string) (content string, imageURLs map[string]string, err error) {
	hiddenIds, hiddenTags, err := rw.readHiddenTagsFile(url)
	if err != nil {
		return "", nil, err
	}

	imageURLs = make(map[string]string)
	hideDepth := 0

	z := html.NewTokenizer(strings.NewReader(input))
	for {
		if z.Next() == html.ErrorToken {
			if z.Err() == io.EOF {
				return content, imageURLs, nil
			}
			return "", nil, z.Err()
		}
		t := z.Token()
		isStart := t.Type == html.StartTagToken
		isEnd := t.Type == html.EndTagToken
		isVoid, _ := voidElements[t.Data]

		// Check if we're nested within a hidden element.
		if hideDepth > 0 {
			if isEnd {
				hideDepth--
			} else if isStart && !isVoid {
				hideDepth++
			}
			continue
		}

		if rw.shouldHideToken(&t, hiddenIds, hiddenTags) {
			rw.cfg.Logger.Printf("Hiding <%v> token with id %q and class(es) %q\n",
				t.Data, getAttrValue(&t, "id"), getAttrValue(&t, "class"))
			if isStart {
				hideDepth = 1
			}
			continue
		}

		extraText := ""

		if isStart && t.Data == "img" {
			hasSrc := false
			numAttr := 0
			for i := range t.Attr {
				attr := &t.Attr[i]
				if attr.Key == "src" && len(attr.Val) > 0 {
					hasSrc = true
					if rw.cfg.DownloadImages {
						imageURL := rw.fixImageURL(attr.Val)
						filename := getLocalImageFilename(imageURL)
						imageURLs[filename] = imageURL
						attr.Val = filename
					}
				} else if attr.Key == "title" && len(attr.Val) > 0 {
					extraText = "\n<div class=\"img-title\">" +
						html.EscapeString(attr.Val) + "</div>\n"
				} else if attr.Key == "srcset" {
					// Drop srcset attributes, since browsers will load them
					// preferentially over rewritten src attributes.
					continue
				}
				t.Attr[numAttr] = *attr
				numAttr++
			}
			if !hasSrc {
				// kindlegen barfs on empty <img> tags. One appears in
				// http://online.wsj.com/articles/google-to-collect-data-to-define-healthy-human-1406246214.
				continue
			}
			t.Attr = t.Attr[:numAttr]
		} else if (isStart || isEnd) && t.Data == "h1" {
			// Downgrade <h1> to <h2>.
			t.Data = "h2"
		} else if (isStart || isEnd) && (t.Data == "h4" || t.Data == "h5" || t.Data == "h6") {
			// <h6> seems to mainly be used by people who don't know what
			// they're doing. Upgrade <h4>, <h5>, and <h6> to <h3>.
			t.Data = "h3"
		} else if isStart && t.Data == "iframe" {
			// Readability puts YouTube videos into iframes but kindlegen
			// doesn't know what to do with them.
			continue
		} else if (isStart || isEnd) && t.Data == "noscript" {
			// Tell the tokenizer to interpret nested elements. This handles the
			// non-JS tags for lazily-loaded images on theverge.com.
			if isStart {
				z.NextIsNotRawText()
			}
			// Keep kindlegen from complaining about <noscript>.
			continue
		} else if (isStart || isEnd) && t.Data == "body" {
			// Why does Readability leave body tags within the content
			// sometimes? See e.g.
			// http://kirtimukha.com/surfings/Cogitation/wisdom_of_insecurity_by_alan_wat.htm
			continue
		}

		content += t.String() + extraText
	}
	return content, imageURLs, nil
}
