package translation

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
	"github.com/coolerks/Fealdhord/internal/reader"
)

type mtUnit struct {
	translationUnit
	blockID    string
	encoded    string
	htmlBridge *mtHTML
	textBridge *mtText
}

func matchingMTTranslations(sources []SourceBlock, cached []BlockTranslation) []BlockTranslation {
	original := make(map[string]string, len(sources))
	for _, source := range sources {
		original[source.ID] = SanitizeTranslatedHTML(source.InnerHTML)
	}
	return slices.DeleteFunc(matchingTranslations(sources, cached), func(translated BlockTranslation) bool {
		source := original[translated.ID]
		if hasTranslatableText(source) && !hasTranslatableText(translated.TranslatedHTML) {
			return true
		}
		want := encodeHTMLForMT(source).atoms
		if len(want) == 0 {
			return false
		}
		got := encodeHTMLForMT(translated.TranslatedHTML).atoms
		slices.Sort(want)
		slices.Sort(got)
		return !slices.Equal(want, got)
	})
}

func mtUnitFits(encoded string, caps EngineCapabilities) bool {
	return (caps.MaxSingleItemChars <= 0 || utf8.RuneCountInString(encoded) <= caps.MaxSingleItemChars) &&
		(caps.MaxBatchChars <= 0 || utf8.RuneCountInString(encoded) <= caps.MaxBatchChars) &&
		(caps.MaxBatchBytes <= 0 || len(encoded) <= caps.MaxBatchBytes)
}

func planMTBlock(block SourceBlock, engine Engine) (plannedBlock, []mtUnit, error) {
	source := SanitizeTranslatedHTML(block.InnerHTML)
	whole := translationUnit{ID: block.ID, HTML: source}
	parent := plannedBlock{source: block, units: []translationUnit{whole}}
	caps := engine.Capabilities()
	if caps.SupportsHTML {
		bridge := encodeHTMLForMT(source)
		if mtUnitFits(bridge.encoded, caps) {
			return parent, []mtUnit{{translationUnit: whole, blockID: block.ID, encoded: bridge.encoded, htmlBridge: bridge}}, nil
		}
	}
	if bridge := encodeTextForMT(source); mtUnitFits(bridge.encoded, caps) {
		return parent, []mtUnit{{translationUnit: whole, blockID: block.ID, encoded: bridge.encoded, textBridge: bridge}}, nil
	}

	// Split balanced HTML, keeping all code atoms and resource attributes local.
	limit := 4 * max(caps.MaxSingleItemChars, caps.MaxBatchChars, 5000)
	if caps.MaxBatchBytes > 0 {
		limit = min(limit, caps.MaxBatchBytes)
	}
	pieces, err := splitInline(source, block.ID, limit, func(unit translationUnit) bool {
		return mtUnitFits(encodeTextForMT(unit.HTML).encoded, caps)
	})
	if err != nil {
		return plannedBlock{}, nil, err
	}
	hash := sha256.New()
	fmt.Fprintf(hash, "mt-v1\x00%s\x00%s", engine.ID(), block.SourceHash)
	units := make([]mtUnit, 0, len(pieces))
	for _, piece := range pieces {
		fmt.Fprintf(hash, "\x00%s", piece.HTML)
		bridge := encodeTextForMT(piece.HTML)
		units = append(units, mtUnit{translationUnit: piece, blockID: block.ID, encoded: bridge.encoded, textBridge: bridge})
	}
	parent.units, parent.key = pieces, fmt.Sprintf("%x", hash.Sum(nil))
	return parent, units, nil
}

func (u mtUnit) isHTML() bool { return u.htmlBridge != nil }

func (u mtUnit) restore(raw string) (string, bool) {
	var restored string
	var ok bool
	if u.isHTML() {
		restored, ok = u.htmlBridge.restore(raw)
	} else {
		restored, ok = u.textBridge.restore(raw)
	}
	if !ok {
		return "", false
	}
	return validateUnit(u.translationUnit, restored)
}

