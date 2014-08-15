package main

import (
	"code.google.com/p/go.net/html"
	//"encoding/json"
	"io"
	"strings"
)

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

	// element -> class -> true
	hiddenTokens map[string]map[string]bool
}

func newRewriter(cfg Config) *Rewriter {
	r := &Rewriter{cfg: cfg}

	// TODO: Move this into a config file.
	r.hiddenTokens = make(map[string]map[string]bool)
	for _, s := range []string{
		// avidbruxist.com
		"div.postauthor",
		// bloomberg.com
		"div.caption_preview",
		"div.image_full_view",
		// businessweek.com
		"span.credit",
		// modernfarmer.com
		"span.mf-single-article-tags",
		"span.mf-single-article-post-tags",
		// nationaljournal.com
		"a.facebookSocialStrip",
		"a.googleSocialStrip",
		"a.twitterSocialStrip",
		"div.socialStrip",
		"span.shareThisStory",
		// npr.org
		"b.hide-caption",
		// nytimes.com
		"a.skip-to-text-link",
		"div.pullQuote",
		// wsj.com
		"span.ticker",
		"span.t-content",
	} {
		parts := strings.Split(s, ".")
		if len(parts) != 2 {
			r.cfg.Logger.Fatalf("Expected element.class in %q", s)
		}
		if _, ok := r.hiddenTokens[parts[0]]; !ok {
			r.hiddenTokens[parts[0]] = make(map[string]bool)
		}
		r.hiddenTokens[parts[0]][parts[1]] = true
	}

	return r
}

func (r *Rewriter) shouldHideToken(t html.Token) bool {
	if classes, ok := r.hiddenTokens[t.Data]; ok {
		for _, c := range strings.Fields(getAttrValue(t, "class")) {
			if _, ok := classes[c]; ok {
				return true
			}
		}
	}
	return false
}

func (r *Rewriter) rewriteContent(input string) (content string, imageUrls map[string]string, err error) {
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

		if r.shouldHideToken(t) {
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
