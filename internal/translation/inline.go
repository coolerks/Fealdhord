package translation

import (
	"cmp"
	"fmt"
	stdhtml "html"
	"regexp"
	"slices"
	"strings"
	"unicode"

	xhtml "golang.org/x/net/html"
)

type inlineTag struct {
	raw, key string
}

func readInlineTag(raw string, token xhtml.Token) inlineTag {
	switch token.Data {
	case "b":
		token.Data = "strong"
	case "i":
		token.Data = "em"
	case "br":
		if token.Type == xhtml.SelfClosingTagToken {
			token.Type = xhtml.StartTagToken
		}
	}
	slices.SortFunc(token.Attr, func(a, b xhtml.Attribute) int {
		return cmp.Or(cmp.Compare(a.Namespace, b.Namespace), cmp.Compare(a.Key, b.Key), cmp.Compare(a.Val, b.Val))
	})
	return inlineTag{raw: raw, key: token.String()}
}

func isInlineTag(kind xhtml.TokenType) bool {
	return kind == xhtml.StartTagToken || kind == xhtml.EndTagToken || kind == xhtml.SelfClosingTagToken
}

var numericReference = regexp.MustCompile(`&#(?:[xX][0-9A-Fa-f]{1,6}|[0-9]{1,7});`)

func normalizeNumericText(text, originalText string) string {
	return numericReference.ReplaceAllStringFunc(text, func(reference string) string {
		if strings.Contains(originalText, reference) {
			return reference
		}
		decoded := stdhtml.UnescapeString(reference)
		// Leave ASCII syntax, controls and invalid code points alone.
		for _, r := range decoded {
			if r >= '\u00a0' && r != unicode.ReplacementChar &&
				(unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r) ||
					unicode.IsPunct(r) || unicode.IsSymbol(r) || unicode.IsSpace(r)) {
				return decoded
			}
			break
		}
		return reference
	})
}

func hasEscapedNumericText(raw string) bool {
	return strings.Contains(raw, "#") && numericReference.MatchString(stdhtml.UnescapeString(raw))
}

// restoreInline verifies the translated tag sequence and writes the original tags.
// Only text nodes can contribute model-generated content.
func restoreInline(source, translated string) (string, bool) {
	var sourceTags []inlineTag
	var sourceText strings.Builder
	tokens := xhtml.NewTokenizer(strings.NewReader(source))
	for kind := tokens.Next(); kind != xhtml.ErrorToken; kind = tokens.Next() {
		if isInlineTag(kind) {
			raw := string(tokens.Raw())
			sourceTags = append(sourceTags, readInlineTag(raw, tokens.Token()))
		} else if kind == xhtml.TextToken {
			sourceText.Write(tokens.Text())
		}
	}

	var out strings.Builder
	nextTag, protected := 0, 0
	tokens = xhtml.NewTokenizer(strings.NewReader(translated))
	for kind := tokens.Next(); kind != xhtml.ErrorToken; kind = tokens.Next() {
		raw := string(tokens.Raw())
		switch {
		case isInlineTag(kind):
			token := tokens.Token()
			if nextTag == len(sourceTags) || readInlineTag(raw, token).key != sourceTags[nextTag].key {
				return "", false
			}
			if token.Data == "code" || token.Data == "kbd" {
				switch kind {
				case xhtml.StartTagToken:
					protected++
				case xhtml.EndTagToken:
					protected--
				}
			}
			raw = sourceTags[nextTag].raw
			nextTag++
		case kind == xhtml.TextToken && protected == 0:
			text := string(tokens.Text())
			if normalized := normalizeNumericText(text, sourceText.String()); normalized != text {
				raw = stdhtml.EscapeString(normalized)
			}
		}
		out.WriteString(raw)
	}
	return out.String(), nextTag == len(sourceTags)
}

type inlineRetry struct {
	original string
	markers  []string
	tags     []string
	pattern  *regexp.Regexp
}

// useInlineRetry substitutes text markers for HTML tags in the existing bounded repair round.
func useInlineRetry(unit translationUnit) translationUnit {
	stem := "FT"
	plain := stdhtml.UnescapeString(unit.HTML)
	for strings.Contains(plain, "[["+stem) {
		stem += "T"
	}
	retry := &inlineRetry{original: unit.HTML}
	var out strings.Builder
	tokens := xhtml.NewTokenizer(strings.NewReader(unit.HTML))
	for kind := tokens.Next(); kind != xhtml.ErrorToken; kind = tokens.Next() {
		if isInlineTag(kind) {
			marker := fmt.Sprintf("[[%s%d]]", stem, len(retry.tags))
			retry.tags = append(retry.tags, string(tokens.Raw()))
			retry.markers = append(retry.markers, marker)
			out.WriteString(marker)
		} else {
			out.Write(tokens.Raw())
		}
	}
	if len(retry.tags) == 0 {
		return unit
	}
	retry.pattern = regexp.MustCompile(`\[\[` + stem + `[0-9]+\]\]`)
	unit.HTML, unit.retry = out.String(), retry
	return unit
}

func (r *inlineRetry) restore(translated string) (string, bool) {
	var text strings.Builder
	tokens := xhtml.NewTokenizer(strings.NewReader(translated))
	for kind := tokens.Next(); kind != xhtml.ErrorToken; kind = tokens.Next() {
		if kind != xhtml.TextToken {
			return "", false
		}
		text.Write(tokens.Text())
	}
	plain := text.String()
	found := r.pattern.FindAllStringIndex(plain, -1)
	if len(found) != len(r.tags) {
		return "", false
	}
	var out strings.Builder
	start := 0
	for i, part := range found {
		if plain[part[0]:part[1]] != r.markers[i] {
			return "", false
		}
		out.WriteString(stdhtml.EscapeString(plain[start:part[0]]))
		out.WriteString(r.tags[i])
		start = part[1]
	}
	out.WriteString(stdhtml.EscapeString(plain[start:]))
	return out.String(), true
}
