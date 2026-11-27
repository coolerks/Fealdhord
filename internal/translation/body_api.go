package translation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
	id         string
	blockID    string
	sourceHTML string
	encoded    string
	htmlBridge *mtHTML
	textBridge *mtText
	isHTML     bool
}

type mtPlannedBlock struct {
	source SourceBlock
	units  []mtUnit
}

// splitPlainForMT 将超长纯文本按句子/换行边界切分为不超过 maxRunes 的片段。
func splitPlainForMT(text string, maxRunes int) []string {
	if maxRunes <= 0 || utf8.RuneCountInString(text) <= maxRunes {
		return []string{text}
	}
	runes := []rune(text)
	var parts []string
	for len(runes) > maxRunes {
		cut := maxRunes
		for i := maxRunes; i > maxRunes/2; i-- {
			switch runes[i-1] {
			case '\n', '。', '！', '？', '.', '!', '?', ';', '；':
				cut = i
				goto found
			}
		}
		for i := maxRunes; i > maxRunes/2; i-- {
			if runes[i-1] == ' ' || runes[i-1] == ',' || runes[i-1] == '，' {
				cut = i
				goto found
			}
		}
	found:
		parts = append(parts, string(runes[:cut]))
		runes = runes[cut:]
	}
	if len(runes) > 0 {
		parts = append(parts, string(runes))
	}
	return parts
}