func groupMTBatches(units []mtUnit, caps EngineCapabilities) [][]mtUnit {
	if len(units) == 0 {
		return nil
	}
	maxItems := caps.MaxBatchItems
	if maxItems <= 0 {
		maxItems = 50
	}
	maxChars := caps.MaxBatchChars
	if maxChars <= 0 {
		maxChars = 15000
	}
	maxBytes := caps.MaxBatchBytes

	var batches [][]mtUnit
	var cur []mtUnit
	curChars := 0
	curBytes := 0
	curHTML := units[0].isHTML()

	flush := func() {
		if len(cur) > 0 {
			batches = append(batches, cur)
			cur = nil
			curChars = 0
			curBytes = 0
		}
	}

	for _, u := range units {
		uChars := utf8.RuneCountInString(u.encoded)
		uBytes := len(u.encoded) + 1 // +1 预留换行/JSON 逗号开销
		if len(cur) > 0 && (u.isHTML() != curHTML ||
			len(cur) >= maxItems ||
			curChars+uChars > maxChars ||
			(maxBytes > 0 && curBytes+uBytes > maxBytes)) {
			flush()
		}
		if len(cur) == 0 {
			curHTML = u.isHTML()
		}
		cur = append(cur, u)
		curChars += uChars
		curBytes += uBytes
	}
	flush()
	return batches
}

