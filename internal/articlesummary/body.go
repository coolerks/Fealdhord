package articlesummary

import (
	"math/bits"
	"regexp"
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"github.com/coolerks/Fealdhord/internal/feed"
)

var conditionPattern = regexp.MustCompile(`(?i)生效|适用|除外|例外|截止|自[0-9０-９]|仅限|不适用|\b(effective|eligib\w*|except\w*|deadline|applicable|only if|unless)\b`)
var continuationPattern = regexp.MustCompile(`(?i)(?:\.{3}|…|阅读原文|阅读全文|查看全文|read more|continue reading)\s*[»→。.!]*\s*$`)

func readableBody(raw, fallback string) (string, bool) {
	if strings.TrimSpace(raw) == "" {
		return strings.TrimSpace(fallback), denseConditions(fallback)
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(raw))
	if err != nil {
		return strings.TrimSpace(fallback), denseConditions(fallback)
	}
	root := doc.Find("body")
	root.Find("pre,script,style,iframe,object,template,svg,nav,footer,aside").Remove()
	dense := root.Find("li").FilterFunction(func(_ int, item *goquery.Selection) bool {
		return item.ChildrenFiltered("ul,ol").Length() == 0 && strings.TrimSpace(item.Text()) != ""
	}).Length() >= 3 || root.Find("tbody tr,table > tr").Length() >= 3
	root.Find("th,td,dt,dd").AppendHtml(" ")
	cleaned, err := root.Html()
	if err != nil {
		return strings.TrimSpace(fallback), denseConditions(fallback)
	}
	text := feed.Text(cleaned)
	return text, dense || denseConditions(text)
}

func denseConditions(text string) bool {
	paragraphs := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' })
	matches := 0
	for _, paragraph := range paragraphs {
		if conditionPattern.MatchString(paragraph) {
			matches++
		}
	}
	return matches >= 2
}

func incomplete(text string) bool { return continuationPattern.MatchString(text) }

func withinLimits(text string, hanLimit, englishLimit int64) bool {
	var han, words uint64
	inWord := false
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Han, r):
			han++
			inWord = false
		case unicode.Is(unicode.Latin, r) || unicode.IsDigit(r):
			if !inWord {
				words++
			}
			inWord = true
		case unicode.IsLetter(r):
			// Unmeasured scripts still get a model judgment.
			return false
		case inWord && (r == '\'' || r == '’' || unicode.IsMark(r)):
		default:
			inWord = false
		}
	}
	if han+words == 0 || hanLimit <= 0 || englishLimit <= 0 {
		return false
	}
	// Compare the two fractions exactly, including very large user settings.
	limitHan, limitEnglish := uint64(hanLimit), uint64(englishLimit)
	if han > limitHan || words > limitEnglish {
		return false
	}
	rightHi, rightLo := bits.Mul64(limitHan-han, limitEnglish)
	wordsHi, wordsLo := bits.Mul64(words, limitHan)
	return wordsHi < rightHi || (wordsHi == rightHi && wordsLo <= rightLo)
}
