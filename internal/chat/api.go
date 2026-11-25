package chat

import (
	"context"

	"github.com/coolerks/Fealdhord/internal/aiconfig"
)

type API struct {
	service     *Service
	pickFolders func(context.Context) ([]string, error)
}

func NewAPI(service *Service, pickFolders ...func(context.Context) ([]string, error)) *API {
	api := &API{service: service}
	if len(pickFolders) != 0 {
		api.pickFolders = pickFolders[0]
	}
	return api
}

func (a *API) PickGroupFolders(ctx context.Context) ([]string, error) {
	if a.pickFolders == nil {
		return nil, modelError("Unavailable")
	}
	return a.pickFolders(ctx)
}

func (a *API) CreateGroup(ctx context.Context, name string, folders []string) (Group, error) {
	if a.service == nil {
		return Group{}, modelError("StorageError")
	}
	return a.service.CreateGroup(ctx, name, folders)
}

func (a *API) Groups(ctx context.Context) ([]Group, error) {
	if a.service == nil {
		return nil, modelError("StorageError")
	}
	return a.service.Groups(ctx)
}

func (a *API) GroupConversations(ctx context.Context, id string) ([]Conversation, error) {
	if a.service == nil {
		return nil, modelError("StorageError")
	}
	return a.service.GroupConversations(ctx, id)
}

func (a *API) CreateInGroup(ctx context.Context, id, title string) (Conversation, error) {
	if a.service == nil {
		return Conversation{}, modelError("StorageError")
	}
	return a.service.CreateInGroup(ctx, id, title)
}

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

func (a *API) Branch(ctx context.Context, id, messageID string) (Conversation, error) {
	if a.service == nil {
		return Conversation{}, modelError("StorageError")
	}
	return a.service.Branch(ctx, id, messageID)
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

func (a *API) Archived(ctx context.Context, offset int64) (ArchivedPage, error) {
	if a.service == nil {
		return ArchivedPage{}, modelError("StorageError")
	}
	return a.service.Archived(ctx, offset)
}

func (a *API) Rename(ctx context.Context, id, title string) (Conversation, error) {
	if a.service == nil {
		return Conversation{}, modelError("StorageError")
	}
	return a.service.Rename(ctx, id, title)
}

func (a *API) SetPinned(ctx context.Context, id string, pinned bool) (Conversation, error) {
	if a.service == nil {
		return Conversation{}, modelError("StorageError")
	}
	return a.service.SetPinned(ctx, id, pinned)
}

func (a *API) SetArchived(ctx context.Context, id string, archived bool) error {
	if a.service == nil {
		return modelError("StorageError")
	}
	return a.service.SetArchived(ctx, id, archived)
}

func (a *API) Delete(ctx context.Context, id string, archived bool) error {
	if a.service == nil {
		return modelError("StorageError")
	}
	return a.service.Delete(ctx, id, archived)
}
