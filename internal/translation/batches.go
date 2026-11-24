package translation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"regexp"
	"strings"
	"unicode"

	"github.com/coolerks/Fealdhord/internal/ai"
	xhtml "golang.org/x/net/html"
)

const (
	firstBatchTokens = 350
	bodyBatchTokens  = 1200
	maxBatchItems    = 10
	maxBodyOutput    = 4096
)

var errTranslationBudget = &ai.ModelError{Code: "ContextWindowExceeded"}

type translationUnit struct {
	ID, HTML                          string
	prefix, suffix, leading, trailing string
	code                              []protectedCode
}

type protectedCode struct{ marker, original string }

type plannedBlock struct {
	source SourceBlock
	units  []translationUnit
	key    string
}

type translationItem struct {
	ID   string `json:"id"`
	HTML string `json:"html"`
}

type translationPayload struct {
	ArticleTitle string            `json:"article_title"`
	Title        string            `json:"title,omitempty"`
	Context      string            `json:"context,omitempty"`
	Items        []translationItem `json:"items"`
}

type batchPlanner struct {
	model, modelVersion, system, title string
	budget                             ai.Budget
}

func (p batchPlanner) request(units []translationUnit, title bool, section string) (*ai.ModelRequest, bool) {
	items := make([]translationItem, 0, len(units))
	payload := translationPayload{ArticleTitle: truncateRunes(p.title, 256), Context: section, Items: items}
	output := 64
	for _, unit := range units {
		payload.Items = append(payload.Items, translationItem{ID: unit.ID, HTML: unit.HTML})
		output += (ai.EstimateTokens(unit.HTML)*3+1)/2 + 24
	}
	if title {
		payload.Title = truncateRunes(p.title, 512)
		output += (ai.EstimateTokens(payload.Title)*3+1)/2 + 24
	}
	maximumOutput := min(maxBodyOutput, p.budget.OutputReserve)
	output = max(min(256, maximumOutput), output)
	if output > maximumOutput {
		return nil, false
	}
	body, _ := json.Marshal(payload)
	req := &ai.ModelRequest{
		Model: p.model, System: p.system, Budget: p.budget, MaxOutputTokens: &output,
		Messages: []ai.Message{{Role: "user", Content: []ai.ContentBlock{{Type: "text", Text: string(body)}}}},
	}
	return req, ai.EstimateRequest(*req) <= p.budget.InputLimit
}

func (p batchPlanner) fits(units []translationUnit, title bool, section string, tokenTarget, maxItems int) bool {
	if len(units) > maxItems {
		return false
	}
	var tokens int
	for _, unit := range units {
		tokens += ai.EstimateTokens(unit.HTML)
	}
	if tokens > tokenTarget {
		return false
	}
	_, fits := p.request(units, title, section)
	return fits
}

func (p batchPlanner) group(units []translationUnit, needTitle bool, contextFor func(translationUnit) string) [][]translationUnit {
	var batches [][]translationUnit
	for len(units) > 0 {
		n := 0
		title := needTitle && len(batches) == 0
		for n < len(units) && p.fits(units[:n+1], title, contextFor(units[0]), bodyBatchTokens, maxBatchItems) {
			n++
		}
		if n == 0 && title {
			batches = append(batches, nil)
			continue
		}
		if n == 0 {
			// A single unit was already fitted without optional reading context.
			n = 1
		}
		batches = append(batches, units[:n])
		units = units[n:]
	}
	if len(batches) == 0 && needTitle {
		batches = append(batches, nil)
	}
	return batches
}

func (p batchPlanner) firstBatch(units []translationUnit, start string, needsTitle bool, contextFor func(translationUnit) string) ([]translationUnit, []translationUnit) {
	index := 0
	for i, unit := range units {
		if unit.ID == start || strings.HasPrefix(unit.ID, start+".") {
			index = i
			break
		}
	}
	var first, remainder []translationUnit
	seenBlocks := 0
	lastBlock := ""
	for i, unit := range units {
		block, _, fragment := strings.Cut(unit.ID, ".")
		if i >= index && block != lastBlock {
			seenBlocks++
			lastBlock = block
		}
		if i >= index && seenBlocks <= maxBatchItems && !fragment &&
			p.fits(append(first, unit), needsTitle, contextFor(unit), firstBatchTokens, 3) {
			first = append(first, unit)
		} else {
			remainder = append(remainder, unit)
		}
	}
	return first, remainder
}

