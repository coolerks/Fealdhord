package ai

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type CatalogModel struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"displayName"`
	Metadata    Metadata `json:"metadata,omitzero"`
}

// ListModels reads the catalog on the saved provider's origin.
func ListModels(ctx context.Context, baseURL, apiKey string, anthropic bool) ([]CatalogModel, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	client, err := newModelClient(baseURL, apiKey, "models", nil)
	if err != nil {
		return nil, err
	}
	target, _ := url.Parse(client.endpoint)
	for {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
		if err != nil {
			return nil, &ModelError{Code: "ValidationError"}
		}
		request.Header.Set("Accept", "application/json")
		response, err := client.do(request, anthropic, false)
		if err != nil {
			return nil, err
		}
		if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusMethodNotAllowed {
			response.Body.Close()
			if target.Path == "/v1/models" {
				target.Path, target.RawPath = "/api/v1/models", ""
				continue
			}
			return nil, &ModelError{Code: "ModelCatalogUnavailable"}
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			response.Body.Close()
			return nil, chatStatusError(response)
		}
		var payload struct {
			Data []struct {
				ID            string `json:"id"`
				DisplayName   string `json:"display_name"`
				Name          string `json:"name"`
				Context       int    `json:"context_length"`
				ContextWindow int    `json:"contextWindow"`
				TopProvider   struct {
					MaxOutput int `json:"max_completion_tokens"`
				} `json:"top_provider"`
				Architecture struct {
					Input []string `json:"input_modalities"`
				} `json:"architecture"`
				Pricing struct {
					Prompt     json.RawMessage
					Completion json.RawMessage
				} `json:"pricing"`
			} `json:"data"`
		}
		if err := decodeModelResponse(response, &payload); err != nil {
			return nil, err
		}
		if payload.Data == nil {
			return nil, protocolError()
		}
		models := make([]CatalogModel, 0, len(payload.Data))
		seen := make(map[string]bool)
		for _, entry := range payload.Data {
			id := strings.TrimSpace(entry.ID)
			if id == "" || len(id) > 300 || seen[id] {
				continue
			}
			name := strings.TrimSpace(entry.DisplayName)
			if name == "" || len(name) > 200 {
				name = strings.TrimSpace(entry.Name)
			}
			if len(name) > 200 {
				name = ""
			}
			seen[id] = true
			meta := Metadata{Context: positive(entry.Context, entry.ContextWindow), OutputLimit: entry.TopProvider.MaxOutput, Modalities: entry.Architecture.Input, Brand: ProviderBrand(baseURL), PromptPrice: pricePerMillion(entry.Pricing.Prompt), CompletionPrice: pricePerMillion(entry.Pricing.Completion)}
			if meta.Context > 0 || meta.OutputLimit > 0 || len(meta.Modalities) > 0 || meta.PromptPrice != nil || meta.CompletionPrice != nil {
				meta.Source = "服务目录"
			}
			models = append(models, CatalogModel{ID: id, DisplayName: name, Metadata: meta})
		}
		return models, nil
	}
}

func pricePerMillion(value json.RawMessage) *float64 {
	price, err := strconv.ParseFloat(strings.Trim(string(value), `"`), 64)
	if err != nil || price < 0 || math.IsInf(price, 0) || math.IsNaN(price) {
		return nil
	}
	price *= 1_000_000
	if math.IsInf(price, 0) {
		return nil
	}
	return &price
}
