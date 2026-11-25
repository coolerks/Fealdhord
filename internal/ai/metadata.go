package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

type Metadata struct {
	Context         int      `json:"context,omitempty"`
	InputLimit      int      `json:"inputLimit,omitempty"`
	OutputLimit     int      `json:"outputLimit,omitempty"`
	Modalities      []string `json:"modalities,omitempty"`
	PromptPrice     *float64 `json:"promptPrice,omitempty"`
	CompletionPrice *float64 `json:"completionPrice,omitempty"`
	Source          string   `json:"source,omitempty"`
	UpdatedAt       string   `json:"updatedAt,omitempty"`
	Brand           string   `json:"brand,omitempty"`
	BrandOverride   string   `json:"brandOverride,omitempty"`
	ReferencePrice  bool     `json:"referencePrice,omitempty"`
}

func (metadata Metadata) SupportsImage() bool {
	return slices.Contains(metadata.Modalities, "image")
}

type CatalogIndex struct {
	directory string
	mu        sync.Mutex
	loadedAt  time.Time
	loading   chan struct{}
	models    map[string][]Metadata
	brands    map[string]bool
}

func NewCatalogIndex(directory string) *CatalogIndex { return &CatalogIndex{directory: directory} }

type directoryEntry struct {
	Canonical string                           `json:"canonical_model_id"`
	Provider  string                           `json:"provider"`
	Price     struct{ Input, Output *float64 } `json:"cost"`
	ID        string                           `json:"id"`
	Context   int                              `json:"contextWindow"`
	Output    int                              `json:"maxTokens"`
	Input     []string                         `json:"input"`
	Limits    struct {
		Context int
		Input   int
		Output  int
	} `json:"limit"`
	Modalities struct{ Input []string } `json:"modalities"`
}

func (index *CatalogIndex) Load(ctx context.Context) {
	index.mu.Lock()
	if active := index.loading; active != nil {
		index.mu.Unlock()
		select {
		case <-ctx.Done():
		case <-active:
		}
		return
	}
	if time.Since(index.loadedAt) < 30*time.Minute {
		index.mu.Unlock()
		return
	}
	index.loading = make(chan struct{})
	index.mu.Unlock()
	models := map[string][]Metadata{}
	brands := map[string]bool{}
	for _, source := range []struct{ name, url string }{
		{"Models.dev", "https://models.dev/models.json?type=all"},
		{"Models.dev providers", "https://models.dev/api.json"},
		{"Pi", "https://pi.dev/api/models"},
	} {
		var entries map[string]directoryEntry
		data := index.snapshot(ctx, source.name, source.url)
		if source.name == "Models.dev providers" {
			var providers map[string]struct {
				Models map[string]directoryEntry `json:"models"`
			}
			if json.Unmarshal(data, &providers) == nil {
				for provider, details := range providers {
					brands[provider] = true
					indexEntries(models, details.Models, source.name, provider)
				}
			}
		} else if source.name == "Pi" {
			var providers map[string]map[string]directoryEntry
			if json.Unmarshal(data, &providers) != nil {
				continue
			}
			for provider, entries := range providers {
				brands[provider] = true
				indexEntries(models, entries, source.name, provider)
			}
		} else if json.Unmarshal(data, &entries) == nil {
			indexEntries(models, entries, source.name, "")
		}
	}
	index.mu.Lock()
	defer index.mu.Unlock()
	close(index.loading)
	index.loading = nil
	index.loadedAt = time.Now()
	if len(models) > 0 {
		stamp := index.loadedAt.UTC().Format(time.RFC3339)
		for key, candidates := range models {
			for i := range candidates {
				candidates[i].UpdatedAt = stamp
			}
			models[key] = candidates
		}
		index.models = models
		index.brands = brands
	}
}

func indexEntries(models map[string][]Metadata, entries map[string]directoryEntry, source, brand string) {
	for id, entry := range entries {
		if entry.ID != "" {
			id = entry.ID
		}
		entryBrand := brand
		if entryBrand == "" {
			entryBrand = entry.Provider
		}
		if prefix, _, ok := strings.Cut(entry.Canonical, "/"); ok {
			entryBrand = prefix
		}
		if prefix, _, ok := strings.Cut(id, "/"); entryBrand == "" && ok {
			entryBrand = prefix
		}
		metadata := Metadata{Context: positive(entry.Context, entry.Limits.Context), InputLimit: entry.Limits.Input, OutputLimit: positive(entry.Output, entry.Limits.Output), Modalities: entry.Input, Brand: entryBrand, Source: source}
		if len(metadata.Modalities) == 0 {
			metadata.Modalities = entry.Modalities.Input
		}
		if source != "Models.dev" {
			metadata.PromptPrice = validCatalogPrice(entry.Price.Input)
			metadata.CompletionPrice = validCatalogPrice(entry.Price.Output)
			metadata.ReferencePrice = metadata.PromptPrice != nil || metadata.CompletionPrice != nil
		}
		for key := id; key != ""; {
			models[key] = append(models[key], metadata)
			_, next, ok := strings.Cut(key, "/")
			if !ok {
				break
			}
			key = next
		}
	}
}