func planMTBlock(block SourceBlock, caps EngineCapabilities) mtPlannedBlock {
	sanitized := SanitizeTranslatedHTML(block.InnerHTML)
	maxSingle := caps.MaxSingleItemChars
	if maxSingle <= 0 {
		maxSingle = 5000
	}
	if caps.SupportsHTML {
		hb := encodeHTMLForMT(sanitized)
		if utf8.RuneCountInString(hb.encoded) <= maxSingle &&
			(caps.MaxBatchBytes <= 0 || len(hb.encoded) <= caps.MaxBatchBytes) {
			return mtPlannedBlock{
				source: block,
				units: []mtUnit{{
					id:         block.ID,
					blockID:    block.ID,
					sourceHTML: sanitized,
					encoded:    hb.encoded,
					htmlBridge: hb,
					isHTML:     true,
				}},
			}
		}
	}
	// 纯文本占位符模式（或超长 HTML 段落安全降级拆分）
	tb := encodeTextForMT(sanitized)
	if utf8.RuneCountInString(tb.encoded) <= maxSingle &&
		(caps.MaxBatchBytes <= 0 || len(tb.encoded) <= caps.MaxBatchBytes) {
		return mtPlannedBlock{
			source: block,
			units: []mtUnit{{
				id:         block.ID,
				blockID:    block.ID,
				sourceHTML: sanitized,
				encoded:    tb.encoded,
				textBridge: tb,
				isHTML:     false,
			}},
		}
	}
	// 超出单段上限时按纯文本句子切分
	limit := maxSingle
	if caps.MaxBatchBytes > 0 && caps.MaxBatchBytes/4 < limit {
		limit = max(200, caps.MaxBatchBytes/4)
	}
	pieces := splitPlainForMT(block.PlainText, limit)
	units := make([]mtUnit, 0, len(pieces))
	for i, piece := range pieces {
		pb := &mtText{encoded: piece, hasTags: false}
		units = append(units, mtUnit{
			id:         block.ID + "#" + string(rune('0'+i/10)) + string(rune('0'+i%10)),
			blockID:    block.ID,
			sourceHTML: piece,
			encoded:    piece,
			textBridge: pb,
			isHTML:     false,
		})
	}
	return mtPlannedBlock{source: block, units: units}
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
	curHTML := units[0].isHTML

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
		if len(cur) > 0 && (u.isHTML != curHTML ||
			len(cur) >= maxItems ||
			curChars+uChars > maxChars ||
			(maxBytes > 0 && curBytes+uBytes > maxBytes)) {
			flush()
		}
		if len(cur) == 0 {
			curHTML = u.isHTML
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
	if !req.Force && existing.ContentHash == article.ContentHash && version != "" && existing.SourceVersion == version {
		cached := decodeRecord(existing)
		if cached.Status == "completed" || cached.Status == "skipped_same_lang" {
			return cached, nil
		}
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
	if !req.Force {
		cached := decodeRecord(existing)
		if existing.ContentHash == article.ContentHash {
			w.record.TranslatedTitle, w.record.TranslatedSummary = cached.TranslatedTitle, cached.TranslatedSummary
			w.record.DetectedLang = cached.DetectedLang
		}
		for _, matched := range matchingTranslations(sources, cached.Blocks) {
			w.results[matched.ID] = matched
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
	plannedByBlock := make(map[string]mtPlannedBlock, len(sources))
	fragmentResults := make(map[string]map[string]string)
	var pending []mtUnit
	for _, block := range sources {
		if _, ok := w.results[block.ID]; ok {
			continue
		}
		pb := planMTBlock(block, caps)
		plannedByBlock[block.ID] = pb
		pending = append(pending, pb.units...)
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
	var lastFailure error
	if needTitle {
		titleRes, titleErr := s.callEngineWithLimit(ctx, engine, BatchRequest{
			Texts:      []string{article.Title},
			TargetLang: req.TargetLang,
			IsHTML:     false,
		}, req.Background)
		if titleErr != nil {
			lastFailure = titleErr
			if !isTransientMTError(titleErr) {
				return s.finalizeMTBody(ctx, &w, sources, needTitle, lastFailure)
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
			IsHTML:     batch[0].isHTML,
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
			rawOut := res.Translations[i]
			var restored string
			if u.isHTML && u.htmlBridge != nil {
				var ok bool
				restored, ok = u.htmlBridge.restore(rawOut)
				if !ok {
					// 单段降级为模式 B 纯文本占位符重试一次
					tb := encodeTextForMT(u.sourceHTML)
					if fbRes, fbErr := s.callEngineWithLimit(ctx, engine, BatchRequest{
						Texts:      []string{tb.encoded},
						TargetLang: req.TargetLang,
						IsHTML:     false,
					}, req.Background); fbErr == nil && len(fbRes.Translations) > 0 {
						restored, _ = tb.restore(fbRes.Translations[0])
					}
				}
			} else if u.textBridge != nil {
				restored, _ = u.textBridge.restore(rawOut)
			} else {
				restored = SanitizeTranslatedHTML(rawOut)
			}
			if restored == "" {
				lastFailure = &ai.ModelError{Code: "InvalidModelOutput"}
				continue
			}

			pb := plannedByBlock[u.blockID]
			if len(pb.units) == 1 {
				w.results[u.blockID] = BlockTranslation{
					ID:             u.blockID,
					SourceHash:     pb.source.SourceHash,
					TranslatedHTML: restored,
				}
			} else {
				frags := fragmentResults[u.blockID]
				if frags == nil {
					frags = make(map[string]string, len(pb.units))
					fragmentResults[u.blockID] = frags
				}
				frags[u.id] = restored
				if len(frags) == len(pb.units) {
					var joined strings.Builder
					for _, part := range pb.units {
						joined.WriteString(frags[part.id])
					}
					w.results[u.blockID] = BlockTranslation{
						ID:             u.blockID,
						SourceHash:     pb.source.SourceHash,
						TranslatedHTML: SanitizeTranslatedHTML(joined.String()),
					}
				}
			}
		}

		if err := s.persistBody(ctx, &w, "translating", ""); err != nil {
			return w.record, err
		}
	}

	return s.finalizeMTBody(ctx, &w, sources, needTitle, lastFailure)
}

func (s *Service) finalizeMTBody(ctx context.Context, w *bodyWork, sources []SourceBlock, needTitle bool, lastFailure error) (Record, error) {
	if ctx.Err() != nil {
		status := "failed"
		if len(w.results) > 0 {
			status = "partial"
		}
		rec := w.snapshot(status, modelErrorCode(ctx.Err()))
		if data, err := json.Marshal(w.checkpoints); err == nil {
			_ = s.saveCheckpoint(context.Background(), rec, string(data), sql.NullInt64{}, sql.NullInt64{})
		}
		s.notify(rec)
		return rec, &ai.ModelError{Code: rec.ErrorCode}
	}
	status, code := "completed", ""
	if len(w.results) != len(sources) || (needTitle && w.record.TranslatedTitle == "") {
		if lastFailure == nil {
			lastFailure = &ai.ModelError{Code: "InvalidModelOutput"}
		}
		status, code = "failed", modelErrorCode(lastFailure)
		if len(w.results) > 0 {
			status = "partial"
		}
	}
	if err := s.persistBody(ctx, w, status, code); err != nil {
		return w.record, err
	}
	if w.record.TranslatedTitle != "" {
		s.cacheBodyTitle(ctx, w.record)
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
	for attempt := 0; attempt < 3; attempt++ {
		if err := s.limiter.Wait(ctx, engine.ID(), caps, chars, background); err != nil {
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
