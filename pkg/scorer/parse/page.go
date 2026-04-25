package parse

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// ParsedPage is the single-pass representation of an HTML page that
// all analysers consume. The parse work happens once per page; each
// analyser reads the fields it needs and never reopens the raw bytes.
type ParsedPage struct {
	URL         string
	StatusCode  int
	HTMLSize    int
	FetchedAt   time.Time
	LastModHTTP time.Time // from Last-Modified header

	Title          string
	MetaDescrip    string
	MetaAuthor     string
	Canonical      string
	OGTags         map[string]string // keys like "og:title"
	TwitterTags    map[string]string
	HasViewport    bool
	HeadScriptTags int // counted for render-blocking-script check
	RenderBlocking int // subset of HeadScriptTags without async/defer

	Headings []Heading
	WordText string // cleaned body text
	Words    []string
	WordN    int

	Paragraphs []string
	Sentences  []string

	HasArticleTag bool
	ArticleWordN  int

	Lists       int // count of <ul>+<ol>
	Tables      int
	PreBlocks   int // code blocks
	Blockquotes int
	QTags       int

	Anchors []Anchor

	JSONLD []map[string]any // parsed JSON-LD blobs; each is either a top-level object or flattened from @graph

	// Freshness signals harvested from Article/BlogPosting JSON-LD.
	DateModified  time.Time
	DatePublished time.Time
}

// Heading is an `h1`..`h6` occurrence.
type Heading struct {
	Level int
	Text  string
}

// Anchor is an `<a href>` occurrence.
type Anchor struct {
	Href     string
	Text     string
	Rel      string
	Internal bool // relative to the page's own host
}

var multiWS = regexp.MustCompile(`\s+`)

// ParsePage reads an HTML body and returns a ParsedPage. pageURL is
// the post-redirect URL used for internal-link classification.
func ParsePage(pageURL string, body []byte) (*ParsedPage, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	p := &ParsedPage{
		URL:         pageURL,
		HTMLSize:    len(body),
		OGTags:      map[string]string{},
		TwitterTags: map[string]string{},
	}
	host := hostOf(pageURL)

	// <head> attributes.
	p.Title = strings.TrimSpace(doc.Find("head title").First().Text())
	doc.Find("head meta").Each(func(_ int, s *goquery.Selection) {
		name, _ := s.Attr("name")
		property, _ := s.Attr("property")
		content, _ := s.Attr("content")
		name = strings.ToLower(name)
		property = strings.ToLower(property)
		content = strings.TrimSpace(content)
		switch {
		case name == "description":
			p.MetaDescrip = content
		case name == "author":
			p.MetaAuthor = content
		case name == "viewport":
			p.HasViewport = true
		case strings.HasPrefix(property, "og:"):
			p.OGTags[property] = content
		case strings.HasPrefix(name, "twitter:"):
			p.TwitterTags[name] = content
		}
	})
	if href, ok := doc.Find(`head link[rel="canonical"]`).First().Attr("href"); ok {
		p.Canonical = strings.TrimSpace(href)
	}

	// Render-blocking script heuristic: <script> in <head> without
	// async, defer, or type=module.
	doc.Find("head script").Each(func(_ int, s *goquery.Selection) {
		p.HeadScriptTags++
		if _, hasSrc := s.Attr("src"); !hasSrc {
			return
		}
		_, a := s.Attr("async")
		_, d := s.Attr("defer")
		t, _ := s.Attr("type")
		if !a && !d && strings.ToLower(t) != "module" {
			p.RenderBlocking++
		}
	})

	// Headings.
	for lvl := 1; lvl <= 6; lvl++ {
		sel := doc.Find("h" + string(rune('0'+lvl)))
		sel.Each(func(_ int, s *goquery.Selection) {
			text := strings.TrimSpace(multiWS.ReplaceAllString(s.Text(), " "))
			if text == "" {
				return
			}
			p.Headings = append(p.Headings, Heading{Level: lvl, Text: text})
		})
	}

	// Body text + structural counts.
	body2 := doc.Find("body").First()
	p.HasArticleTag = body2.Find("article").Length() > 0
	if p.HasArticleTag {
		text := strings.TrimSpace(multiWS.ReplaceAllString(body2.Find("article").First().Text(), " "))
		p.ArticleWordN = len(strings.Fields(text))
	}
	p.Lists = body2.Find("ul,ol").Length()
	p.Tables = body2.Find("table").Length()
	p.PreBlocks = body2.Find("pre").Length()
	p.Blockquotes = body2.Find("blockquote").Length()
	p.QTags = body2.Find("q").Length()

	bodyText := strings.TrimSpace(multiWS.ReplaceAllString(body2.Text(), " "))
	p.WordText = bodyText
	p.Words = strings.Fields(bodyText)
	p.WordN = len(p.Words)

	// Paragraphs and sentences.
	body2.Find("p").Each(func(_ int, s *goquery.Selection) {
		t := strings.TrimSpace(multiWS.ReplaceAllString(s.Text(), " "))
		if t == "" {
			return
		}
		p.Paragraphs = append(p.Paragraphs, t)
	})
	p.Sentences = splitSentences(bodyText)

	// Anchors.
	body2.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		href = strings.TrimSpace(href)
		if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(strings.ToLower(href), "javascript:") || strings.HasPrefix(strings.ToLower(href), "mailto:") || strings.HasPrefix(strings.ToLower(href), "tel:") {
			return
		}
		rel, _ := s.Attr("rel")
		text := strings.TrimSpace(multiWS.ReplaceAllString(s.Text(), " "))
		a := Anchor{Href: href, Text: text, Rel: rel}
		targetHost := hostOf(resolveHref(pageURL, href))
		a.Internal = targetHost == host || targetHost == ""
		p.Anchors = append(p.Anchors, a)
	})

	// JSON-LD blocks.
	doc.Find(`script[type="application/ld+json"]`).Each(func(_ int, s *goquery.Selection) {
		raw := strings.TrimSpace(s.Text())
		if raw == "" {
			return
		}
		for _, obj := range parseJSONLDBlob([]byte(raw)) {
			p.JSONLD = append(p.JSONLD, obj)
			// Harvest freshness signals. Schema.org allows either full
			// RFC3339 or a plain ISO date.
			if t := firstString(obj, "dateModified"); t != "" {
				if parsed, ok := parseSchemaDate(t); ok {
					if parsed.After(p.DateModified) {
						p.DateModified = parsed
					}
				}
			}
			if t := firstString(obj, "datePublished"); t != "" {
				if parsed, ok := parseSchemaDate(t); ok {
					if p.DatePublished.IsZero() || parsed.Before(p.DatePublished) {
						p.DatePublished = parsed
					}
				}
			}
		}
	})

	return p, nil
}

