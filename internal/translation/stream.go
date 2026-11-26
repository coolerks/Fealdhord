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

const (
	translationFlushInterval = 120 * time.Millisecond
	maxTranslationLine       = 4 << 20
)

type translationLines struct {
	line    strings.Builder
	discard bool
}

func (l *translationLines) feed(data string, accept func(string)) {
	for data != "" {
		line, rest, complete := strings.Cut(data, "\n")
		if len(line) > maxTranslationLine-l.line.Len() {
			l.line.Reset()
			l.discard = true
		}
		if !l.discard {
			l.line.WriteString(line)
		}
		if !complete {
			return
		}
		if !l.discard {
			accept(l.line.String())
		}
		l.line.Reset()
		l.discard = false
		data = rest
	}
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
	changed := false
	titleMissing := title
	firstRecord := true
	flush := func() error {
		if !changed {
			return nil
		}
		if err := s.persistBody(ctx, work, "translating", ""); err != nil {
			return err
		}
		changed = false
		return nil
	}
	target := work.record.TargetLang
	accept := func(line string) {
		var item translationItem
		if len(strings.TrimSpace(line)) == 0 || json.Unmarshal([]byte(line), &item) != nil || strings.TrimSpace(item.Text) == "" {
			return
		}
		if titleMissing && item.ID == "title" {
			work.record.TranslatedTitle = strings.TrimSpace(item.Text)
			titleMissing, changed = false, true
			return
		}
		unit, ok := expected[item.ID]
		if !ok {
			return
		}
		if strings.TrimSpace(item.Text) == strings.TrimSpace(unit.wireText()) {
			if language, confident := DetectLocal(item.Text); confident && !MatchesTarget(language, true, target) {
				return
			}
		}
		result, ok := validateUnit(unit, item.Text)
		if !ok {
			return
		}
		work.accept(unit, result)
		delete(expected, item.ID)
		changed = true
	}

	ticker := time.NewTicker(translationFlushInterval)
	defer ticker.Stop()
	var lines translationLines
	var usage *ai.Usage
	var finishReason string
	var firstText time.Duration
readStream:
	for {
		select {
		case <-ctx.Done():
			return batch, title, &ai.ModelError{Code: modelErrorCode(ctx.Err())}
		case <-ticker.C:
			if err := flush(); err != nil {
				return batch, title, err
			}
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
				lines.feed(event.Text, accept)
				if firstRecord && changed {
					if err := flush(); err != nil {
						return batch, title, err
					}
					firstRecord = false
				}
			case "usage":
				usage = event.Usage
			case "message_end":
				finishReason = event.FinishReason
				switch finishReason {
				case "stop", "length":
					if !lines.discard {
						accept(lines.line.String())
					}
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
	if err := flush(); err != nil {
		return batch, title, err
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
		slog.WarnContext(ctx, "正文翻译批次未齐", "code", code, "finish", finishReason,
			"repair", repair, "items", len(batch), "missing", len(missing), "title_missing", titleMissing,
			"duration_ms", time.Since(start).Milliseconds())
	}
	return missing, titleMissing, err
}
