package translation

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/coolerks/Fealdhord/internal/database"
)

const maxTranslationLine = 4 << 20

type translationLines struct {
	line    strings.Builder
	discard bool
}

func (l *translationLines) feed(data string, accept func(string) error) error {
	for data != "" {
		line, rest, complete := strings.Cut(data, "\n")
		if len(line) > maxTranslationLine-l.line.Len() {
			l.line.Reset()
			l.discard = true
		}
		if !l.discard {
			l.line.WriteString(line)
		}
		// Providers can finish a JSON object before sending the newline or stream terminator.
		if !complete && (l.discard || !strings.HasSuffix(strings.TrimRight(line, " \t\r"), "}") || !json.Valid([]byte(l.line.String()))) {
			return nil
		}
		if !l.discard {
			if err := accept(l.line.String()); err != nil {
				return err
			}
		}
		l.line.Reset()
		l.discard = false
		data = rest
	}
	return nil
}

func (s *Service) runBodyBatch(ctx context.Context, work *bodyWork, adapter ai.Model, planner batchPlanner,
	batch []translationUnit, title bool, hint string, background, repair bool,
) ([]translationUnit, bool, error) {
	request, fits := planner.request(batch, title, hint)
	if !fits {
		request, fits = planner.request(batch, title, "")
		if !fits {
			return batch, title, errTranslationBudget
		}
	}
	if repair {
		output := min(request.Budget.OutputReserve, max(512, *request.MaxOutputTokens*2))
		request.MaxOutputTokens = &output
	}
	if background {
		if err := s.store.ReserveTranslationAttempt(ctx, int64(ai.EstimateRequest(*request)), time.Now()); err != nil {
			switch {
			case errors.Is(err, database.ErrArticleAIBudget):
				return batch, title, &ai.ModelError{Code: "BudgetExceeded"}
			case ctx.Err() != nil:
				return batch, title, &ai.ModelError{Code: modelErrorCode(ctx.Err())}
			default:
				return batch, title, &ai.ModelError{Code: "StorageError"}
			}
		}
	}

	start := time.Now()
	batchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	events, err := adapter.Stream(batchCtx, request)
	if err != nil {
		slog.WarnContext(ctx, "正文翻译请求失败", "code", modelErrorCode(err), "repair", repair, "items", len(batch), "duration_ms", time.Since(start).Milliseconds())
		return batch, title, err
	}

	expected := make(map[string]translationUnit, len(batch))
	for _, unit := range batch {
		expected[unit.ID] = unit
	}
	titleMissing := title
	rejected := make(map[string]string)
	malformedRecords := 0
	target := work.record.TargetLang
	accept := func(line string) error {
		if strings.TrimSpace(line) == "" {
			return nil
		}
		var item translationItem
		if json.Unmarshal([]byte(line), &item) != nil {
			malformedRecords++
			return nil
		}
		if titleMissing && item.ID == "title" {
			if titleText := strings.TrimSpace(item.Text); titleText != "" {
				work.record.TranslatedTitle = titleText
				titleMissing = false
				return s.persistBody(ctx, work, "translating", "")
			}
			return nil
		}
		unit, ok := expected[item.ID]
		if !ok {
			return nil
		}
		text := strings.TrimSpace(item.Text)
		if text == "" {
			rejected[item.ID] = "empty_text"
			return nil
		}
		if text == strings.TrimSpace(unit.wireText()) {
			if language, confident := DetectLocal(item.Text); confident && !MatchesTarget(language, true, target) {
				rejected[item.ID] = "unchanged_source"
				return nil
			}
		}
		result, ok := validateUnit(unit, item.Text)
		if !ok {
			rejected[item.ID] = "inline_format"
			return nil
		}
		work.accept(unit, result)
		delete(expected, item.ID)
		return s.persistBody(ctx, work, "translating", "")
	}

	var lines translationLines
	var usage *ai.Usage
	var finishReason string
	var firstText time.Duration
readStream:
	for {
		select {
		case <-ctx.Done():
			return batch, title, &ai.ModelError{Code: modelErrorCode(ctx.Err())}
		case event, open := <-events:
			if !open {
				if finishReason == "" {
					err = &ai.ModelError{Code: "ProviderProtocolError"}
				}
				break readStream
			}
			switch event.Type {
			case "text_delta":
				if firstText == 0 && event.Text != "" {
					firstText = time.Since(start)
				}
				if err := lines.feed(event.Text, accept); err != nil {
					return batch, title, err
				}
			case "usage":
				usage = event.Usage
			case "message_end":
				finishReason = event.FinishReason
				switch finishReason {
				case "stop", "length":
				case "refusal", "content_filter":
					err = &ai.ModelError{Code: "TranslationRefused"}
				default:
					err = &ai.ModelError{Code: "ProviderProtocolError"}
				}
				break readStream
			case "error":
				err = event.Error
				if err == nil {
					err = &ai.ModelError{Code: "ProviderProtocolError"}
				}
				break readStream
			}
		}
	}
	// Diagnose any residual malformed or truncated line at stream termination.
	if ctx.Err() == nil && !lines.discard && (finishReason == "stop" || finishReason == "length" || canSkipFailedBatch(err)) {
		if err := accept(lines.line.String()); err != nil {
			return batch, title, err
		}
	}
	if usage != nil && usage.InputTokens != nil {
		work.input += int64(*usage.InputTokens)
	} else {
		work.inputKnown = false
	}
	if usage != nil && usage.OutputTokens != nil {
		work.output += int64(*usage.OutputTokens)
	} else {
		work.outputKnown = false
	}
	var missing []translationUnit
	for _, unit := range batch {
		if _, ok := expected[unit.ID]; ok {
			missing = append(missing, unit)
		}
	}
	slog.DebugContext(ctx, "正文翻译模型耗时", "repair", repair, "items", len(batch), "first_text_ms", firstText.Milliseconds(),
		"duration_ms", time.Since(start).Milliseconds(), "estimated_input", ai.EstimateRequest(*request), "output_limit", *request.MaxOutputTokens)
	if err != nil || len(missing) > 0 || titleMissing {
		code := "InvalidModelOutput"
		if err != nil {
			code = modelErrorCode(err)
		}
		var missingDetails []string
		for _, unit := range missing[:min(len(missing), 8)] {
			reason := rejected[unit.ID]
			if reason == "" {
				reason = "missing_record"
			}
			missingDetails = append(missingDetails, unit.ID+":"+reason)
		}
		slog.WarnContext(ctx, "正文翻译批次未齐", "code", code, "finish", finishReason,
			"repair", repair, "items", len(batch), "missing", len(missing), "title_missing", titleMissing,
			"missing_details", missingDetails, "malformed_records", malformedRecords,
			"first_text_ms", firstText.Milliseconds(), "output_limit", *request.MaxOutputTokens, "duration_ms", time.Since(start).Milliseconds())
	}
	return missing, titleMissing, err
}
