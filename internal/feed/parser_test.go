package feed

import (
	"context"
	"errors"
	"github.com/coolerks/Fealdhord/internal/netpolicy"
	"net/http"
	"strings"
	"testing"
)

func TestThreeFormatsAndSafeContent(t *testing.T) {
	samples := []struct{ name, body string }{
		{"rss", `<rss version="2.0"><channel><title>订阅</title><link>https://example.org/</link><description>摘要</description><item><guid>one</guid><title>文章</title><link>/one</link><pubDate>bad date</pubDate><description><![CDATA[<p>正文</p><script>attack()</script><img src="/cover.jpg" onerror="attack()"/><iframe src="https://evil.org"></iframe>]]></description></item></channel></rss>`},
		{"atom", `<feed xmlns="http://www.w3.org/2005/Atom"><title>订阅</title><id>feed</id><updated>2026-11-05T00:00:00Z</updated><entry><id>one</id><title>文章</title><link href="/one"/><content type="html">&lt;p&gt;正文&lt;/p&gt;</content></entry></feed>`},
		{"json", `{"version":"https://jsonfeed.org/version/1.1","title":"订阅","items":[{"id":"one","url":"/one","title":"文章","content_html":"<p>正文</p>","image":"/image.jpg","banner_image":"/banner.jpg"}]}`},
	}
	for _, sample := range samples {
		t.Run(sample.name, func(t *testing.T) {
			doc, err := Parse(context.Background(), []byte(sample.body), "https://example.org/feed")
			if err != nil {
				t.Fatal(err)
			}
			if len(doc.Items) != 1 || doc.Items[0].URL != "https://example.org/one" || !strings.Contains(doc.Items[0].Text, "正文") {
				t.Fatalf("规范化不正确 %+v", doc)
			}
			if strings.Contains(doc.Items[0].HTML, "script") || strings.Contains(doc.Items[0].HTML, "onerror") || strings.Contains(doc.Items[0].Text, "attack") {
				t.Fatal("危险正文未清理")
			}
			if sample.name == "json" && len(doc.Items[0].Images) != 2 {
				t.Fatalf("JSON 图片候选丢失 %+v", doc.Items[0].Images)
			}
		})
	}
	doc, err := Parse(context.Background(), []byte(`{"version":"https://jsonfeed.org/version/1.1","title":"纯文本 <name>","items":[{"id":"one","content_text":"字面 <code> 与 & 符号"}]}`), "https://example.org/feed")
	if err != nil || doc.Items[0].Text != "字面 <code> 与 & 符号" || doc.Items[0].Title != "无标题" || doc.Title != "纯文本 <name>" {
		t.Fatalf("纯文本或缺标题回退错误 %+v %v", doc, err)
	}
}
func TestMalformedLimitsAndCancellation(t *testing.T) {
	for _, body := range []string{`<rss><channel>`, `<!DOCTYPE rss [<!ENTITY x "secret">]><rss/>`, strings.Repeat("<a>", 129) + strings.Repeat("</a>", 129), `{"version":`} {
		if _, err := Parse(context.Background(), []byte(body), "https://example.org/"); !errors.Is(err, ErrParse) {
			t.Fatalf("危险／畸形源未拒绝 %v", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Parse(ctx, []byte("bad"), "https://example.org/"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

type fakeFetch struct{ body string }

func (f fakeFetch) Fetch(context.Context, string, netpolicy.Request) (netpolicy.Response, error) {
	return netpolicy.Response{URL: "https://example.org/page", Status: http.StatusOK, Body: []byte(f.body)}, nil
}
func TestDiscoveryExplicitCandidates(t *testing.T) {
	body := `<html><head><link rel="alternate" type="application/rss+xml" href="/rss" title="RSS"><link rel="alternate" type="application/atom+xml" href="atom"><link rel="alternate" type="application/rss+xml" href="http://127.0.0.1/private"><link rel="alternate" type="application/rss+xml" href="/rss"></head></html>`
	found, err := Discover(context.Background(), fakeFetch{body}, "https://example.org/page")
	if err != nil || len(found) != 2 || found[0].URL != "https://example.org/rss" {
		t.Fatalf("发现不正确 %+v %v", found, err)
	}
	if _, err := Discover(context.Background(), fakeFetch{"<html>无订阅</html>"}, "https://example.org/page"); !errors.Is(err, ErrNoFeed) {
		t.Fatal(err)
	}
}
func TestXMLBaseSrcsetAndNoFabricatedLinks(t *testing.T) {
	body := `<feed xmlns="http://www.w3.org/2005/Atom" xml:base="https://example.org/root/"><title>T</title><id>feed</id><entry xml:base="posts/"><id>one</id><link href="one"/><content type="html">&lt;img src="track.gif" width="1" height="1"/&gt;&lt;img srcset="small.jpg 200w, medium.jpg 480w, large.jpg 2000w" data-src="fallback.jpg"/&gt;</content></entry><entry><id>two</id><content type="text">no link</content></entry></feed>`
	doc, err := Parse(context.Background(), []byte(body), "https://example.org/rss")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Items[0].URL != "https://example.org/root/posts/one" || len(doc.Items[0].Images) < 1 || doc.Items[0].Images[0].URL != "https://example.org/root/posts/medium.jpg" {
		t.Fatalf("基址或 srcset 错误 %+v", doc.Items[0])
	}
	if doc.Items[1].URL != "" || len(doc.Items[1].Images) != 0 {
		t.Fatal("伪造了缺失 URL 或图片")
	}
}
