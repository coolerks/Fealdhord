package transfer

import "context"

// API 仅注册白名单文件能力，生命周期、路径与编码函数不暴露绑定。
type API struct{ service *Service }

func NewAPI(service *Service) *API                          { return &API{service: service} }
func (a *API) ChooseImport(ctx context.Context) TokenResult { return a.service.choose(ctx, "import") }
func (a *API) ChooseExport(ctx context.Context) TokenResult { return a.service.choose(ctx, "export") }
func (a *API) PreviewOPML(ctx context.Context, request TokenRequest) PreviewResult {
	return a.service.preview(ctx, request)
}
func (a *API) ImportOPML(ctx context.Context, request ImportRequest) ImportResult {
	return a.service.importOPML(ctx, request)
}
func (a *API) ExportOPML(ctx context.Context, request TokenRequest) FileResult {
	return a.service.exportOPML(ctx, request)
}
func (a *API) Discard(_ context.Context, request DiscardRequest) ActionResult {
	return a.service.discard(request)
}
