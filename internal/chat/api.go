package chat

import (
	"context"

	"github.com/coolerks/Fealdhord/internal/aiconfig"
)

type API struct{ service *Service }

func NewAPI(service *Service) *API { return &API{service: service} }

func (a *API) List(ctx context.Context, scope Scope) ([]Conversation, error) {
	if a.service == nil {
		return nil, modelError("StorageError")
	}
	return a.service.List(ctx, scope)
}

func (a *API) Create(ctx context.Context, scope Scope, title string) (Conversation, error) {
	if a.service == nil {
		return Conversation{}, modelError("StorageError")
	}
	return a.service.Create(ctx, scope, title)
}

func (a *API) Messages(ctx context.Context, id string) ([]Message, error) {
	if a.service == nil {
		return nil, modelError("StorageError")
	}
	return a.service.Messages(ctx, id)
}

func (a *API) Model(ctx context.Context) (aiconfig.Model, error) {
	if a.service == nil {
		return aiconfig.Model{}, modelError("StorageError")
	}
	return a.service.Model(ctx)
}

func (a *API) Send(ctx context.Context, input SendInput) error {
	if a.service == nil {
		return modelError("StorageError")
	}
	return a.service.Send(ctx, input)
}

func (a *API) Cancel(ctx context.Context, id string) error {
	if a.service == nil {
		return modelError("StorageError")
	}
	return a.service.Cancel(ctx, id)
}
