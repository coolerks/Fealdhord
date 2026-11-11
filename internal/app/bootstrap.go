package app

import (
	"context"
	"crypto/rand"
	"runtime"
	"sync"
	"time"
)

const EventName = "fealdhord:event"
const historyLimit = 16

type operation struct {
	snapshot OperationSnapshot
	cancel   context.CancelFunc
}

// BootstrapService 提供无网络副作用的工程诊断；不冒充 RSS 刷新服务。
type BootstrapService struct {
	mu       sync.Mutex
	ctx      context.Context
	stop     context.CancelFunc
	wg       sync.WaitGroup
	emit     func(OperationEvent)
	interval time.Duration
	closed   bool
	activeID string
	history  []string
	jobs     map[string]*operation
}

// NewBootstrapService 同时返回收尾函数，收尾函数不暴露为前端绑定。
func NewBootstrapService(parent context.Context, emit func(OperationEvent)) (*BootstrapService, func()) {
	s := newBootstrapService(parent, emit, 250*time.Millisecond)
	return s, s.close
}

func newBootstrapService(parent context.Context, emit func(OperationEvent), interval time.Duration) *BootstrapService {
	ctx, stop := context.WithCancel(parent)
	return &BootstrapService{ctx: ctx, stop: stop, emit: emit, interval: interval, jobs: make(map[string]*operation)}
}

// 发布构建通过 Go 链接器注入对应的版本标签。
var version = "0.1.0-dev"

func (s *BootstrapService) GetInfo() InfoResult {
	return InfoResult{OK: true, RequestID: rand.Text(), Data: &Info{
		Name: "折栈 Fealdhord", Version: version, WailsVersion: "3.0.0",
		Platform: runtime.GOOS, Architecture: runtime.GOARCH,
	}}
}

// StartProbe 合并同时开始的诊断；每次只有一个活动任务。
func (s *BootstrapService) StartProbe() OperationResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.ctx.Err() != nil {
		return operationError("Unavailable", "应用正在关闭，请稍后重试。")
	}
	if s.activeID != "" {
		return operationOK(s.jobs[s.activeID].snapshot)
	}
	id := "probe_" + rand.Text()
	ctx, cancel := context.WithCancel(s.ctx)
	snapshot := OperationSnapshot{OperationID: id, Status: "running", Total: 20, Sequence: 1}
	s.jobs[id] = &operation{snapshot: snapshot, cancel: cancel}
	s.history = append(s.history, id)
	if len(s.history) > historyLimit {
		delete(s.jobs, s.history[0])
		s.history = s.history[1:]
	}
	s.activeID = id
	s.wg.Add(1)
	go s.run(ctx, id)
	return operationOK(snapshot)
}

func (s *BootstrapService) GetOperation(req OperationRequest) OperationResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[req.OperationID]
	if !ok {
		return missingOperation(req.OperationID)
	}
	return operationOK(job.snapshot)
}

// Cancel 请求取消但不预判终态；最终事实由 worker 快照决定。
func (s *BootstrapService) Cancel(req OperationRequest) OperationResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[req.OperationID]
	if !ok {
		return missingOperation(req.OperationID)
	}
	job.cancel()
	return operationOK(job.snapshot)
}

func (s *BootstrapService) run(ctx context.Context, id string) {
	defer s.wg.Done()
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
		case <-ticker.C:
		}
		s.mu.Lock()
		job := s.jobs[id]
		if ctx.Err() != nil {
			job.snapshot.Status = "cancelled"
		} else {
			job.snapshot.Completed++
			if job.snapshot.Completed == job.snapshot.Total {
				job.snapshot.Status = "succeeded"
			}
		}
		job.snapshot.Sequence++
		snapshot := job.snapshot
		terminal := snapshot.Status != "running"
		if terminal {
			job.cancel()
			s.activeID = ""
		}
		s.mu.Unlock()
		s.publish(snapshot, terminal)
		if terminal {
			return
		}
	}
}

func (s *BootstrapService) publish(snapshot OperationSnapshot, terminal bool) {
	if s.emit == nil {
		return
	}
	kind := "operation.progress"
	if terminal {
		kind = "operation.terminal"
	}
	s.emit(OperationEvent{
		SchemaVersion: 1, EventID: snapshot.OperationID + ":" + time.Now().UTC().Format(time.RFC3339Nano),
		OperationID: snapshot.OperationID, Sequence: snapshot.Sequence, Type: kind,
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), Payload: snapshot,
	})
}

func (s *BootstrapService) close() {
	s.mu.Lock()
	s.closed = true
	s.stop()
	s.mu.Unlock()
	s.wg.Wait()
}

func operationOK(snapshot OperationSnapshot) OperationResult {
	return OperationResult{OK: true, Data: &snapshot, RequestID: rand.Text()}
}

func operationError(code, message string) OperationResult {
	return OperationResult{Error: &AppError{Code: code, Message: message}, RequestID: rand.Text()}
}

func missingOperation(id string) OperationResult {
	if id == "" {
		return operationError("ValidationError", "缺少操作编号。")
	}
	return operationError("NotFound", "诊断记录不存在或已过期。")
}