func (s *Service) translateBodyWithAPI(
	ctx context.Context,
	article database.ArticleTranslationInput,
	req TranslateRequest,
	engineID EngineID,
	existing sqlc.ArticleTranslation,
) (Record, error) {
	if s.reader == nil {
		return Record{}, &ai.ModelError{Code: "StorageError"}
	}
	detail := s.reader.Get(ctx, reader.Request{ArticleID: article.ID, Mode: req.SourceMode})
	if !detail.OK && req.SourceMode == "reader" && (detail.Error == nil || detail.Error.Code == "NotFound") {
		detail = s.reader.Extract(ctx, reader.Request{ArticleID: article.ID, Mode: "reader"})
	}
	if !detail.OK || detail.Data == nil {
		if detail.Error != nil {
			return Record{}, &ai.ModelError{Code: detail.Error.Code}
		}
		return Record{}, &ai.ModelError{Code: "ReaderError"}
	}
	version := detail.Data.SourceVersion
	if req.SourceVersion != "" && req.SourceVersion != version {
		return Record{}, &ai.ModelError{Code: "Conflict"}
	}
	if !req.Force && bodyMatchesTarget(detail.Data.Text, req.TargetLang) {
		rec := Record{
			ArticleID: article.ID, TargetLang: req.TargetLang, SourceMode: req.SourceMode,
			ContentHash: article.ContentHash, SourceVersion: version, DetectedLang: req.TargetLang,
			Status: "skipped_same_lang", Blocks: []BlockTranslation{},
			UpdatedAt: max(time.Now().UTC().UnixMilli(), existing.UpdatedAt+1),
		}
		if err := s.saveRecord(ctx, rec, sql.NullInt64{}, sql.NullInt64{}); err != nil {
			return Record{}, &ai.ModelError{Code: "StorageError"}
		}
		s.notify(rec)
		return rec, nil
	}

	sources, err := ExtractBlocks(detail.Data.HTML)
	if err != nil {
		return Record{}, &ai.ModelError{Code: "ReaderError"}
	}
	w := bodyWork{
		record: Record{
			ArticleID: article.ID, TargetLang: req.TargetLang, SourceMode: req.SourceMode,
			ContentHash: article.ContentHash, SourceVersion: version, ModelID: "mt:" + string(engineID),
			TotalBlocks: len(sources), UpdatedAt: existing.UpdatedAt,
		},
		sources: sources, results: make(map[string]BlockTranslation), checkpoints: make(map[string]fragmentCheckpoint),
		parents: make(map[string]plannedBlock),
	}
	var priorCheckpoints map[string]fragmentCheckpoint
	if !req.Force {
		cached := decodeRecord(existing)
		if existing.ContentHash == article.ContentHash {
			w.record.TranslatedTitle, w.record.TranslatedSummary = cached.TranslatedTitle, cached.TranslatedSummary
			w.record.DetectedLang = cached.DetectedLang
		}
		for _, matched := range matchingMTTranslations(sources, cached.Blocks) {
			w.results[matched.ID] = matched
		}
		if existing.SourceVersion == version && len(existing.CheckpointJson) <= maxCheckpointBytes {
			_ = json.Unmarshal([]byte(existing.CheckpointJson), &priorCheckpoints)
		}
	}
	if w.record.TranslatedTitle == "" {
		if summary, err := s.store.GetArticleTranslation(ctx, article.ID, req.TargetLang, "summary"); err == nil && summary.ContentHash == article.ContentHash {
			w.record.TranslatedTitle, w.record.TranslatedSummary = summary.TranslatedTitle, summary.TranslatedSummary
		}
	}
	for _, block := range sources {
		if _, ok := w.results[block.ID]; ok {
			continue
		}
		if original := SanitizeTranslatedHTML(block.InnerHTML); original != "" && !hasTranslatableText(original) {
			w.results[block.ID] = BlockTranslation{ID: block.ID, SourceHash: block.SourceHash, TranslatedHTML: original}
		}
	}
	needTitle := article.Title != "" && w.record.TranslatedTitle == ""
	needsSummary := strings.TrimSpace(article.Summary) != "" && w.record.TranslatedSummary == ""
	if !needTitle && len(w.results) == len(sources) {
		if err := s.persistBody(ctx, &w, "completed", ""); err != nil {
			return Record{}, err
		}
		return w.record, nil
	}

	engine, err := s.resolveEngine(ctx, engineID, nil)
	if err != nil {
		return Record{}, err
	}
	localLang, localConfident := DetectLocal(detail.Data.Text)
	if localConfident && w.record.DetectedLang == "" {
		w.record.DetectedLang = localLang
	}
	sourceForCheck := ""
	if localConfident {
		sourceForCheck = localLang
	}
	if !engine.SupportsLanguagePair(sourceForCheck, req.TargetLang) {
		return Record{}, &ai.ModelError{Code: "UnsupportedLanguage"}
	}

	caps := engine.Capabilities()
	var pending []mtUnit
	var lastFailure error
	for _, block := range sources {
		if _, ok := w.results[block.ID]; ok {
			continue
		}
		parent, units, err := planMTBlock(block, engine)
		if err != nil {
			lastFailure = errTranslationBudget
			continue
		}
		w.restoreFragments(parent, priorCheckpoints[block.ID])
		for _, unit := range units {
			w.parents[unit.ID] = parent
			if w.checkpoints[block.ID].Pieces[unit.ID] != "" {
				continue
			}
			if !hasTranslatableText(unit.HTML) {
				if original, valid := validateUnit(unit.translationUnit, unit.HTML); valid {
					w.accept(unit.translationUnit, original)
					continue
				}
			}
			pending = append(pending, unit)
		}
	}

	batches := groupMTBatches(pending, caps)
	if !req.Background && len(batches) > 1 && req.StartBlockID != "" {
		for i, batch := range batches {
			if slices.ContainsFunc(batch, func(u mtUnit) bool { return u.blockID == req.StartBlockID }) {
				if i > 0 {
					copy(batches[1:i+1], batches[:i])
					batches[0] = batch
				}
				break
			}
		}
	}

	if err := s.persistBody(ctx, &w, "translating", ""); err != nil {
		return Record{}, err
	}

	// 1. 翻译标题（如缺失）
	if needTitle {
		titleRes, titleErr := s.callEngineWithLimit(ctx, engine, BatchRequest{
			Texts:      []string{article.Title},
			TargetLang: req.TargetLang,
			IsHTML:     false,
		}, req.Background)
		if titleErr == nil {
			titleErr = titleRes.FailedIndices[0]
		}
		if titleErr != nil {
			lastFailure = titleErr
			if !isTransientMTError(titleErr) {
				return s.finalizeMTBody(ctx, &w, needTitle, needsSummary, lastFailure)
			}
		} else if len(titleRes.Translations) > 0 {
			w.record.TranslatedTitle = strings.TrimSpace(titleRes.Translations[0])
			if w.record.DetectedLang == "" && titleRes.DetectedLang != "" {
				w.record.DetectedLang = titleRes.DetectedLang
			}
		}
	}

	// 2. 逐批翻译正文段落，每批完成立即落库并推送进度
	for _, batch := range batches {
		if ctx.Err() != nil {
			break
		}
		texts := make([]string, len(batch))
		for i, u := range batch {
			texts[i] = u.encoded
		}
		res, batchErr := s.callEngineWithLimit(ctx, engine, BatchRequest{
			Texts:      texts,
			TargetLang: req.TargetLang,
			IsHTML:     batch[0].isHTML(),
		}, req.Background)
		if batchErr != nil {
			lastFailure = batchErr
			if !isTransientMTError(batchErr) {
				break
			}
			continue
		}
		if w.record.DetectedLang == "" && res.DetectedLang != "" {
			w.record.DetectedLang = res.DetectedLang
		}

		for i, u := range batch {
			if i >= len(res.Translations) {
				lastFailure = &ai.ModelError{Code: "InvalidModelOutput"}
				continue
			}
			if res.FailedIndices != nil && res.FailedIndices[i] != nil {
				lastFailure = res.FailedIndices[i]
				continue
			}
			restored, ok := u.restore(res.Translations[i])
			if !ok && u.isHTML() {
				u.htmlBridge = nil
				u.textBridge = encodeTextForMT(u.HTML)
				if mtUnitFits(u.textBridge.encoded, caps) {
					fallback, err := s.callEngineWithLimit(ctx, engine, BatchRequest{
						Texts: []string{u.textBridge.encoded}, TargetLang: req.TargetLang,
					}, req.Background)
					if err != nil {
						lastFailure = err
						continue
					}
					if failure := fallback.FailedIndices[0]; failure != nil {
						lastFailure = failure
						continue
					}
					if len(fallback.Translations) == 1 {
						restored, ok = u.restore(fallback.Translations[0])
					}
				}
			}
			if !ok {
				lastFailure = &ai.ModelError{Code: "InvalidModelOutput"}
				continue
			}
			w.accept(u.translationUnit, restored)
		}

		if err := s.persistBody(ctx, &w, "translating", ""); err != nil {
			if ctx.Err() != nil {
				return s.cancelledBody(ctx, &w)
			}
			return w.record, err
		}
	}

	return s.finalizeMTBody(ctx, &w, needTitle, needsSummary, lastFailure)
}