// parseJSONLDBlob accepts either a single object, an array of objects,
// or an object with @graph. Returns a flat list of top-level objects.
// Invalid JSON returns nil.
func parseJSONLDBlob(raw []byte) []map[string]any {
	var any1 any
	if err := json.Unmarshal(raw, &any1); err != nil {
		return nil
	}
	var out []map[string]any
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if g, ok := x["@graph"]; ok {
				walk(g)
				return
			}
			out = append(out, x)
		case []any:
			for _, e := range x {
				walk(e)
			}
		}
	}
	walk(any1)
	return out
}

// firstString reads a string value from common JSON-LD shapes:
// either the raw string, or an object with "@value".
func firstString(obj map[string]any, key string) string {
	v, ok := obj[key]
	if !ok {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case map[string]any:
		if s, ok := x["@value"].(string); ok {
			return s
		}
	case []any:
		if len(x) > 0 {
			if s, ok := x[0].(string); ok {
				return s
			}
		}
	}
	return ""
}

// parseSchemaDate accepts the common schema.org/JSON-LD date shapes:
// RFC3339, date-only (YYYY-MM-DD), and a loose zoned RFC3339.
func parseSchemaDate(s string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339, "2006-01-02", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// splitSentences is a deliberately dumb splitter — enough for BLUF
// heuristics without pulling in an NLP library.
func splitSentences(s string) []string {
	out := []string{}
	cur := strings.Builder{}
	for _, r := range s {
		cur.WriteRune(r)
		if r == '.' || r == '!' || r == '?' {
			t := strings.TrimSpace(cur.String())
			if t != "" {
				out = append(out, t)
			}
			cur.Reset()
		}
	}
	if rem := strings.TrimSpace(cur.String()); rem != "" {
		out = append(out, rem)
	}
	return out
}

// hostOf returns the lowercase host of a URL, or "" if it doesn't parse
// or is relative.
func hostOf(u string) string {
	i := strings.Index(u, "://")
	if i < 0 {
		return ""
	}
	rest := u[i+3:]
	end := len(rest)
	for j, r := range rest {
		if r == '/' || r == '?' || r == '#' {
			end = j
			break
		}
	}
	host := rest[:end]
	if at := strings.LastIndex(host, "@"); at >= 0 {
		host = host[at+1:]
	}
	if colon := strings.Index(host, ":"); colon >= 0 {
		host = host[:colon]
	}
	return strings.ToLower(host)
}

// resolveHref resolves a (possibly relative) href against a page URL.
// Returns the input if parsing fails.
func resolveHref(pageURL, href string) string {
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	// Trivial relative resolution: absolute-path → scheme+host+path.
	if strings.HasPrefix(href, "/") {
		i := strings.Index(pageURL, "://")
		if i < 0 {
			return href
		}
		rest := pageURL[i+3:]
		end := len(rest)
		for j, r := range rest {
			if r == '/' {
				end = j
				break
			}
		}
		return pageURL[:i+3] + rest[:end] + href
	}
	// Relative → drop last path segment.
	idx := strings.LastIndex(pageURL, "/")
	if idx < 0 {
		return pageURL + "/" + href
	}
	return pageURL[:idx+1] + href
}
