package translation

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

func incompleteMTSummary(row sqlc.ArticleTranslation, title, summary string) bool {
	return row.SourceMode == "summary" && row.Status == "completed" && strings.HasPrefix(row.ModelID, "mt:") &&
		((strings.TrimSpace(title) != "" && strings.TrimSpace(row.TranslatedTitle) == "") ||
			(strings.TrimSpace(summary) != "" && strings.TrimSpace(row.TranslatedSummary) == ""))
}

func (s *Service) translateSummaryWithAPI(
	ctx context.Context,
	article database.ArticleTranslationInput,
	req TranslateRequest,
	detectedLang string,
	engineID EngineID,
) (Record, error) {
	engine, err := s.resolveEngine(ctx, engineID, nil)
	if err != nil {
		return Record{}, err
	}
	if !engine.SupportsLanguagePair(detectedLang, req.TargetLang) {
		return Record{}, &ai.ModelError{Code: "UnsupportedLanguage"}
	}

	title := truncateRunes(strings.TrimSpace(article.Title), 512)
	summary := truncateRunes(strings.TrimSpace(article.Summary), 1500)
	texts := make([]string, 0, 2)
	titleIdx, summaryIdx := -1, -1
	if title != "" {
		titleIdx = len(texts)
		texts = append(texts, title)
	}
	if summary != "" {
		summaryIdx = len(texts)
		texts = append(texts, summary)
	}
	if len(texts) == 0 {
		rec := Record{
			ArticleID: article.ID, TargetLang: req.TargetLang, SourceMode: "summary",
			ContentHash: article.ContentHash, DetectedLang: detectedLang, Status: "completed",
			Blocks: []BlockTranslation{}, ModelID: "mt:" + string(engineID), UpdatedAt: time.Now().UTC().UnixMilli(),
		}
		if err := s.saveRecord(ctx, rec, sql.NullInt64{}, sql.NullInt64{}); err != nil {
			return Record{}, &ai.ModelError{Code: "StorageError"}
		}
		s.notify(rec)
		return rec, nil
	}

	res, err := s.callEngineWithLimit(ctx, engine, BatchRequest{
		Texts:      texts,
		TargetLang: req.TargetLang,
		IsHTML:     false,
	}, req.Background)
	if err != nil {
		return Record{}, err
	}
	for i := range texts {
		if failure := res.FailedIndices[i]; failure != nil {
			return Record{}, failure
		}
		if i >= len(res.Translations) || strings.TrimSpace(res.Translations[i]) == "" {
			return Record{}, &ai.ModelError{Code: "InvalidModelOutput"}
		}
	}
	if len(res.Translations) != len(texts) {
		return Record{}, &ai.ModelError{Code: "InvalidModelOutput"}
	}

	lang := detectedLang
	// A batch's detected language can describe its first input, the title.
	if len(texts) == 1 && strings.TrimSpace(res.DetectedLang) != "" {
		lang = strings.TrimSpace(res.DetectedLang)
	}

	var outTitle, outSummary string
	if titleIdx >= 0 {
		outTitle = strings.TrimSpace(res.Translations[titleIdx])
	}
	if summaryIdx >= 0 {
		outSummary = strings.TrimSpace(res.Translations[summaryIdx])
	}

	rec := Record{
		ArticleID:         article.ID,
		TargetLang:        req.TargetLang,
		SourceMode:        "summary",
		ContentHash:       article.ContentHash,
		DetectedLang:      lang,
		Status:            "completed",
		TranslatedTitle:   outTitle,
		TranslatedSummary: outSummary,
		Blocks:            []BlockTranslation{},
		ModelID:           "mt:" + string(engineID),
		UpdatedAt:         time.Now().UTC().UnixMilli(),
	}
	if !req.Force && lang != "" && strings.EqualFold(normalizeDetectedLang(lang), req.TargetLang) {
		rec.Status = "skipped_same_lang"
		rec.TranslatedTitle = ""
		rec.TranslatedSummary = ""
	}
	if err := s.saveRecord(ctx, rec, sql.NullInt64{}, sql.NullInt64{}); err != nil {
		return Record{}, &ai.ModelError{Code: "StorageError"}
	}
	s.notify(rec)
	if s.onChanged != nil {
		s.onChanged()
	}
	return rec, nil
}

func normalizeDetectedLang(code string) string {
	c := strings.ToLower(strings.TrimSpace(code))
	switch c {
	case "zh", "zh-cn", "zh-hans", "zh-chs":
		return "zh-CN"
	case "zh-tw", "zh-hant", "zh-cht", "cht":
		return "zh-TW"
	case "jp":
		return "ja"
	case "kor":
		return "ko"
	case "fra":
		return "fr"
	case "spa":
		return "es"
	default:
		return code
	}
}
