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
	"github.com/rivo/uniseg"
	xhtml "golang.org/x/net/html"
)

const (
	defaultBatchKiB = 50
)

var errTranslationBudget = &ai.ModelError{Code: "ContextWindowExceeded"}

type translationUnit struct {
	ID, HTML                          string
	prefix, suffix, leading, trailing string
	code                              []protectedCode
	format                            *inlineFormat
}

type protectedCode struct{ marker, original string }

type plannedBlock struct {
	source SourceBlock
	units  []translationUnit
	key    string
}

type translationItem struct {
	ID   string `json:"id"`
	Text string `json:"text"`
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
	batchBytes                         int
}

func (p batchPlanner) byteLimit() int {
	if p.batchBytes > 0 {
		return p.batchBytes
	}
	return defaultBatchKiB << 10
}

func (p batchPlanner) request(units []translationUnit, title bool, section string) (*ai.ModelRequest, bool) {
	payload := translationPayload{ArticleTitle: truncateRunes(p.title, 256), Context: section, Items: make([]translationItem, 0, len(units))}
	output := 32
	bytes := 0
	for _, unit := range units {
		text := unit.wireText()
		payload.Items = append(payload.Items, translationItem{ID: unit.ID, Text: text})
		bytes += len(text)
		output += (ai.EstimateTokens(text)*3+1)/2 + 14
		if bytes > p.byteLimit() || output > p.budget.OutputReserve {
			return nil, false
		}
	}
	if title {
		payload.Title = truncateRunes(p.title, 512)
		bytes += len(payload.Title)
		output += (ai.EstimateTokens(payload.Title)*3+1)/2 + 14
	}
	if bytes > p.byteLimit() || output > p.budget.OutputReserve {
		return nil, false
	}
	body, _ := json.Marshal(payload)
	output = max(min(256, p.budget.OutputReserve), output)
	req := &ai.ModelRequest{
		Model: p.model, System: p.system, Budget: p.budget, MaxOutputTokens: &output,
		Messages: []ai.Message{{Role: "user", Content: []ai.ContentBlock{{Type: "text", Text: string(body)}}}},
	}
	return req, ai.EstimateRequest(*req) <= p.budget.InputLimit
}

func (p batchPlanner) group(units []translationUnit, needTitle bool, contextFor func(translationUnit) string, startsSection func(translationUnit) bool) [][]translationUnit {
	var batches [][]translationUnit
	for len(units) > 0 {
		title := needTitle && len(batches) == 0
		hint := contextFor(units[0])
		fits := func(n int) bool {
			_, ok := p.request(units[:n], title, hint)
			return ok
		}
		// Search within the byte and token budgets without repeatedly serializing every prefix.
		low, high := 0, len(units)
		for low < high {
			middle := low + (high-low+1)/2
			if fits(middle) {
				low = middle
			} else {
				high = middle - 1
			}
		}
		n := low
		if n == 0 && title {
			batches = append(batches, nil)
			continue
		}
		// Units were checked without optional context during splitting.
		n = max(1, n)
		if n < len(units) {
			for boundary := n; boundary > n/2; boundary-- {
				if startsSection(units[boundary]) {
					n = boundary
					break
				}
			}
		}
		batches = append(batches, units[:n])
		units = units[n:]
	}
	if len(batches) == 0 && needTitle {
		batches = append(batches, nil)
	}
	return batches
}

