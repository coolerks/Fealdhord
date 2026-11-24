package translation

import (
	"regexp"
	"strings"
	"unicode"
)

// LanguageOption 定义系统支持的目标翻译语言。
type LanguageOption struct {
	Code       string `json:"code"`
	Label      string `json:"label"`
	PromptName string `json:"promptName"`
}

var SupportedLanguages = []LanguageOption{
	{Code: "zh-CN", Label: "简体中文", PromptName: "Simplified Chinese (简体中文)"},
	{Code: "zh-TW", Label: "繁體中文", PromptName: "Traditional Chinese (繁體中文)"},
	{Code: "en", Label: "English", PromptName: "English"},
	{Code: "ja", Label: "日本語", PromptName: "Japanese (日本語)"},
	{Code: "ko", Label: "한국어", PromptName: "Korean (한국어)"},
	{Code: "fr", Label: "Français", PromptName: "French"},
	{Code: "de", Label: "Deutsch", PromptName: "German"},
	{Code: "es", Label: "Español", PromptName: "Spanish"},
	{Code: "ru", Label: "Русский", PromptName: "Russian"},
}

func NormalizeTargetLang(code string) string {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return "zh-CN"
	}
	for _, option := range SupportedLanguages {
		if strings.EqualFold(option.Code, trimmed) {
			return option.Code
		}
	}
	return trimmed
}

func LanguagePromptName(code string) string {
	norm := NormalizeTargetLang(code)
	for _, option := range SupportedLanguages {
		if option.Code == norm {
			return option.PromptName
		}
	}
	return norm
}

var urlPattern = regexp.MustCompile(`https?://\S+`)

var englishStops = map[string]bool{
	"the": true, "and": true, "that": true, "for": true, "with": true, "this": true,
	"from": true, "have": true, "are": true, "was": true, "were": true, "been": true,
	"will": true, "would": true, "about": true, "which": true, "their": true, "there": true,
	"what": true, "when": true, "how": true, "can": true, "not": true, "but": true, "into": true,
}

var frenchStops = map[string]bool{
	"les": true, "des": true, "une": true, "dans": true, "pour": true, "que": true,
	"qui": true, "pas": true, "sur": true, "est": true, "avec": true, "plus": true, "vous": true, "nous": true,
}

var germanStops = map[string]bool{
	"der": true, "die": true, "das": true, "und": true, "nicht": true, "mit": true,
	"sich": true, "auf": true, "für": true, "ist": true, "den": true, "ein": true, "eine": true, "werden": true,
}

var spanishStops = map[string]bool{
	"los": true, "las": true, "una": true, "para": true, "por": true, "con": true,
	"que": true, "del": true, "como": true, "pero": true, "más": true, "este": true, "esta": true, "sobre": true,
}

// 常见繁体特有高频字样本，用于区分 zh-CN 与 zh-TW。
const traditionalMarkers = "這為個來時會說們對實學開關點體國發後過還進與從動經產當樣無現業種義問題長書圖電語機網"

// DetectLocal 对文本进行零成本本地语言初筛。
// 仅在特征足够明确时返回 confident=true，避免误判导致漏翻或误跳。
func DetectLocal(sample string) (lang string, confident bool) {
	cleaned := urlPattern.ReplaceAllString(sample, " ")
	var han, kana, hangul, cyrillic, latin, total int
	var tradCount int
	for _, r := range cleaned {
		switch {
		case unicode.In(r, unicode.Hiragana, unicode.Katakana):
			kana++
			total++
		case unicode.In(r, unicode.Hangul):
			hangul++
			total++
		case unicode.In(r, unicode.Han):
			han++
			total++
			if strings.ContainsRune(traditionalMarkers, r) {
				tradCount++
			}
		case unicode.In(r, unicode.Cyrillic):
			cyrillic++
			total++
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= 0x00C0 && r <= 0x024F):
			latin++
			total++
		}
	}
	if total < 8 {
		return "", false
	}
	ratio := func(n int) float64 { return float64(n) / float64(total) }

	// 日文：存在明显假名即可高置信度判定为日文（日文常夹杂大量汉字）。
	if kana >= 3 && ratio(kana+han) >= 0.55 {
		return "ja", true
	}
	// 韩文：谚文占比高。
	if hangul >= 4 && ratio(hangul) >= 0.45 {
		return "ko", true
	}
	// 中文：汉字占比高且几乎无假名。
	if han >= 6 && kana == 0 && ratio(han) >= 0.40 {
		if tradCount >= 3 && float64(tradCount)/float64(han) >= 0.08 {
			return "zh-TW", true
		}
		return "zh-CN", true
	}
	// 俄文等西里尔文。
	if cyrillic >= 8 && ratio(cyrillic) >= 0.55 {
		return "ru", true
	}
	// 拉丁语系：用高频停用词区分英语/法语/德语/西语。
	if latin >= 20 && ratio(latin) >= 0.70 {
		words := strings.FieldsFunc(strings.ToLower(cleaned), func(r rune) bool {
			return !unicode.IsLetter(r)
		})
		if len(words) < 6 {
			return "", false
		}
		var en, fr, de, es int
		for _, w := range words {
			if englishStops[w] {
				en++
			}
			if frenchStops[w] {
				fr++
			}
			if germanStops[w] {
				de++
			}
			if spanishStops[w] {
				es++
			}
		}
		scores := []struct {
			code  string
			count int
		}{
			{"en", en}, {"fr", fr}, {"de", de}, {"es", es},
		}
		best, second := scores[0], scores[1]
		for _, item := range scores[1:] {
			if item.count > best.count {
				second = best
				best = item
			} else if item.count > second.count {
				second = item
			}
		}
		if best.count >= 3 && best.count >= second.count*2 {
			return best.code, true
		}
	}
	return "", false
}

// MatchesTarget 判断本地高置信度语言检测结果是否已匹配目标语言（匹配则无需消耗 Token 翻译）。
func MatchesTarget(detected string, confident bool, targetLang string) bool {
	if !confident || detected == "" {
		return false
	}
	target := NormalizeTargetLang(targetLang)
	return strings.EqualFold(detected, target)
}
