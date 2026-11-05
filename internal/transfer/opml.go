package transfer

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/feed"
	"github.com/coolerks/Fealdhord/internal/netpolicy"
	"golang.org/x/net/html/charset"
)

const maxFileBytes = 10 << 20

var ErrDocument = errors.New("OPML 文件无效或超过限制")
var ErrExpired = errors.New("文件授权或预览已过期")
var ErrFileChanged = errors.New("文件已变化")

type rawOutline struct {
	key, parent, title, url, site, kind string
	children                            bool
	depth                               int
}

func parseOPML(ctx context.Context, data []byte) ([]database.ImportOutline, error) {
	if len(data) == 0 || len(data) > maxFileBytes {
		return nil, ErrDocument
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.CharsetReader = charset.NewReaderLabel
	raw := []rawOutline{}
	stack := []string{}
	outlines := []int{}
	rootSeen, bodySeen := false, false
	tokens := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, ErrDocument
		}
		tokens++
		if tokens > 200000 || ctx.Err() != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, ErrDocument
		}
		switch node := token.(type) {
		case xml.Directive:
			return nil, ErrDocument
		case xml.ProcInst:
			if node.Target != "xml" {
				return nil, ErrDocument
			}
		case xml.StartElement:
			name := strings.ToLower(node.Name.Local)
			if len(stack) == 0 {
				if rootSeen || name != "opml" {
					return nil, ErrDocument
				}
				rootSeen = true
				for _, attr := range node.Attr {
					if attr.Name.Local == "version" && attr.Value != "1.0" && attr.Value != "1.1" && attr.Value != "2.0" {
						return nil, ErrDocument
					}
				}
			}
			if len(stack) > 32 {
				return nil, ErrDocument
			}
			if name == "body" && len(stack) == 1 && stack[0] == "opml" {
				if bodySeen {
					return nil, ErrDocument
				}
				bodySeen = true
			}
			inBody := false
			for _, ancestor := range stack {
				if ancestor == "body" {
					inBody = true
				}
			}
			stack = append(stack, name)
			if name != "outline" || !inBody {
				continue
			}
			if len(outlines) >= 16 || len(raw) >= 100000 {
				return nil, ErrDocument
			}
			attrs := map[string]string{}
			for _, attr := range node.Attr {
				key := strings.ToLower(attr.Name.Local)
				if _, exists := attrs[key]; exists {
					return nil, ErrDocument
				}
				attrs[key] = attr.Value
			}
			title := attrs["text"]
			if title == "" {
				title = attrs["title"]
			}
			item := rawOutline{key: fmt.Sprintf("outline-%d", len(raw)+1), title: strings.TrimSpace(feed.Text(title)), url: strings.TrimSpace(attrs["xmlurl"]), site: strings.TrimSpace(attrs["htmlurl"]), kind: strings.ToLower(attrs["type"]), depth: len(outlines)}
			if len(outlines) > 0 {
				parent := outlines[len(outlines)-1]
				raw[parent].children = true
				item.parent = raw[parent].key
			}
			raw = append(raw, item)
			outlines = append(outlines, len(raw)-1)
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, ErrDocument
			}
			if strings.EqualFold(node.Name.Local, "outline") && len(outlines) > 0 {
				outlines = outlines[:len(outlines)-1]
			}
			stack = stack[:len(stack)-1]
		}
	}
	if !rootSeen || !bodySeen || len(stack) != 0 {
		return nil, ErrDocument
	}
	result := []database.ImportOutline{}
	invalid := map[string]bool{}
	folderDepth := map[string]int{}
	unique := map[string]bool{}
	for _, raw := range raw {
		item := database.ImportOutline{Key: raw.key, ParentKey: raw.parent, Title: raw.title, Depth: raw.depth, Kind: "folder", Status: "new"}
		parentDepth := folderDepth[item.ParentKey]
		if raw.url != "" {
			item.Kind = "feed"
			canonical, err := netpolicy.NormalizeURL(raw.url)
			if err != nil || len(canonical) > 8192 {
				item.Status = "invalid"
				item.Reason = "订阅地址格式、协议或目标不受支持。"
			} else {
				item.URL = canonical
				unique[canonical] = true
			}
			if raw.children {
				item.Status = "invalid"
				item.Reason = "订阅节点不能作为分类容器。"
			}
			if item.Title == "" {
				parsed, _ := url.Parse(canonical)
				if parsed != nil {
					item.Title = parsed.Hostname()
				}
			}
			if len(item.Title) > 512 {
				item.Status = "invalid"
				item.Reason = "订阅名称过长。"
			}
			if raw.site != "" {
				item.SiteURL, _ = netpolicy.NormalizeURL(raw.site)
			}
		} else {
			if !raw.children && (raw.kind == "rss" || raw.kind == "atom" || raw.kind == "json") {
				item.Status = "invalid"
				item.Reason = "缺少订阅地址。"
			}
			if item.Title == "" || len(item.Title) > 256 {
				item.Status = "invalid"
				item.Reason = "分类名称为空或过长。"
			}
			folderDepth[item.Key] = parentDepth + 1
			if folderDepth[item.Key] > 5 {
				item.Status = "invalid"
				item.Reason = "分类层级最多五层。"
			}
		}
		if invalid[item.ParentKey] {
			item.Status = "invalid"
			item.Reason = "上级节点无法导入，保留层级而不静默展开。"
		}
		invalid[item.Key] = item.Status == "invalid"
		result = append(result, item)
	}
	if len(unique) > 10000 {
		return nil, ErrDocument
	}
	return result, nil
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(data []byte) (int, error) {
	if b.Len()+len(data) > maxFileBytes {
		return 0, ErrDocument
	}
	return b.Buffer.Write(data)
}
func encodeOPML(ctx context.Context, snapshot database.OPMLSnapshot) ([]byte, error) {
	output := &limitedBuffer{}
	output.WriteString(xml.Header)
	encoder := xml.NewEncoder(output)
	token := func(value xml.Token) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return encoder.EncodeToken(value)
	}
	opml := xml.StartElement{Name: xml.Name{Local: "opml"}, Attr: []xml.Attr{{Name: xml.Name{Local: "version"}, Value: "2.0"}}}
	if err := token(opml); err != nil {
		return nil, err
	}
	head := xml.StartElement{Name: xml.Name{Local: "head"}}
	title := xml.StartElement{Name: xml.Name{Local: "title"}}
	body := xml.StartElement{Name: xml.Name{Local: "body"}}
	for _, value := range []xml.Token{head, title, xml.CharData("折栈订阅"), title.End(), head.End(), body} {
		if err := token(value); err != nil {
			return nil, err
		}
	}
	feeds := map[string][]int{}
	categories := map[string][]int{}
	active := 0
	nodes := 0
	visitedCategories := map[string]bool{}
	visitedFeeds := map[string]bool{}
	for index, category := range snapshot.Categories {
		categories[category.ParentID.String] = append(categories[category.ParentID.String], index)
	}
	for index, f := range snapshot.Feeds {
		if f.ArchivedAt.Valid {
			continue
		}
		active++
		memberships := snapshot.Relations[f.ID]
		if len(memberships) == 0 {
			feeds[""] = append(feeds[""], index)
		} else {
			for _, id := range memberships {
				feeds[id] = append(feeds[id], index)
			}
		}
	}
	if active > 10000 {
		return nil, ErrDocument
	}
	writeFeed := func(index int) error {
		nodes++
		if nodes > 99980 {
			return ErrDocument
		}
		f := snapshot.Feeds[index]
		visitedFeeds[f.ID] = true
		attrs := []xml.Attr{{Name: xml.Name{Local: "type"}, Value: "rss"}, {Name: xml.Name{Local: "text"}, Value: f.Title}, {Name: xml.Name{Local: "title"}, Value: f.Title}, {Name: xml.Name{Local: "xmlUrl"}, Value: f.NormalizedUrl}}
		if f.SiteUrl != "" {
			attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "htmlUrl"}, Value: f.SiteUrl})
		}
		outline := xml.StartElement{Name: xml.Name{Local: "outline"}, Attr: attrs}
		if err := token(outline); err != nil {
			return err
		}
		return token(outline.End())
	}
	var writeFolder func(string, int) error
	writeFolder = func(parent string, depth int) error {
		if depth > 5 {
			return ErrDocument
		}
		for _, index := range categories[parent] {
			nodes++
			if nodes > 99980 {
				return ErrDocument
			}
			category := snapshot.Categories[index]
			if visitedCategories[category.ID] {
				return ErrDocument
			}
			visitedCategories[category.ID] = true
			outline := xml.StartElement{Name: xml.Name{Local: "outline"}, Attr: []xml.Attr{{Name: xml.Name{Local: "text"}, Value: category.Name}}}
			if err := token(outline); err != nil {
				return err
			}
			for _, index := range feeds[category.ID] {
				if err := writeFeed(index); err != nil {
					return err
				}
			}
			if err := writeFolder(category.ID, depth+1); err != nil {
				return err
			}
			if err := token(outline.End()); err != nil {
				return err
			}
		}
		return nil
	}
	if err := writeFolder("", 0); err != nil {
		return nil, err
	}
	for _, index := range feeds[""] {
		if err := writeFeed(index); err != nil {
			return nil, err
		}
	}
	if len(visitedCategories) != len(snapshot.Categories) || len(visitedFeeds) != active {
		return nil, ErrDocument
	}
	for _, value := range []xml.Token{body.End(), opml.End()} {
		if err := token(value); err != nil {
			return nil, err
		}
	}
	if err := encoder.Flush(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
