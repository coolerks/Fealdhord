package chat

import (
	"context"

	"github.com/coolerks/Fealdhord/internal/aiconfig"
)

type DesktopHooks struct {
	PickFiles          func(context.Context) ([]string, error)
	PickFolders        func(context.Context) ([]string, error)
	ReadClipboardFiles func() []string
	SaveFile           func(ctx context.Context, title, filename, displayName, pattern string) (string, error)
}

type API struct {
	service     *Service
	pickFolders func(context.Context) ([]string, error)
	hooks       DesktopHooks
}

func NewAPI(service *Service, pickFolders ...func(context.Context) ([]string, error)) *API {
	api := &API{service: service}
	if len(pickFolders) != 0 {
		api.pickFolders = pickFolders[0]
		api.hooks.PickFolders = pickFolders[0]
	}
	return api
}

func NewAPIWithHooks(service *Service, hooks DesktopHooks) *API {
	api := &API{service: service, hooks: hooks, pickFolders: hooks.PickFolders}
	return api
}

func (a *API) PickAttachments(ctx context.Context, kind string) ([]AttachmentCandidate, error) {
	var paths []string
	var err error
	switch kind {
	case "files":
		if a.hooks.PickFiles == nil {
			return nil, modelError("Unavailable")
		}
		paths, err = a.hooks.PickFiles(ctx)
	case "folders":
		if a.hooks.PickFolders == nil {
			return nil, modelError("Unavailable")
		}
		paths, err = a.hooks.PickFolders(ctx)
	default:
		return nil, modelError("ValidationError")
	}
	if err != nil {
		return nil, err
	}
	return inspectPaths(paths), nil
}

func (a *API) InspectClipboardFiles() []AttachmentCandidate {
	if a.hooks.ReadClipboardFiles == nil {
		return nil
	}
	return inspectPaths(a.hooks.ReadClipboardFiles())
}

func (a *API) SaveMermaidImage(ctx context.Context, defaultName, format, data string) (bool, error) {
	if a.hooks.SaveFile == nil {
		return false, modelError("Unavailable")
	}
	if format != "svg" && format != "png" {
		return false, modelError("ValidationError")
	}
	title := "保存 Mermaid 图片"
	displayName := "SVG 矢量图"
	pattern := "*.svg"
	if format == "png" {
		displayName = "PNG 图片"
		pattern = "*.png"
	}
	if defaultName == "" {
		defaultName = "diagram." + format
	}
	path, err := a.hooks.SaveFile(ctx, title, defaultName, displayName, pattern)
	if err != nil {
		return false, err
	}
	if path == "" {
		return false, nil
	}
	if err := saveExportFile(path, format, data); err != nil {
		return false, err
	}
	return true, nil
}

func (a *API) Steer(ctx context.Context, input SteerInput) error {
	if a.service == nil {
		return modelError("StorageError")
	}
	return a.service.Steer(ctx, input)
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

func (a *API) SaveGroup(ctx context.Context, input GroupInput) (Group, error) {
	if a.service == nil {
		return Group{}, modelError("StorageError")
	}
	return a.service.SaveGroup(ctx, input)
}

func (a *API) GroupDetails(ctx context.Context, id string) (Group, error) {
	if a.service == nil {
		return Group{}, modelError("StorageError")
	}
	return a.service.GroupDetails(ctx, id)
}

func (a *API) ArchivedGroups(ctx context.Context) ([]Group, error) {
	if a.service == nil {
		return nil, modelError("StorageError")
	}
	return a.service.ArchivedGroups(ctx)
}

func (a *API) SetGroupArchived(ctx context.Context, id string, archived bool) error {
	if a.service == nil {
		return modelError("StorageError")
	}
	return a.service.SetGroupArchived(ctx, id, archived)
}

func (a *API) DeleteGroup(ctx context.Context, id string, archived bool) error {
	if a.service == nil {
		return modelError("StorageError")
	}
	return a.service.DeleteGroup(ctx, id, archived)
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
