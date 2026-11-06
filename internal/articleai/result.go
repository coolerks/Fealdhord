package articleai

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

func parseResult(task string, categories []sqlc.ArticleAICategoriesRow, response *ai.ModelResponse) ([]database.ArticleAIMatch, database.ArticleAIScores, error) {
	var scores database.ArticleAIScores
	invalid := code("InvalidModelOutput")
	if response == nil || response.FinishReason != "stop" || len(response.Message.ToolCalls) != 0 {
		return nil, scores, invalid
	}
	var text strings.Builder
	for _, block := range response.Message.Content {
		if block.Type != "text" || text.Len()+len(block.Text) > 64<<10 {
			return nil, scores, invalid
		}
		text.WriteString(block.Text)
	}
	decoder := json.NewDecoder(strings.NewReader(text.String()))
	decoder.DisallowUnknownFields()
	if task == "featured" {
		var output struct {
			Importance *float64
			Relevance  *float64
			Reason     string
		}
		if decoder.Decode(&output) != nil || decoder.Decode(new(any)) != io.EOF ||
			!probability(output.Importance) || !probability(output.Relevance) || len(output.Reason) > 512 {
			return nil, scores, invalid
		}
		return nil, database.ArticleAIScores{Importance: *output.Importance, Relevance: *output.Relevance, Reason: output.Reason}, nil
	}
	var output struct {
		Categories []struct {
			ID         string
			Confidence *float64
			Reason     string
		}
	}
	if decoder.Decode(&output) != nil || decoder.Decode(new(any)) != io.EOF || output.Categories == nil || len(output.Categories) > len(categories) {
		return nil, scores, invalid
	}
	allowed := make(map[string]string, len(categories))
	for _, category := range categories {
		allowed[category.ID] = category.Name
	}
	matches := make([]database.ArticleAIMatch, 0, len(output.Categories))
	for _, match := range output.Categories {
		name, known := allowed[match.ID]
		if !known || !probability(match.Confidence) || len(match.Reason) > 512 {
			return nil, scores, invalid
		}
		delete(allowed, match.ID)
		matches = append(matches, database.ArticleAIMatch{ID: match.ID, Name: name, Confidence: *match.Confidence, Reason: match.Reason})
	}
	return matches, scores, nil
}

func probability(value *float64) bool { return value != nil && *value >= 0 && *value <= 1 }
