package aiconfig

import (
	"context"

	"github.com/coolerks/Fealdhord/internal/ai"
)

func (a *API) FetchModels(ctx context.Context, providerID string) ([]ai.CatalogModel, error) {
	if a.service == nil {
		return nil, configError("StorageError")
	}
	return a.service.fetchModels(ctx, providerID)
}

func (s *Service) fetchModels(ctx context.Context, providerID string) ([]ai.CatalogModel, error) {
	s.credentialMu.Lock()
	provider, err := s.db.GetAIProvider(ctx, providerID)
	if err != nil {
		s.credentialMu.Unlock()
		return nil, conflictError(err)
	}
	key, err := s.providerKey(provider)
	s.credentialMu.Unlock()
	if err != nil {
		return nil, err
	}
	return ai.ListModels(ctx, provider.BaseUrl, key, provider.Protocol == "messages")
}
