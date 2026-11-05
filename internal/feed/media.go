package feed

import (
	"bytes"
	"encoding/xml"
	"github.com/PuerkitoBio/goquery"
	"github.com/mmcdole/gofeed"
	"golang.org/x/net/html/charset"
	"io"
	"net/url"
	"strconv"
	"strings"
)

type sourceBase struct {
	base, link, content, summary string
	explicit                     bool
}

func xmlSourceBases(data []byte, base string) []sourceBase {
	if len(data) == 0 || data[0] == '{' {
		return nil
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.CharsetReader = charset.NewReaderLabel
	stack := []string{base}
	result := []sourceBase{}
	current, entryDepth := -1, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF || err != nil {
			break
		}
		switch node := token.(type) {
		case xml.StartElement:
			scope := stack[len(stack)-1]
			for _, attr := range node.Attr {
				if attr.Name.Local == "base" && attr.Name.Space == "http://www.w3.org/XML/1998/namespace" && attr.Value != "" {
					scope = ResolveURL(attr.Value, scope)
				}
			}
			stack = append(stack, scope)
			if node.Name.Local == "item" || node.Name.Local == "entry" {
				result = append(result, sourceBase{base: scope, explicit: scope != base})
				current = len(result) - 1
				entryDepth = len(stack)
			}
			if current >= 0 {
				switch node.Name.Local {
				case "link":
					rel := ""
					for _, attr := range node.Attr {
						if attr.Name.Local == "rel" {
							rel = attr.Value
						}
					}
					if result[current].link == "" || rel == "alternate" {
						result[current].link = scope
					}
				case "content", "encoded":
					if scope != base {
						result[current].content = scope
					}
				case "summary", "description":
					if scope != base {
						result[current].summary = scope
					}
				}
			}
		case xml.EndElement:
			if current >= 0 && len(stack) == entryDepth {
				current = -1
			}
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	return result
}
func imagePath(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	p := strings.ToLower(u.Path)
	for _, suffix := range []string{".jpg", ".jpeg", ".png", ".webp", ".gif"} {
		if strings.HasSuffix(p, suffix) {
			return true
		}
	}
	return false
}
func excludedImage(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return true
	}
	path := strings.ToLower(u.Path)
	for _, term := range []string{"favicon", "/avatar", "gravatar", "/tracking", "/pixel", "/spacer"} {
		if strings.Contains(path, term) {
			return true
		}
	}
	return false
}
func declaredSmall(width, height string) bool {
	for _, raw := range []string{width, height} {
		n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(raw), "px"))
		if err == nil && n > 0 && n < 64 {
			return true
		}
	}
	return false
}
func sourceSet(raw string) string {
	if len(raw) > 8192 {
		return ""
	}
	chosen, best := "", 1e12
	for _, part := range strings.Split(raw, ",") {
		fields := strings.Fields(part)
		if len(fields) == 0 || len(fields) > 2 {
			continue
		}
		score := 0.0
		if len(fields) == 2 {
			last := fields[1]
			if len(last) < 2 {
				continue
			}
			value, err := strconv.ParseFloat(last[:len(last)-1], 64)
			if err != nil || value <= 0 {
				continue
			}
			target := 480.0
			if strings.HasSuffix(last, "x") {
				target = 2
			} else if !strings.HasSuffix(last, "w") {
				continue
			}
			score = value - target
			if score < 0 {
				score = 100000 - score
			}
		}
		if score < best {
			best = score
			chosen = fields[0]
		}
	}
	return chosen
}
func extractImages(entry *gofeed.Item, base string, scope sourceBase, jsonItem bool, banner string) []ImageCandidate {
	result := []ImageCandidate{}
	add := func(raw, source, alt, context string) {
		if len(result) >= 24 {
			return
		}
		resolved := ResolveURL(raw, context)
		if resolved == "" || excludedImage(resolved) {
			return
		}
		for _, old := range result {
			if old.URL == resolved {
				return
			}
		}
		result = append(result, ImageCandidate{URL: resolved, Source: source, Alt: bounded(alt, 1024)})
	}
	for _, key := range []string{"content", "thumbnail"} {
		for _, ext := range entry.Extensions["media"][key] {
			if declaredSmall(ext.Attrs["width"], ext.Attrs["height"]) {
				continue
			}
			if key == "thumbnail" || strings.HasPrefix(ext.Attrs["type"], "image/") || ext.Attrs["medium"] == "image" || imagePath(ext.Attrs["url"]) {
				context := base
				if ext.Attrs["base"] != "" {
					context = ResolveURL(ext.Attrs["base"], context)
				}
				add(ext.Attrs["url"], "media_"+key, "", context)
			}
		}
	}
	for _, enclosure := range entry.Enclosures {
		if enclosure != nil && strings.HasPrefix(enclosure.Type, "image/") {
			add(enclosure.URL, "enclosure", "", base)
		}
	}
	if jsonItem && entry.Image != nil {
		add(entry.Image.URL, "json_image", entry.Image.Title, base)
	}
	if jsonItem {
		add(banner, "json_image", "", base)
	}
	for _, part := range []struct{ raw, source, context string }{{entry.Content, "content", scope.content}, {entry.Description, "summary", scope.summary}} {
		context := part.context
		if context == "" {
			context = base
		}
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(part.raw))
		if err != nil {
			continue
		}
		if link, ok := doc.Find("base[href]").First().Attr("href"); ok {
			if resolved := ResolveURL(link, context); resolved != "" {
				context = resolved
			}
		}
		doc.Find("img").Each(func(_ int, s *goquery.Selection) {
			width, _ := s.Attr("width")
			height, _ := s.Attr("height")
			if declaredSmall(width, height) {
				return
			}
			alt, _ := s.Attr("alt")
			for _, key := range []string{"srcset", "data-srcset"} {
				if raw, ok := s.Attr(key); ok {
					add(sourceSet(raw), part.source, alt, context)
				}
			}
			for _, key := range []string{"data-src", "data-original", "src"} {
				if raw, ok := s.Attr(key); ok {
					add(raw, part.source, alt, context)
				}
			}
		})
	}
	return result
}

// PageImages 仅由用户显式开启网页补图后的受控网页字节调用。
func PageImages(body []byte, base string) []ImageCandidate {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	out := []ImageCandidate{}
	for _, kind := range []struct{ selector, source string }{{`meta[property="og:image"]`, "og"}, {`meta[name="twitter:image"]`, "twitter"}} {
		doc.Find(kind.selector).Each(func(_ int, s *goquery.Selection) {
			if len(out) >= 5 {
				return
			}
			raw, _ := s.Attr("content")
			target := ResolveURL(raw, base)
			if target == "" || excludedImage(target) {
				return
			}
			for _, old := range out {
				if old.URL == target {
					return
				}
			}
			out = append(out, ImageCandidate{URL: target, Source: kind.source})
		})
	}
	return out
}