func (p batchPlanner) split(block SourceBlock) (plannedBlock, error) {
	raw, code := protectCode(SanitizeTranslatedHTML(block.InnerHTML))
	whole := translationUnit{ID: block.ID, HTML: raw, code: code}
	units := []translationUnit{whole}
	if _, fits := p.request(units, false, ""); !fits {
		var err error
		limit := min(p.byteLimit(), p.budget.InputLimit*3, p.budget.OutputReserve*2)
		units, err = splitInline(raw, block.ID, limit, func(unit translationUnit) bool {
			_, fits := p.request([]translationUnit{unit}, false, "")
			return fits
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
	fmt.Fprintf(hash, "v2\x00%s\x00%s\x00%s\x00%s", block.SourceHash, p.model, p.modelVersion, p.system)
	for i := range units {
		units[i].format = encodeInline(units[i].HTML)
		fmt.Fprintf(hash, "\x00%s", units[i].HTML)
	}
	return plannedBlock{source: block, units: units, key: hex.EncodeToString(hash.Sum(nil))}, nil
}

func protectCode(raw string) (string, []protectedCode) {
	var protected []protectedCode
	stem := "[[FC"
	for strings.Contains(stdhtml.UnescapeString(raw), stem) {
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
		original := takeInlineAtom(z, markup, token.Data)
		marker := fmt.Sprintf("%s%d]]", stem, len(protected))
		protected = append(protected, protectedCode{marker, original})
		out.WriteString("<" + token.Data + ">" + marker + "</" + token.Data + ">")
	}
	return out.String(), protected
}

type openInline struct{ start, end string }

// splitInline closes and reopens inline tags at each split so every unit can be validated separately.
func splitInline(raw, id string, textLimit int, fits func(translationUnit) bool) ([]translationUnit, error) {
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
				markup = takeInlineAtom(z, markup, token.Data)
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
			remaining := token.Data
			state := -1
			for remaining != "" {
				sentence, rest, next := uniseg.FirstSentenceInString(remaining, state)
				state, remaining = next, rest
				fitsText := func(text string) bool {
					return len(text) <= textLimit && fits(translationUnit{ID: nextID(), HTML: body.String() + stdhtml.EscapeString(text) + suffix()})
				}
				for sentence != "" {
					if fitsText(sentence) {
						body.WriteString(stdhtml.EscapeString(sentence))
						hasText = hasText || strings.TrimSpace(sentence) != ""
						break
					}
					if hasText {
						if err := finish(); err != nil {
							return nil, err
						}
						continue
					}
					n := fittedTextBoundary(sentence, textLimit, fitsText)
					if n == 0 {
						return nil, errTranslationBudget
					}
					body.WriteString(stdhtml.EscapeString(sentence[:n]))
					hasText = true
					sentence = sentence[n:]
					if err := finish(); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	if err := z.Err(); !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(open) != 0 {
		return nil, errors.New("unbalanced source inline HTML")
	}
	if hasText || len(units) == 0 || body.String() != prefix {
		if err := finish(); err != nil {
			return nil, err
		}
	}
	return units, nil
}

// fittedTextBoundary prefers a clause, then a word, then a complete grapheme.
func fittedTextBoundary(text string, ceiling int, fits func(string) bool) int {
	graphemes := uniseg.NewGraphemes(text)
	var ends []int
	var words, clauses []int
	for graphemes.Next() {
		_, end := graphemes.Positions()
		if end > ceiling {
			break
		}
		ends = append(ends, end)
		if graphemes.IsWordBoundary() {
			words = append(words, end)
		}
		if strings.ContainsAny(graphemes.Str(), ",;，；、،؛") {
			clauses = append(clauses, end)
		}
	}
	i, j := 0, len(ends)
	for i < j {
		middle := i + (j-i)/2
		if fits(text[:ends[middle]]) {
			i = middle + 1
		} else {
			j = middle
		}
	}
	if i == 0 {
		return 0
	}
	limit := ends[i-1]
	for _, candidates := range [][]int{clauses, words} {
		for k := len(candidates) - 1; k >= 0; k-- {
			if candidates[k] <= limit && candidates[k] > limit/2 {
				return candidates[k]
			}
		}
	}
	return limit
}

func (unit translationUnit) wireText() string {
	if unit.format != nil {
		return unit.format.text
	}
	return encodeInline(unit.HTML).text
}

var codeMarkup = regexp.MustCompile(`<(?:code|kbd)>[^<]+</(?:code|kbd)>`)

func validateUnit(unit translationUnit, raw string) (string, bool) {
	source := unit.HTML
	var clean string
	if unit.format != nil {
		var ok bool
		clean, ok = unit.format.restore(raw)
		if !ok {
			return "", false
		}
	} else {
		clean = SanitizeTranslatedHTML(raw)
	}
	clean, matches := restoreInline(source, clean)
	if clean == "" || !matches ||
		(hasTranslatableText(source) && !hasTranslatableText(clean)) ||
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
