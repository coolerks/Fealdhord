// Package feed 负责受控发现、解析与不可信内容规范化。
package feed

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"html"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/coolerks/Fealdhord/internal/netpolicy"
	"github.com/microcosm-cc/bluemonday"
	"github.com/mmcdole/gofeed"
	jsonfeed "github.com/mmcdole/gofeed/json"
	"golang.org/x/net/html/charset"
)

var ErrParse = errors.New("无法解析订阅内容")
var ErrNoFeed = errors.New("网页未提供订阅地址")

type Candidate struct {
	URL   string `json:"url"`
	Title string `json:"title"`
	Type  string `json:"type"`
}
type ImageCandidate struct {
	URL    string `json:"url"`
	Source string `json:"source"`
	Alt    string `json:"alt"`
}
type Item struct {
	GUID, Title, URL, Author, Summary, HTML, Text string
	PublishedAt, UpdatedAt                        *int64
	Images                                        []ImageCandidate
}
type Document struct {
	Title, URL, SiteURL, Description string
	Items                            []Item
}

var cleanHTML = func() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("p", "br", "div", "span", "h1", "h2", "h3", "h4", "h5", "h6", "blockquote", "pre", "code", "strong", "em", "b", "i", "u", "s", "ul", "ol", "li", "table", "thead", "tbody", "tr", "th", "td", "hr", "a")
	p.AllowAttrs("href").OnElements("a")
	p.AllowURLSchemes("http", "https")
	p.RequireParseableURLs(true)
	return p
}()

func bounded(s string, max int) string {
	if len(s) <= max {
		return s
	}
	s = s[:max]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}
func Text(raw string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(raw))
	if err != nil {
		return ""
	}
	doc.Find("script,style,iframe,object,template,svg").Remove()
	doc.Find("p,br,div,li,pre,h1,h2,h3,h4,blockquote,tr").Each(func(_ int, s *goquery.Selection) { s.AppendHtml("\n\n") })
	return strings.TrimSpace(doc.Find("body").Text())
}
func ResolveURL(raw, base string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	b, err := url.Parse(base)
	if err != nil {
		return ""
	}
	target, err := netpolicy.NormalizeURL(b.ResolveReference(u).String())
	if err != nil {
		return ""
	}
	return target
}
func structuralCheck(data []byte) error {
	if len(data) == 0 || int64(len(data)) > netpolicy.FeedLimit {
		return ErrParse
	}
	depth, tokens := 0, 0
	if data[0] == '{' {
		decoder := json.NewDecoder(bytes.NewReader(data))
		for {
			token, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				return ErrParse
			}
			tokens++
			if delim, ok := token.(json.Delim); ok {
				if delim == '{' || delim == '[' {
					depth++
				} else {
					depth--
				}
			}
			if depth > 128 || tokens > 200000 {
				return ErrParse
			}
		}
	} else {
		decoder := xml.NewDecoder(bytes.NewReader(data))
		decoder.CharsetReader = charset.NewReaderLabel
		items := 0
		for {
			token, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				return ErrParse
			}
			tokens++
			switch t := token.(type) {
			case xml.StartElement:
				depth++
				if t.Name.Local == "item" || t.Name.Local == "entry" {
					items++
				}
			case xml.EndElement:
				depth--
			case xml.Directive:
				return ErrParse
			}
			if depth > 128 || items > 10000 || tokens > 200000 {
				return ErrParse
			}
		}
	}
	return nil
}
func Parse(ctx context.Context, data []byte, base string) (*Document, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data = bytes.TrimSpace(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}))
	if err := structuralCheck(data); err != nil {
		return nil, err
	}
	parser := gofeed.NewParser()
	parser.KeepOriginalFeed = true
	parsed, err := parser.Parse(bytes.NewReader(data))
	if err != nil || len(parsed.Items) > 10000 {
		return nil, ErrParse
	}
	document := &Document{URL: base, Title: bounded(Text(parsed.Title), 4096), SiteURL: ResolveURL(parsed.Link, base), Description: bounded(Text(parsed.Description), 16384), Items: []Item{}}
	if document.Title == "" {
		u, _ := url.Parse(base)
		document.Title = u.Hostname()
	}
	originalJSON, _ := parsed.OriginalFeed().(*jsonfeed.Feed)
	if originalJSON != nil {
		document.Title = bounded(originalJSON.Title, 4096)
		document.Description = bounded(originalJSON.Description, 16384)
		if document.Title == "" {
			u, _ := url.Parse(base)
			document.Title = u.Hostname()
		}
	}
	for index, entry := range parsed.Items {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if entry == nil {
			continue
		}
		item := Item{GUID: bounded(entry.GUID, 4096), Title: bounded(Text(entry.Title), 4096), URL: ResolveURL(entry.Link, base), Summary: bounded(Text(entry.Description), 16384)}
		if len(entry.GUID) > 4096 {
			return nil, ErrParse
		}
		if originalJSON != nil {
			item.Title = bounded(entry.Title, 4096)
			item.Summary = bounded(entry.Description, 16384)
		}
		if item.Title == "" {
			item.Title = "无标题"
		}
		if len(entry.Authors) > 0 && entry.Authors[0] != nil {
			item.Author = bounded(entry.Authors[0].Name, 1024)
		}
		content := entry.Content
		if content == "" {
			content = entry.Description
		}
		if originalJSON != nil && index < len(originalJSON.Items) {
			source := originalJSON.Items[index]
			if source.ContentHTML == "" {
				content = "<p>" + html.EscapeString(source.ContentText) + "</p>"
			}
		}
		if len(content) > 4<<20 {
			return nil, ErrParse
		}
		item.HTML = cleanHTML.Sanitize(content)
		item.Text = Text(item.HTML)
		if len(item.Text)+len(item.HTML) > 4<<20 {
			return nil, ErrParse
		}
		item.PublishedAt = timestamp(entry.PublishedParsed)
		item.UpdatedAt = timestamp(entry.UpdatedParsed)
		imageBase := base
		if item.URL != "" {
			imageBase = item.URL
		}
		add := func(raw, source, alt string) {
			if len(item.Images) >= 24 {
				return
			}
			resolved := ResolveURL(raw, imageBase)
			if resolved == "" {
				return
			}
			for _, old := range item.Images {
				if old.URL == resolved {
					return
				}
			}
			item.Images = append(item.Images, ImageCandidate{resolved, source, bounded(alt, 1024)})
		}
		for _, key := range []string{"content", "thumbnail"} {
			for _, ext := range entry.Extensions["media"][key] {
				if key == "thumbnail" || strings.HasPrefix(ext.Attrs["type"], "image/") || ext.Attrs["medium"] == "image" {
					add(ext.Attrs["url"], "media_"+key, "")
				}
			}
		}
		for _, enclosure := range entry.Enclosures {
			if enclosure != nil && strings.HasPrefix(enclosure.Type, "image/") {
				add(enclosure.URL, "enclosure", "")
			}
		}
		if entry.Image != nil && parsed.FeedType == "json" {
			add(entry.Image.URL, "json_image", entry.Image.Title)
		}
		if originalJSON != nil && index < len(originalJSON.Items) {
			add(originalJSON.Items[index].BannerImage, "json_image", "")
		}
		for _, part := range []struct{ raw, source string }{{entry.Content, "content"}, {entry.Description, "summary"}} {
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(part.raw))
			if err == nil {
				doc.Find("img").Each(func(_ int, s *goquery.Selection) {
					raw, _ := s.Attr("src")
					if raw == "" {
						raw, _ = s.Attr("data-src")
					}
					if raw == "" {
						raw, _ = s.Attr("data-original")
					}
					alt, _ := s.Attr("alt")
					add(raw, part.source, alt)
				})
			}
		}
		document.Items = append(document.Items, item)
	}
	return document, nil
}
func timestamp(value *time.Time) *int64 {
	if value == nil || value.IsZero() {
		return nil
	}
	n := value.UTC().UnixMilli()
	return &n
}

