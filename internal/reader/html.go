package reader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/feed"
	"github.com/microcosm-cc/bluemonday"
	xhtml "golang.org/x/net/html"
)

var ErrContent = errors.New("正文无法提取")
var displayPolicy = func() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("p", "br", "div", "span", "h1", "h2", "h3", "h4", "h5", "h6", "blockquote", "pre", "code", "strong", "em", "b", "i", "u", "s", "ul", "ol", "li", "table", "thead", "tbody", "tfoot", "tr", "th", "td", "hr", "a", "img")
	p.AllowAttrs("href").Matching(regexp.MustCompile(`^#reader-link$`)).OnElements("a")
	p.AllowAttrs("data-reader-link").Matching(regexp.MustCompile(`^[a-f0-9]{64}$`)).OnElements("a")
	p.AllowAttrs("src").Matching(regexp.MustCompile(`^/media/inline/[a-f0-9]{64}$`)).OnElements("img")
	p.AllowAttrs("alt", "loading", "decoding").OnElements("img")
	p.AllowAttrs("width", "height").Matching(regexp.MustCompile(`^[0-9]{1,4}$`)).OnElements("img")
	p.AllowRelativeURLs(true)
	return p
}()

func boundedDocument(ctx context.Context, raw string) (*xhtml.Node, error) {
	if len(raw) > 5<<20 {
		return nil, ErrContent
	}
	doc, err := xhtml.Parse(strings.NewReader(raw))
	if err != nil {
		return nil, ErrContent
	}
	type entry struct {
		node  *xhtml.Node
		depth int
	}
	stack := []entry{{doc, 0}}
	count := 0
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		count++
		if count > 100000 || current.depth > 128 {
			return nil, ErrContent
		}
		if count%256 == 0 && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		for child := current.node.FirstChild; child != nil; child = child.NextSibling {
			stack = append(stack, entry{child, current.depth + 1})
		}
	}
	return doc, ctx.Err()
}
func resourceID(articleID, mode, kind, url string) string {
	sum := sha256.Sum256([]byte(articleID + "\x00" + mode + "\x00" + kind + "\x00" + url))
	return hex.EncodeToString(sum[:])
}
func prepareHTML(ctx context.Context, article database.ReaderArticle, mode, raw, base string, images bool) (string, []database.ReaderResource, error) {
	if _, err := boundedDocument(ctx, raw); err != nil {
		return "", nil, err
	}
	safe := feed.SanitizeBody(raw, base)
	if strings.TrimSpace(feed.Text(safe)) == "" && article.Text != "" {
		safe = "<p>" + html.EscapeString(article.Text) + "</p>"
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(safe))
	if err != nil {
		return "", nil, ErrContent
	}
	resources := []database.ReaderResource{}
	seen := map[string]bool{}
	imageCount, linkCount := 0, 0
	register := func(kind, url string) string {
		id := resourceID(article.ID, mode, kind, url)
		if !seen[id] {
			seen[id] = true
			resources = append(resources, database.ReaderResource{ID: id, ArticleID: article.ID, ArticleHash: article.Hash, Mode: mode, Kind: kind, URL: url})
		}
		return id
	}
	doc.Find("img").Each(func(_ int, node *goquery.Selection) {
		if !images || imageCount >= 32 {
			node.Remove()
			return
		}
		target := feed.ResolveURL(node.AttrOr("src", ""), base)
		if target == "" {
			node.Remove()
			return
		}
		imageCount++
		id := register("image", target)
		node.SetAttr("src", "/media/inline/"+id).SetAttr("loading", "lazy").SetAttr("decoding", "async")
		for _, attr := range []string{"width", "height"} {
			value, err := strconv.Atoi(node.AttrOr(attr, ""))
			if err != nil || value < 1 || value > 4096 {
				node.RemoveAttr(attr)
			}
		}
	})
	doc.Find("a").Each(func(_ int, node *goquery.Selection) {
		target := feed.ResolveURL(node.AttrOr("href", ""), base)
		if target == "" || linkCount >= 256 {
			node.RemoveAttr("href")
			return
		}
		linkCount++
		node.SetAttr("href", "#reader-link").SetAttr("data-reader-link", register("link", target))
	})
	result, err := doc.Find("body").Html()
	if err != nil {
		return "", nil, ErrContent
	}
	result = displayPolicy.Sanitize(result)
	if len(result) > 4<<20 {
		return "", nil, fmt.Errorf("%w: size", ErrContent)
	}
	return result, resources, ctx.Err()
}
