package ai

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type CatalogModel struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
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
		response, err := client.do(request, anthropic)
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
				ID          string `json:"id"`
				DisplayName string `json:"display_name"`
				Name        string `json:"name"`
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
			models = append(models, CatalogModel{ID: id, DisplayName: name})
		}
		return models, nil
	}
}
