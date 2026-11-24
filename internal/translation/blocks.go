package translation

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/coolerks/Fealdhord/internal/reader"
)

// SourceBlock 表示清洗后文章 HTML 中的一个可翻译段落块。
type SourceBlock struct {
	ID         string
	Tag        string
	InnerHTML  string
	PlainText  string
	SourceHash string
}

// BlockTranslation 表示单个段落块的持久化翻译结果。
type BlockTranslation struct {
	ID             string `json:"id"`
	SourceHash     string `json:"sourceHash"`
	TranslatedHTML string `json:"translatedHtml"`
}

// ExtractBlocks 从 reader.PrepareHTML 生成的安全 HTML 中提取带 data-tr-block 的段落列表。
// 提取时会移除块内 <img>（图片在原 DOM 中保留，不在译文中重复渲染），并计算稳定 SourceHash。
func ExtractBlocks(preparedHTML string) ([]SourceBlock, error) {
	if strings.TrimSpace(preparedHTML) == "" {
		return nil, nil
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<body>" + preparedHTML + "</body>"))
	if err != nil {
		return nil, err
	}
	blocks := make([]SourceBlock, 0, 16)
	doc.Find("[data-tr-block]").Each(func(_ int, node *goquery.Selection) {
		id := strings.TrimSpace(node.AttrOr("data-tr-block", ""))
		if id == "" {
			return
		}
		clone := node.Clone()
		clone.Find("img").Remove()
		plain := strings.TrimSpace(clone.Text())
		if plain == "" {
			return
		}
		innerHTML, err := clone.Html()
		if err != nil {
			return
		}
		innerHTML = strings.TrimSpace(innerHTML)
		if innerHTML == "" {
			return
		}
		tag := goquery.NodeName(node)
		sum := sha256.Sum256([]byte(tag + "\x00" + innerHTML))
		blocks = append(blocks, SourceBlock{
			ID:         id,
			Tag:        tag,
			InnerHTML:  innerHTML,
			PlainText:  plain,
			SourceHash: hex.EncodeToString(sum[:16]),
		})
	})
	return blocks, nil
}

// SanitizeTranslatedHTML 对模型返回的段落译文进行严格的行内安全清洗。
func SanitizeTranslatedHTML(raw string) string {
	return strings.TrimSpace(reader.SanitizeInlineTranslationHTML(raw))
}
