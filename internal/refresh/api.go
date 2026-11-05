package refresh

import "context"

// API 只暴露白名单操作，生命周期方法不注册为前端绑定。
type API struct{ service *Service }

func NewAPI(service *Service) *API { return &API{service: service} }
func (a *API) Start(ctx context.Context, request StartRequest) Result {
	return a.service.Start(ctx, request)
}
func (a *API) Get(ctx context.Context, request OperationRequest) Result {
	return a.service.Get(ctx, request)
}
func (a *API) Cancel(ctx context.Context, request OperationRequest) Result {
	return a.service.Cancel(ctx, request)
}

func (a *API) Current(context.Context) Result {
	a.service.mu.Lock()
	defer a.service.mu.Unlock()
	if a.service.active == nil {
		return success(Snapshot{Generation: a.service.generation, Status: "completed", Results: []FeedResult{}})
	}
	return success(snapshot(a.service.active))
}
