package translation

import (
	"fmt"
	stdhtml "html"
	"regexp"
	"strconv"
	"strings"

	xhtml "golang.org/x/net/html"
)

// mtHTML 实现模式 A（原生 HTML 桥接）：
// - 将 <code>/<kbd> 整块替换为 <span translate="no" class="notranslate">C0</span>，防止代码被翻译；
// - 将带属性的开始标签（如 <a href="...">）精简为 <a id="0">，降低计费字符并保护属性不被篡改；
// - 还原后统一经过 SanitizeTranslatedHTML。
type mtHTML struct {
	encoded string
	atoms   []string
	tags    []string
}

var (
	mtCodeSpanPattern = regexp.MustCompile(`(?i)<span\b[^>]*>\s*C(\d+)\s*</span>`)
	mtTagIDPattern    = regexp.MustCompile(`(?i)<([a-z][a-z0-9]*)\b[^>]*\bid\s*=\s*["']?T(\d+)["']?[^>]*>`)
)

func encodeHTMLForMT(raw string) *mtHTML {
	h := &mtHTML{}
	var out strings.Builder
	tokens := xhtml.NewTokenizer(strings.NewReader(raw))
	for kind := tokens.Next(); kind != xhtml.ErrorToken; kind = tokens.Next() {
		rawTok := string(tokens.Raw())
		if !isInlineTag(kind) {
			out.WriteString(rawTok)
			continue
		}
		tok := tokens.Token()
		if kind == xhtml.StartTagToken && (tok.Data == "code" || tok.Data == "kbd") {
			atom := takeInlineAtom(tokens, rawTok, tok.Data)
			idx := len(h.atoms)
			h.atoms = append(h.atoms, atom)
			fmt.Fprintf(&out, `<span translate="no" class="notranslate">C%d</span>`, idx)
			continue
		}
		if kind == xhtml.StartTagToken && len(tok.Attr) > 0 {
			idx := len(h.tags)
			h.tags = append(h.tags, rawTok)
			fmt.Fprintf(&out, `<%s id="T%d">`, tok.Data, idx)
			continue
		}
		out.WriteString(rawTok)
	}
	h.encoded = out.String()
	return h
}

func (h *mtHTML) restore(translated string) (string, bool) {
	if strings.TrimSpace(translated) == "" {
		return "", false
	}
	atomSeen := make([]bool, len(h.atoms))
	out := mtCodeSpanPattern.ReplaceAllStringFunc(translated, func(match string) string {
		sub := mtCodeSpanPattern.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		idx, err := strconv.Atoi(sub[1])
		if err != nil || idx < 0 || idx >= len(h.atoms) {
			return match
		}
		atomSeen[idx] = true
		return h.atoms[idx]
	})
	tagSeen := make([]bool, len(h.tags))
	out = mtTagIDPattern.ReplaceAllStringFunc(out, func(match string) string {
		sub := mtTagIDPattern.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		idx, err := strconv.Atoi(sub[2])
		if err != nil || idx < 0 || idx >= len(h.tags) {
			return match
		}
		tagSeen[idx] = true
		return h.tags[idx]
	})
	allMatched := true
	for _, seen := range atomSeen {
		if !seen {
			allMatched = false
			break
		}
	}
	if allMatched {
		for _, seen := range tagSeen {
			if !seen {
				allMatched = false
				break
			}
		}
	}
	cleaned := SanitizeTranslatedHTML(out)
	return cleaned, allMatched && cleaned != ""
}

// mtText 实现模式 B（抗干扰占位符纯文本桥接）：
// 适用于纯文本接口（Edge 免费、彩云、百度、有道文本批量）及模式 A 降级。
type mtText struct {
	encoded string
	markers []string
	// hasTags 表示原文是否包含行内 HTML 标签或代码原子。
	hasTags bool
}

// 宽松匹配机翻可能产生的空格或全角括号变形，例如 "⟦ C0 ⟧" / "【C0】" / "[[C0]]"。
var mtPlaceholderPattern = regexp.MustCompile(`(?:⟦|\[\[|【)\s*([CT])(\d+)\s*(?:⟧|\]\]|】)`)

func encodeTextForMT(raw string) *mtText {
	m := &mtText{}
	var out strings.Builder
	tokens := xhtml.NewTokenizer(strings.NewReader(raw))
	for kind := tokens.Next(); kind != xhtml.ErrorToken; kind = tokens.Next() {
		if !isInlineTag(kind) {
			if kind == xhtml.TextToken {
				out.Write(tokens.Text())
			}
			continue
		}
		m.hasTags = true
		rawTok := string(tokens.Raw())
		tok := tokens.Token()
		idx := len(m.markers)
		if kind == xhtml.StartTagToken && (tok.Data == "code" || tok.Data == "kbd") {
			atom := takeInlineAtom(tokens, rawTok, tok.Data)
			m.markers = append(m.markers, atom)
			fmt.Fprintf(&out, "⟦C%d⟧", idx)
		} else {
			m.markers = append(m.markers, rawTok)
			fmt.Fprintf(&out, "⟦T%d⟧", idx)
		}
	}
	m.encoded = out.String()
	return m
}

// restore 尝试完整还原所有行内标记；当 exact=false 时仍返回尽最大努力清洗后的安全译文。
func (m *mtText) restore(translated string) (htmlOut string, exact bool) {
	if strings.TrimSpace(translated) == "" {
		return "", false
	}
	if !m.hasTags {
		cleaned := SanitizeTranslatedHTML(stdhtml.EscapeString(translated))
		return cleaned, cleaned != ""
	}
	matches := mtPlaceholderPattern.FindAllStringSubmatchIndex(translated, -1)
	seen := make([]bool, len(m.markers))
	var out strings.Builder
	cursor := 0
	allValid := len(matches) == len(m.markers)
	for _, loc := range matches {
		idx, err := strconv.Atoi(translated[loc[4]:loc[5]])
		if err != nil || idx < 0 || idx >= len(m.markers) || seen[idx] {
			allValid = false
			continue
		}
		seen[idx] = true
		out.WriteString(stdhtml.EscapeString(translated[cursor:loc[0]]))
		out.WriteString(m.markers[idx])
		cursor = loc[1]
	}
	out.WriteString(stdhtml.EscapeString(translated[cursor:]))
	for _, ok := range seen {
		if !ok {
			allValid = false
			break
		}
	}
	if allValid {
		cleaned := SanitizeTranslatedHTML(out.String())
		return cleaned, cleaned != ""
	}
	// 降级：保留成功还原的代码原子并剥离残余占位符，确保译文可读且安全。
	fallback := mtPlaceholderPattern.ReplaceAllStringFunc(translated, func(s string) string {
		sub := mtPlaceholderPattern.FindStringSubmatch(s)
		if len(sub) >= 3 && sub[1] == "C" {
			if idx, err := strconv.Atoi(sub[2]); err == nil && idx >= 0 && idx < len(m.markers) {
				return m.markers[idx]
			}
		}
		return ""
	})
	cleaned := SanitizeTranslatedHTML(fallback)
	return cleaned, false
}