func (p batchPlanner) split(block SourceBlock) (plannedBlock, error) {
	raw, code := protectCode(SanitizeTranslatedHTML(block.InnerHTML), block.SourceHash)
	whole := translationUnit{ID: block.ID, HTML: raw, code: code}
	units := []translationUnit{whole}
	if !p.fits(units, false, "", bodyBatchTokens, 1) {
		var err error
		units, err = splitInline(raw, block.ID, func(unit translationUnit) bool {
			return p.fits([]translationUnit{unit}, false, "", bodyBatchTokens, 1)
		})
		if err != nil {
			return plannedBlock{}, err
		}
		for i := range units {
			for _, entry := range code {
				if strings.Contains(units[i].HTML, entry.marker) {
					units[i].code = append(units[i].code, entry)
				}
			}
		}
	}
	hash := sha256.New()
	fmt.Fprintf(hash, "v1\x00%s\x00%s\x00%s\x00%s", block.SourceHash, p.model, p.modelVersion, p.system)
	for _, unit := range units {
		fmt.Fprintf(hash, "\x00%s", unit.HTML)
	}
	return plannedBlock{source: block, units: units, key: hex.EncodeToString(hash.Sum(nil))}, nil
}

func protectCode(raw, hash string) (string, []protectedCode) {
	var protected []protectedCode
	stem := "FealdhordCode" + hash + "x"
	for strings.Contains(raw, stem) {
		stem += "x"
	}
	z := xhtml.NewTokenizer(strings.NewReader(raw))
	var out strings.Builder
	for kind := z.Next(); kind != xhtml.ErrorToken; kind = z.Next() {
		markup := string(z.Raw())
		token := z.Token()
		if kind != xhtml.StartTagToken || (token.Data != "code" && token.Data != "kbd") {
			out.WriteString(markup)
			continue
		}
		var original strings.Builder
		original.WriteString(markup)
		for next := z.Next(); next != xhtml.ErrorToken; next = z.Next() {
			original.Write(z.Raw())
			end := z.Token()
			if next == xhtml.EndTagToken && end.Data == token.Data {
				break
			}
		}
		marker := fmt.Sprintf("%s%dZ", stem, len(protected))
		protected = append(protected, protectedCode{marker, original.String()})
		out.WriteString("<" + token.Data + ">" + marker + "</" + token.Data + ">")
	}
	return out.String(), protected
}

type openInline struct{ start, end string }

// splitInline closes and reopens inline tags at each split so every unit can be validated separately.
func splitInline(raw, id string, fits func(translationUnit) bool) ([]translationUnit, error) {
	var units []translationUnit
	var body strings.Builder
	var open []openInline
	prefix := ""
	hasText := false
	z := xhtml.NewTokenizer(strings.NewReader(raw))
	suffix := func() string {
		var result strings.Builder
		for i := len(open) - 1; i >= 0; i-- {
			result.WriteString(open[i].end)
		}
		return result.String()
	}
	nextID := func() string { return fmt.Sprintf("%s.%d", id, len(units)) }
	finish := func() error {
		closing := suffix()
		inner := strings.TrimPrefix(body.String(), prefix)
		leading := inner[:len(inner)-len(strings.TrimLeftFunc(inner, unicode.IsSpace))]
		trailing := inner[len(strings.TrimRightFunc(inner, unicode.IsSpace)):]
		unit := translationUnit{ID: nextID(), HTML: body.String() + closing, prefix: prefix, suffix: closing, leading: leading, trailing: trailing}
		if !fits(unit) {
			return errTranslationBudget
		}
		units = append(units, unit)
		starts := make([]string, 0, len(open))
		for _, tag := range open {
			starts = append(starts, tag.start)
		}
		prefix = strings.Join(starts, "")
		body.Reset()
		body.WriteString(prefix)
		hasText = false
		return nil
	}
	for kind := z.Next(); kind != xhtml.ErrorToken; kind = z.Next() {
		markup := string(z.Raw())
		token := z.Token()
		switch kind {
		case xhtml.StartTagToken:
			if token.Data == "code" || token.Data == "kbd" {
				for next := z.Next(); next != xhtml.ErrorToken; next = z.Next() {
					markup += string(z.Raw())
					end := z.Token()
					if next == xhtml.EndTagToken && end.Data == token.Data {
						break
					}
				}
				if hasText && !fits(translationUnit{ID: nextID(), HTML: body.String() + markup + suffix()}) {
					if err := finish(); err != nil {
						return nil, err
					}
				}
				body.WriteString(markup)
				hasText = true
				continue
			}
			if token.Data != "br" {
				open = append(open, openInline{markup, "</" + token.Data + ">"})
			}
			body.WriteString(markup)
		case xhtml.EndTagToken:
			if len(open) == 0 || open[len(open)-1].end != "</"+token.Data+">" {
				return nil, errors.New("unbalanced source inline HTML")
			}
			open = open[:len(open)-1]
			body.WriteString(markup)
		case xhtml.SelfClosingTagToken:
			body.WriteString(markup)
		case xhtml.TextToken:
			for text := []rune(token.Data); len(text) > 0; {
				n := min(len(text), 96)
				for j := 0; j < n; j++ {
					if strings.ContainsRune("。！？.!?；;\n", text[j]) &&
						(j+1 == len(text) || text[j] > 127 || unicode.IsSpace(text[j+1])) {
						n = j + 1
						break
					}
				}
				if n == 96 && len(text) > n && !unicode.IsSpace(text[n]) {
					for j := n - 1; j >= n/2; j-- {
						if unicode.IsSpace(text[j]) {
							n = j + 1
							break
						}
					}
				}
				for n > 0 && !fits(translationUnit{ID: nextID(), HTML: body.String() + stdhtml.EscapeString(string(text[:n])) + suffix()}) {
					if hasText {
						if err := finish(); err != nil {
							return nil, err
						}
						break
					}
					n /= 2
				}
				if n == 0 {
					return nil, errTranslationBudget
				}
				if !fits(translationUnit{ID: nextID(), HTML: body.String() + stdhtml.EscapeString(string(text[:n])) + suffix()}) {
					continue
				}
				body.WriteString(stdhtml.EscapeString(string(text[:n])))
				hasText = hasText || strings.TrimSpace(string(text[:n])) != ""
				text = text[n:]
			}
		}
	}
	if err := z.Err(); !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(open) != 0 {
		return nil, errors.New("unbalanced source inline HTML")
	}
	if err := finish(); err != nil {
		return nil, err
	}
	return units, nil
}