func Discover(ctx context.Context, fetcher netpolicy.Fetcher, raw string) ([]Candidate, error) {
	response, err := fetcher.Fetch(ctx, raw, netpolicy.Request{Accept: "application/rss+xml, application/atom+xml, application/feed+json, text/html;q=0.8"})
	if err != nil {
		return nil, err
	}
	if response.Status != http.StatusOK {
		return nil, ErrNoFeed
	}
	if document, err := Parse(ctx, response.Body, response.URL); err == nil {
		return []Candidate{{URL: response.URL, Title: document.Title, Type: "feed"}}, nil
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(response.Body))
	if err != nil {
		return nil, ErrNoFeed
	}
	candidates := []Candidate{}
	seen := map[string]bool{}
	doc.Find("link[rel][href]").Each(func(_ int, s *goquery.Selection) {
		if len(candidates) >= 32 {
			return
		}
		rel, _ := s.Attr("rel")
		alternate := false
		for _, r := range strings.Fields(strings.ToLower(rel)) {
			if r == "alternate" {
				alternate = true
			}
		}
		kind, _ := s.Attr("type")
		kind, _, _ = mime.ParseMediaType(kind)
		if !alternate || (kind != "application/rss+xml" && kind != "application/atom+xml" && kind != "application/feed+json" && kind != "application/json") {
			return
		}
		href, _ := s.Attr("href")
		target := ResolveURL(href, response.URL)
		if target == "" || seen[target] {
			return
		}
		seen[target] = true
		title, _ := s.Attr("title")
		candidates = append(candidates, Candidate{target, bounded(Text(title), 4096), kind})
	})
	if len(candidates) == 0 {
		return nil, ErrNoFeed
	}
	return candidates, nil
}
