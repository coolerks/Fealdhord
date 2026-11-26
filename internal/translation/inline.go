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

type inlineMarker struct {
	html  string
	group int
}

type inlineFormat struct {
	text    string
	markers []inlineMarker
	indices map[string]int
	pattern *regexp.Regexp
}

// takeInlineAtom consumes a code or keyboard span, including any nested spans of the same type.
func takeInlineAtom(tokens *xhtml.Tokenizer, markup, tag string) string {
	var out strings.Builder
	out.WriteString(markup)
	depth := 1
	for kind := tokens.Next(); kind != xhtml.ErrorToken; kind = tokens.Next() {
		out.Write(tokens.Raw())
		if tokens.Token().Data != tag {
			continue
		}
		if kind == xhtml.StartTagToken {
			depth++
		}
		if kind == xhtml.EndTagToken {
			depth--
			if depth == 0 {
				break
			}
		}
	}
	return out.String()
}

// encodeInline keeps resource attributes local and exposes each inline code span as one marker.
func encodeInline(raw string) *inlineFormat {
	stem, codeStem := "FT", "CODE"
	plain := stdhtml.UnescapeString(raw)
	for strings.Contains(plain, "[["+stem) {
		stem += "T"
	}
	for strings.Contains(plain, "[["+codeStem) {
		codeStem += "E"
	}
	format := &inlineFormat{}
	var out strings.Builder
	var previousAtom string
	codeCount := 0
	tokens := xhtml.NewTokenizer(strings.NewReader(raw))
	for kind := tokens.Next(); kind != xhtml.ErrorToken; kind = tokens.Next() {
		if !isInlineTag(kind) {
			if kind == xhtml.TextToken {
				out.Write(tokens.Text())
			}
			continue
		}
		markup := string(tokens.Raw())
		tag := tokens.Token().Data
		index := len(format.markers)
		group, atom := index, ""
		var marker string
		if kind == xhtml.StartTagToken && (tag == "code" || tag == "kbd") {
			markup = takeInlineAtom(tokens, markup, tag)
			marker = fmt.Sprintf("[[%s%d]]", codeStem, codeCount)
			codeCount++
			atom = tag
			// Only adjacent atoms of the same type can trade places. HTML pairs stay in order.
			if previousAtom == atom {
				group = format.markers[index-1].group
			}
		} else {
			marker = fmt.Sprintf("[[%s%d]]", stem, index)
		}
		previousAtom = atom
		if format.indices == nil {
			format.indices = make(map[string]int)
		}
		format.indices[marker] = index
		format.markers = append(format.markers, inlineMarker{html: markup, group: group})
		out.WriteString(marker)
	}
	format.text = out.String()
	if len(format.markers) > 0 {
		format.pattern = regexp.MustCompile(`\[\[(?:` + stem + "|" + codeStem + `)[0-9]+\]\]`)
	}
	return format
}

func (f *inlineFormat) restore(plain string) (string, bool) {
	if f.pattern == nil {
		return stdhtml.EscapeString(plain), true
	}
	found := f.pattern.FindAllStringIndex(plain, -1)
	if len(found) != len(f.markers) {
		return "", false
	}
	var out strings.Builder
	seen := make([]bool, len(f.markers))
	start := 0
	for i, part := range found {
		index, ok := f.indices[plain[part[0]:part[1]]]
		if !ok || seen[index] || f.markers[index].group != f.markers[i].group {
			return "", false
		}
		seen[index] = true
		out.WriteString(stdhtml.EscapeString(plain[start:part[0]]))
		out.WriteString(f.markers[index].html)
		start = part[1]
	}
	out.WriteString(stdhtml.EscapeString(plain[start:]))
	return out.String(), true
}
