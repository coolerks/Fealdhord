package feed

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// SanitizeBody 保留可读结构和规范化资源地址，仅供 Go 缓存；前端展示必须再改写成本地资源 ID。
func SanitizeBody(raw, base string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(raw))
	if err != nil {
		return ""
	}
	doc.Find("script,style,iframe,object,embed,svg,math,form,link,meta,base,audio,video,source").Remove()
	doc.Find("a").Each(func(_ int, node *goquery.Selection) {
		target := ResolveURL(node.AttrOr("href", ""), base)
		if target == "" {
			node.RemoveAttr("href")
		} else {
			node.SetAttr("href", target)
		}
	})
	doc.Find("img").Each(func(_ int, node *goquery.Selection) {
		source := node.AttrOr("src", "")
		if source == "" {
			source = node.AttrOr("data-src", node.AttrOr("data-original", ""))
		}
		if selected := sourceSet(node.AttrOr("srcset", node.AttrOr("data-srcset", ""))); selected != "" {
			source = selected
		}
		target := ResolveURL(source, base)
		if target == "" || excludedImage(target) || declaredSmall(node.AttrOr("width", ""), node.AttrOr("height", "")) {
			node.Remove()
			return
		}
		node.SetAttr("src", target)
	})
	body, err := doc.Find("body").Html()
	if err != nil {
		return ""
	}
	return cleanHTML.Sanitize(body)
}