var codeMarkup = regexp.MustCompile(`<(?:code|kbd)>[^<]+</(?:code|kbd)>`)

func validateUnit(unit translationUnit, raw string) (string, bool) {
	clean := SanitizeTranslatedHTML(raw)
	if clean == "" || inlineSignature(unit.HTML) != inlineSignature(clean) ||
		(hasTranslatableText(unit.HTML) && !hasTranslatableText(clean)) ||
		!strings.HasPrefix(clean, unit.prefix) || !strings.HasSuffix(clean, unit.suffix) {
		return "", false
	}
	for _, entry := range unit.code {
		if strings.Count(clean, entry.marker) != 1 {
			return "", false
		}
		found := false
		clean = codeMarkup.ReplaceAllStringFunc(clean, func(markup string) string {
			if strings.Contains(markup, ">"+entry.marker+"<") {
				found = true
				return entry.original
			}
			return markup
		})
		if !found {
			return "", false
		}
	}
	if len(unit.prefix)+len(unit.suffix) >= len(clean) {
		return "", false
	}
	result := clean[len(unit.prefix) : len(clean)-len(unit.suffix)]
	if strings.TrimSpace(result) == "" {
		return "", false
	}
	if unit.leading != "" {
		result = unit.leading + strings.TrimLeftFunc(result, unicode.IsSpace)
	}
	if unit.trailing != "" {
		result = strings.TrimRightFunc(result, unicode.IsSpace) + unit.trailing
	}
	return result, true
}

func hasTranslatableText(raw string) bool {
	z := xhtml.NewTokenizer(strings.NewReader(raw))
	protected := false
	for kind := z.Next(); kind != xhtml.ErrorToken; kind = z.Next() {
		token := z.Token()
		if token.Data == "code" || token.Data == "kbd" {
			if kind == xhtml.StartTagToken || kind == xhtml.EndTagToken {
				protected = kind == xhtml.StartTagToken
			}
		}
		if kind == xhtml.TextToken && !protected && strings.TrimSpace(token.Data) != "" {
			return true
		}
	}
	return false
}

func inlineSignature(raw string) string {
	var tags strings.Builder
	z := xhtml.NewTokenizer(strings.NewReader(raw))
	for kind := z.Next(); kind != xhtml.ErrorToken; kind = z.Next() {
		if kind != xhtml.StartTagToken && kind != xhtml.EndTagToken && kind != xhtml.SelfClosingTagToken {
			continue
		}
		token := z.Token()
		if token.Data == "br" && kind == xhtml.SelfClosingTagToken {
			token.Type = xhtml.StartTagToken
		}
		// Attributes originate from the reader, so extra or changed attributes invalidate the item.
		tags.WriteString(token.String())
	}
	return tags.String()
}