func (index *CatalogIndex) Match(id string) Metadata {
	if index == nil {
		return Metadata{}
	}
	index.mu.Lock()
	defer index.mu.Unlock()
	for key := id; key != ""; {
		var matched Metadata
		for _, source := range []string{"Models.dev", "Models.dev providers", "Pi"} {
			var selected Metadata
			ambiguous := false
			for _, candidate := range index.models[key] {
				if candidate.Source != source {
					continue
				}
				if selected.Source != "" && (candidate.Context != selected.Context || candidate.OutputLimit != selected.OutputLimit || candidate.Brand != selected.Brand) {
					ambiguous = true
					break
				}
				selected = candidate
			}
			if !ambiguous && selected.Source != "" {
				matched = matched.WithFallback(selected)
			}
		}
		if matched.Source != "" {
			return matched
		}
		_, next, ok := strings.Cut(key, "/")
		if !ok {
			break
		}
		key = next
	}
	return Metadata{}
}

func (index *CatalogIndex) snapshot(ctx context.Context, name, endpoint string) []byte {
	const limit = 24 << 20
	cache := filepath.Join(index.directory, name+".json")
	old, _ := os.ReadFile(cache)
	if info, err := os.Stat(cache); err == nil && time.Since(info.ModTime()) < 24*time.Hour && json.Valid(old) {
		return old
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return old
	}
	client := http.Client{Timeout: 5 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		return old
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return old
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil || len(data) > limit || !json.Valid(data) {
		return old
	}
	if os.MkdirAll(index.directory, 0700) == nil {
		if file, err := os.CreateTemp(index.directory, "catalog-*.json"); err == nil {
			path := file.Name()
			_, writeErr := file.Write(data)
			if errors.Join(writeErr, file.Close()) == nil {
				_ = os.Rename(path, cache)
			}
			_ = os.Remove(path)
		}
	}
	return data
}

func (index *CatalogIndex) Brands() []string {
	if index == nil {
		return nil
	}
	index.mu.Lock()
	defer index.mu.Unlock()
	brands := make([]string, 0, len(index.brands))
	for brand := range index.brands {
		if brandName.MatchString(brand) {
			brands = append(brands, brand)
		}
	}
	slices.Sort(brands)
	return brands
}

func validCatalogPrice(price *float64) *float64 {
	if price == nil || *price < 0 || math.IsInf(*price, 0) || math.IsNaN(*price) {
		return nil
	}
	return price
}

func positive(first, fallback int) int {
	if first > 0 {
		return first
	}
	return max(0, fallback)
}

func (metadata Metadata) WithFallback(fallback Metadata) Metadata {
	if metadata.Context == 0 && metadata.InputLimit == 0 && metadata.OutputLimit == 0 && len(metadata.Modalities) == 0 && metadata.Source == "" {
		metadata.Source = fallback.Source
		metadata.UpdatedAt = fallback.UpdatedAt
	}
	metadata.Context = positive(metadata.Context, fallback.Context)
	metadata.InputLimit = positive(metadata.InputLimit, fallback.InputLimit)
	metadata.OutputLimit = positive(metadata.OutputLimit, fallback.OutputLimit)
	if len(metadata.Modalities) == 0 {
		metadata.Modalities = fallback.Modalities
	}
	if metadata.Brand == "" {
		metadata.Brand = fallback.Brand
	}
	if metadata.PromptPrice == nil && metadata.CompletionPrice == nil {
		metadata.PromptPrice = fallback.PromptPrice
		metadata.CompletionPrice = fallback.CompletionPrice
		metadata.ReferencePrice = fallback.ReferencePrice
	}
	return metadata
}

func ProviderBrand(baseURL string) string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	switch base.Hostname() {
	case "api.openai.com":
		return "openai"
	case "api.anthropic.com":
		return "anthropic"
	case "openrouter.ai":
		return "openrouter"
	case "api.deepseek.com":
		return "deepseek"
	case "generativelanguage.googleapis.com":
		return "google"
	}
	return ""
}