func (s *Service) finalizeMTBody(ctx context.Context, w *bodyWork, needTitle, needsSummary bool, lastFailure error) (Record, error) {
	if ctx.Err() != nil {
		return s.cancelledBody(ctx, w)
	}
	status, code := "completed", ""
	if len(w.results) != len(w.sources) || (needTitle && w.record.TranslatedTitle == "") {
		if lastFailure == nil {
			lastFailure = &ai.ModelError{Code: "InvalidModelOutput"}
		}
		status, code = "failed", modelErrorCode(lastFailure)
		if len(w.results) > 0 || hasSavedFragments(w.checkpoints) {
			status = "partial"
		}
	}
	if err := s.persistBody(ctx, w, status, code); err != nil {
		if ctx.Err() != nil {
			return s.cancelledBody(ctx, w)
		}
		return w.record, err
	}
	if w.record.TranslatedTitle != "" {
		s.cacheBodyTitle(ctx, w.record, needsSummary)
	}
	if status != "completed" {
		return w.record, lastFailure
	}
	return w.record, nil
}

func (s *Service) callEngineWithLimit(ctx context.Context, engine Engine, req BatchRequest, background bool) (BatchResult, error) {
	chars := 0
	for _, t := range req.Texts {
		chars += utf8.RuneCountInString(t)
	}
	caps := engine.Capabilities()
	var lastErr error
	req.waitForRetry = func(ctx context.Context, chars int) error {
		return s.limiter.Wait(ctx, engine.ID(), caps, chars, background)
	}
	for attempt := 0; attempt < 3; attempt++ {
		if err := req.waitForRetry(ctx, chars); err != nil {
			return BatchResult{}, &ai.ModelError{Code: modelErrorCode(err)}
		}
		callCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
		res, err := engine.TranslateBatch(callCtx, req)
		cancel()
		if err == nil {
			return res, nil
		}
		lastErr = err
		var me *ai.ModelError
		if errors.As(err, &me) && me.Code == "RateLimitError" && attempt < 2 {
			backoff := me.RetryAfter
			if backoff <= 0 {
				backoff = time.Duration(attempt+1) * 2 * time.Second
			}
			if backoff > 15*time.Second {
				backoff = 15 * time.Second
			}
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return BatchResult{}, &ai.ModelError{Code: modelErrorCode(ctx.Err())}
			case <-timer.C:
			}
			continue
		}
		return BatchResult{}, err
	}
	return BatchResult{}, lastErr
}

func isTransientMTError(err error) bool {
	switch modelErrorCode(err) {
	case "Timeout", "ProviderConnectionError":
		return true
	default:
		return false
	}
}
