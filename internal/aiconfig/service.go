// Package aiconfig manages local model configuration and credential references.
package aiconfig

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

type Service struct {
	db           *sqlc.Queries
	credentials  CredentialStore
	credentialMu sync.Mutex
}

func New(db *sqlc.Queries, credentials CredentialStore) *Service {
	return &Service{db: db, credentials: credentials}
}

type Provider struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Protocol      string `json:"protocol"`
	BaseURL       string `json:"baseUrl"`
	Enabled       bool   `json:"enabled"`
	HasCredential bool   `json:"hasCredential"`
	Revision      int64  `json:"revision"`
}

type ProviderInput struct {
	Provider
	APIKey *string `json:"apiKey"` // nil 保留，空字符串清除。
}

type Model struct {
	ID          string `json:"id"`
	ProviderID  string `json:"providerId"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Enabled     bool   `json:"enabled"`
	Revision    int64  `json:"revision"`
}

type Assignment struct {
	Task    string `json:"task"`
	ModelID string `json:"modelId"`
}

type Snapshot struct {
	Providers   []Provider   `json:"providers"`
	Models      []Model      `json:"models"`
	Assignments []Assignment `json:"assignments"`
}

func publicProvider(row sqlc.AiProvider) Provider {
	return Provider{ID: row.ID, Name: row.Name, Protocol: row.Protocol, BaseURL: row.BaseUrl,
		Enabled: row.Enabled != 0, HasCredential: row.CredentialRef != "", Revision: row.Revision}
}

func publicModel(row sqlc.AiModel) Model {
	return Model{ID: row.ID, ProviderID: row.ProviderID, Name: row.Model, DisplayName: row.DisplayName,
		Enabled: row.Enabled != 0, Revision: row.Revision}
}

func (s *Service) List(ctx context.Context) (Snapshot, error) {
	out := Snapshot{Providers: []Provider{}, Models: []Model{}, Assignments: []Assignment{}}
	providers, err := s.db.ListAIProviders(ctx)
	if err != nil {
		return out, storageError(err)
	}
	models, err := s.db.ListAIModels(ctx)
	if err != nil {
		return out, storageError(err)
	}
	assignments, err := s.db.ListModelAssignments(ctx)
	if err != nil {
		return out, storageError(err)
	}
	for _, row := range providers {
		out.Providers = append(out.Providers, publicProvider(row))
	}
	for _, row := range models {
		out.Models = append(out.Models, publicModel(row))
	}
	for _, row := range assignments {
		out.Assignments = append(out.Assignments, Assignment{Task: row.TaskType, ModelID: row.ModelID})
	}
	return out, nil
}

func endpoint(base string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(base))
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") ||
		u.User != nil || u.Fragment != "" || u.ForceQuery || u.RawQuery != "" {
		return nil, configError("ValidationError")
	}
	return u, nil
}

func (s *Service) SaveProvider(ctx context.Context, input ProviderInput) (Provider, error) {
	input.Name = strings.TrimSpace(input.Name)
	u, err := endpoint(input.BaseURL)
	if err != nil || input.Name == "" || len(input.Name) > 200 ||
		(input.APIKey != nil && (len(*input.APIKey) > 2048 || strings.ContainsAny(*input.APIKey, "\r\n\x00"))) {
		return Provider{}, configError("ValidationError")
	}
	switch input.Protocol {
	case "chat_completions", "responses", "messages":
	default:
		return Provider{}, configError("ValidationError")
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = "/v1"
	}
	s.credentialMu.Lock()
	defer s.credentialMu.Unlock()
	var previous sqlc.AiProvider
	if input.ID != "" {
		previous, err = s.db.GetAIProvider(ctx, input.ID)
		if err != nil {
			return Provider{}, conflictError(err)
		}
		if previous.Revision != input.Revision {
			return Provider{}, configError("Conflict")
		}
		old, err := endpoint(previous.BaseUrl)
		if err != nil {
			return Provider{}, err
		}
		if previous.CredentialRef != "" && input.APIKey == nil &&
			(old.Scheme != u.Scheme || !strings.EqualFold(old.Host, u.Host) || previous.Protocol != input.Protocol) {
			return Provider{}, configError("CredentialRequired")
		}
	}

	ref := previous.CredentialRef
	if input.APIKey != nil {
		ref = ""
		if *input.APIKey != "" {
			ref = rand.Text()
			if s.credentials.Set(ref, *input.APIKey) != nil {
				return Provider{}, configError("CredentialError")
			}
		}
	}
	now := time.Now().UTC().UnixMilli()
	var row sqlc.AiProvider
	if input.ID == "" {
		row, err = s.db.CreateAIProvider(ctx, sqlc.CreateAIProviderParams{ID: rand.Text(), Name: input.Name, Protocol: input.Protocol,
			BaseUrl: u.String(), CredentialRef: ref, Enabled: flag(input.Enabled), CreatedAt: now, UpdatedAt: now})
	} else {
		row, err = s.db.UpdateAIProvider(ctx, sqlc.UpdateAIProviderParams{ID: input.ID, Revision: input.Revision, Name: input.Name, Protocol: input.Protocol,
			BaseUrl: u.String(), CredentialRef: ref, Enabled: flag(input.Enabled), UpdatedAt: now})
	}
	if err != nil {
		if ref != "" && ref != previous.CredentialRef {
			_ = s.credentials.Delete(ref)
		}
		return Provider{}, conflictError(err)
	}
	if previous.CredentialRef != "" && previous.CredentialRef != ref {
		_ = s.credentials.Delete(previous.CredentialRef)
	}
	return publicProvider(row), nil
}

func (s *Service) DeleteProvider(ctx context.Context, id string, revision int64) error {
	s.credentialMu.Lock()
	defer s.credentialMu.Unlock()
	row, err := s.db.GetAIProvider(ctx, id)
	if err != nil {
		return conflictError(err)
	}
	count, err := s.db.DeleteAIProvider(ctx, sqlc.DeleteAIProviderParams{ID: id, Revision: revision})
	if err != nil {
		return storageError(err)
	}
	if count != 1 {
		return configError("Conflict")
	}
	if row.CredentialRef != "" {
		_ = s.credentials.Delete(row.CredentialRef)
	}
	return nil
}

func (s *Service) SaveModel(ctx context.Context, input Model) (Model, error) {
	input.Name, input.DisplayName = strings.TrimSpace(input.Name), strings.TrimSpace(input.DisplayName)
	if input.ProviderID == "" || input.Name == "" || len(input.Name) > 300 || len(input.DisplayName) > 200 {
		return Model{}, configError("ValidationError")
	}
	if input.DisplayName == "" {
		input.DisplayName = input.Name
	}
	now := time.Now().UTC().UnixMilli()
	var row sqlc.AiModel
	var err error
	if input.ID == "" {
		row, err = s.db.CreateAIModel(ctx, sqlc.CreateAIModelParams{ID: rand.Text(), ProviderID: input.ProviderID,
			Model: input.Name, DisplayName: input.DisplayName, Enabled: flag(input.Enabled), CreatedAt: now, UpdatedAt: now})
	} else {
		row, err = s.db.UpdateAIModel(ctx, sqlc.UpdateAIModelParams{ID: input.ID, ProviderID: input.ProviderID, Revision: input.Revision,
			Model: input.Name, DisplayName: input.DisplayName, Enabled: flag(input.Enabled), UpdatedAt: now})
	}
	if err != nil {
		return Model{}, conflictError(err)
	}
	return publicModel(row), nil
}

func (s *Service) DeleteModel(ctx context.Context, id string, revision int64) error {
	count, err := s.db.DeleteAIModel(ctx, sqlc.DeleteAIModelParams{ID: id, Revision: revision})
	if err != nil {
		return storageError(err)
	}
	if count != 1 {
		return configError("Conflict")
	}
	return nil
}

func (s *Service) Assign(ctx context.Context, task, modelID string) error {
	switch task {
	case "chat", "classification", "summary", "translation", "featured":
	default:
		return configError("ValidationError")
	}
	if modelID == "" {
		if err := s.db.ClearModelAssignment(ctx, task); err != nil {
			return storageError(err)
		}
		return nil
	}
	count, err := s.db.SetModelAssignment(ctx, sqlc.SetModelAssignmentParams{TaskType: task, ModelID: modelID})
	if err != nil {
		return storageError(err)
	}
	if count != 1 {
		return configError("ConfigurationError")
	}
	return nil
}

func (s *Service) Resolve(ctx context.Context, task string) (Model, ai.Model, error) {
	s.credentialMu.Lock()
	defer s.credentialMu.Unlock()
	row, err := s.db.ResolveModelAssignment(ctx, task)
	if errors.Is(err, sql.ErrNoRows) {
		return Model{}, nil, configError("ConfigurationError")
	}
	if err != nil {
		return Model{}, nil, storageError(err)
	}
	return s.adapter(row.AiProvider, row.AiModel)
}

func (s *Service) TestConnection(ctx context.Context, modelID string) error {
	model, adapter, err := s.enabledModel(ctx, modelID)
	if err != nil {
		return err
	}
	limit := 32
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	_, err = adapter.Generate(ctx, &ai.ModelRequest{Model: model.Name, MaxOutputTokens: &limit, Messages: []ai.Message{{Role: "user", Content: []ai.ContentBlock{{Type: "text", Text: "Reply OK."}}}}})
	return err
}

func (s *Service) enabledModel(ctx context.Context, id string) (Model, ai.Model, error) {
	s.credentialMu.Lock()
	defer s.credentialMu.Unlock()
	row, err := s.db.GetEnabledAIModel(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Model{}, nil, configError("ConfigurationError")
	}
	if err != nil {
		return Model{}, nil, storageError(err)
	}
	return s.adapter(row.AiProvider, row.AiModel)
}

func (s *Service) adapter(p sqlc.AiProvider, model sqlc.AiModel) (Model, ai.Model, error) {
	var err error
	var key string
	if p.CredentialRef != "" {
		key, err = s.credentials.Get(p.CredentialRef)
		if err != nil {
			return Model{}, nil, configError("CredentialError")
		}
	}
	var adapter ai.Model
	switch p.Protocol {
	case "chat_completions":
		adapter, err = ai.NewChatCompletions(p.BaseUrl, key, nil)
	case "responses":
		adapter, err = ai.NewResponses(p.BaseUrl, key, nil)
	case "messages":
		adapter, err = ai.NewMessages(p.BaseUrl, key, nil)
	default:
		err = configError("ConfigurationError")
	}
	return publicModel(model), adapter, err
}

func configError(code string) error { return &ai.ModelError{Code: code} }

func conflictError(err error) error {
	var sqliteErr *sqlite.Error
	if errors.Is(err, sql.ErrNoRows) || (errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE) {
		return configError("Conflict")
	}
	if errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY {
		return configError("ConfigurationError")
	}
	return storageError(err)
}

func storageError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return configError("Cancelled")
	case errors.Is(err, context.DeadlineExceeded):
		return configError("Timeout")
	default:
		return configError("StorageError")
	}
}

func flag(enabled bool) int64 {
	if enabled {
		return 1
	}
	return 0
}
