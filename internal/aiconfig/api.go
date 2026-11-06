package aiconfig

import "context"

// API exposes editable settings without exposing credential access or model execution.
type API struct{ service *Service }

func NewAPI(service *Service) *API { return &API{service: service} }

func (a *API) List(ctx context.Context) (Snapshot, error) {
	if a.service == nil {
		return Snapshot{}, configError("StorageError")
	}
	return a.service.List(ctx)
}

func (a *API) SaveProvider(ctx context.Context, input ProviderInput) (Provider, error) {
	if a.service == nil {
		return Provider{}, configError("StorageError")
	}
	return a.service.SaveProvider(ctx, input)
}

func (a *API) DeleteProvider(ctx context.Context, id string, revision int64) error {
	if a.service == nil {
		return configError("StorageError")
	}
	return a.service.DeleteProvider(ctx, id, revision)
}

func (a *API) SaveModel(ctx context.Context, input Model) (Model, error) {
	if a.service == nil {
		return Model{}, configError("StorageError")
	}
	return a.service.SaveModel(ctx, input)
}

func (a *API) DeleteModel(ctx context.Context, id string, revision int64) error {
	if a.service == nil {
		return configError("StorageError")
	}
	return a.service.DeleteModel(ctx, id, revision)
}

func (a *API) Assign(ctx context.Context, task, modelID string) error {
	if a.service == nil {
		return configError("StorageError")
	}
	return a.service.Assign(ctx, task, modelID)
}

func (a *API) TestConnection(ctx context.Context, modelID string) error {
	if a.service == nil {
		return configError("StorageError")
	}
	return a.service.TestConnection(ctx, modelID)
}
