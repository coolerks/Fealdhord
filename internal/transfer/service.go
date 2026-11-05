// Package transfer 通过原生对话框能力执行OPML预览、确认导入和安全文件导出。
package transfer

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coolerks/Fealdhord/internal/app"
	"github.com/coolerks/Fealdhord/internal/database"
)

type Repository interface {
	PlanOPML(context.Context, []database.ImportOutline) (database.ImportPlan, error)
	ImportOPML(context.Context, database.ImportPlan, []string, bool) (database.ImportReport, error)
	OPMLSnapshot(context.Context) (database.OPMLSnapshot, error)
}
type Picker func(context.Context) (string, error)
type grant struct {
	path, purpose string
	info, dirInfo os.FileInfo
	expires       time.Time
}
type preview struct {
	token   string
	plan    database.ImportPlan
	name    string
	expires time.Time
}
type Service struct {
	processing             map[string]context.CancelFunc
	mu                     sync.Mutex
	repository             Repository
	pickImport, pickExport Picker
	tokens                 map[string]grant
	previews               map[string]preview
	now                    func() time.Time
	changed                func()
	queue                  func([]string) error
	ctx                    context.Context
	cancel                 context.CancelFunc
	beforeCommit           func() error
}

func NewService(repository Repository, pickImport, pickExport Picker, changed func(), queue func([]string) error) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{repository: repository, pickImport: pickImport, pickExport: pickExport, changed: changed, queue: queue, tokens: map[string]grant{}, previews: map[string]preview{}, processing: map[string]context.CancelFunc{}, now: time.Now, ctx: ctx, cancel: cancel}
}
func (s *Service) Close() { s.cancel(); s.mu.Lock(); clear(s.tokens); clear(s.previews); s.mu.Unlock() }
func (s *Service) operation(ctx context.Context) (context.Context, context.CancelFunc) {
	child, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(s.ctx, cancel)
	return child, func() { stop(); cancel() }
}
func (s *Service) expire() {
	for token, g := range s.tokens {
		if !s.now().Before(g.expires) {
			delete(s.tokens, token)
		}
	}
	for id, p := range s.previews {
		if !s.now().Before(p.expires) {
			delete(s.previews, id)
		}
	}
}
func safeError(err error) *app.AppError {
	code, message, retry := "IOError", "无法读取或写入所选文件，请重新选择。", true
	switch {
	case errors.Is(err, ErrDocument):
		code, message, retry = "ValidationError", "OPML 文件无效或超限，请检查格式、层级和大小。", false
	case errors.Is(err, ErrExpired):
		code, message = "PermissionError", "文件授权或预览已失效，请重新选择文件。"
	case errors.Is(err, ErrFileChanged):
		code, message = "Conflict", "所选文件或目录已变化，请重新选择。"
	case errors.Is(err, database.ErrConflict):
		code, message = "Conflict", "订阅或分类已变化，请重新生成预览。"
	case errors.Is(err, database.ErrInvalidInput):
		code, message, retry = "ValidationError", "导入选择或文件目标无效。", false
	case errors.Is(err, context.Canceled):
		code, message, retry = "Cancelled", "操作已取消。", false
	case errors.Is(err, context.DeadlineExceeded):
		code, message = "Timeout", "文件处理超时，请重试。"
	}
	return &app.AppError{Code: code, Message: message, Retryable: retry}
}

type TokenData struct {
	Token     string `json:"token"`
	Name      string `json:"name"`
	Cancelled bool   `json:"cancelled"`
}
type TokenResult struct {
	OK        bool          `json:"ok"`
	Data      *TokenData    `json:"data,omitempty"`
	Error     *app.AppError `json:"error,omitempty"`
	RequestID string        `json:"request_id"`
}
type TokenRequest struct {
	Token string `json:"dialog_token"`
}
type PreviewData struct {
	ID    string                   `json:"preview_id"`
	Name  string                   `json:"name"`
	Items []database.ImportOutline `json:"items"`
}
type PreviewResult struct {
	OK        bool          `json:"ok"`
	Data      *PreviewData  `json:"data,omitempty"`
	Error     *app.AppError `json:"error,omitempty"`
	RequestID string        `json:"request_id"`
}
type ImportRequest struct {
	PreviewID     string   `json:"preview_id"`
	Selected      []string `json:"selected"`
	ApplyExisting bool     `json:"apply_existing"`
	Refresh       bool     `json:"refresh"`
}
type Report struct {
	Report        database.ImportReport `json:"report"`
	RefreshQueued bool                  `json:"refresh_queued"`
	RefreshError  string                `json:"refresh_error"`
}
type ImportResult struct {
	OK        bool          `json:"ok"`
	Data      *Report       `json:"data,omitempty"`
	Error     *app.AppError `json:"error,omitempty"`
	RequestID string        `json:"request_id"`
}
type FileData struct {
	Name  string `json:"name"`
	Bytes int    `json:"bytes"`
}
type FileResult struct {
	OK        bool          `json:"ok"`
	Data      *FileData     `json:"data,omitempty"`
	Error     *app.AppError `json:"error,omitempty"`
	RequestID string        `json:"request_id"`
}
type DiscardRequest struct {
	Token     string `json:"dialog_token"`
	PreviewID string `json:"preview_id"`
}
type ActionResult struct {
	OK        bool          `json:"ok"`
	Error     *app.AppError `json:"error,omitempty"`
	RequestID string        `json:"request_id"`
}

func (s *Service) choose(ctx context.Context, purpose string) TokenResult {
	out := TokenResult{RequestID: rand.Text()}
	ctx, cancel := s.operation(ctx)
	defer cancel()
	picker := s.pickImport
	if purpose == "export" {
		picker = s.pickExport
	}
	if picker == nil {
		out.Error = safeError(database.ErrInvalidInput)
		return out
	}
	path, err := picker(ctx)
	if err != nil {
		out.Error = safeError(err)
		return out
	}
	if ctx.Err() != nil {
		out.Error = safeError(ctx.Err())
		return out
	}
	if path == "" {
		out.OK = true
		out.Data = &TokenData{Cancelled: true}
		return out
	}
	path, err = filepath.Abs(path)
	if err != nil {
		out.Error = safeError(err)
		return out
	}
	var info os.FileInfo
	if purpose == "import" {
		info, err = os.Stat(path)
		if err == nil && (!info.Mode().IsRegular() || info.Size() > maxFileBytes) {
			err = ErrDocument
		}
	} else {
		extension := strings.ToLower(filepath.Ext(path))
		if extension != ".opml" && extension != ".xml" {
			err = database.ErrInvalidInput
		} else {
			info, err = os.Lstat(path)
			if os.IsNotExist(err) {
				info = nil
				err = nil
			} else if err == nil && !info.Mode().IsRegular() {
				err = ErrFileChanged
			}
		}
	}
	if err != nil {
		out.Error = safeError(err)
		return out
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil || !dirInfo.IsDir() {
		if err == nil {
			err = ErrFileChanged
		}
		out.Error = safeError(err)
		return out
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire()
	if len(s.tokens) >= 8 {
		out.Error = &app.AppError{Code: "Conflict", Message: "文件操作过多，请先完成或取消已有操作。", Retryable: true}
		return out
	}
	token := rand.Text()
	s.tokens[token] = grant{path: path, purpose: purpose, info: info, dirInfo: dirInfo, expires: s.now().Add(10 * time.Minute)}
	out.OK = true
	out.Data = &TokenData{Token: token, Name: filepath.Base(path)}
	return out
}
func (s *Service) takeToken(token, purpose string, cancel context.CancelFunc) (grant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire()
	value, ok := s.tokens[token]
	if !ok || value.purpose != purpose || s.ctx.Err() != nil {
		return grant{}, ErrExpired
	}
	if len(s.processing) >= 8 {
		return grant{}, ErrExpired
	}
	delete(s.tokens, token)
	s.processing[token] = cancel
	return value, nil
}
func (s *Service) preview(ctx context.Context, request TokenRequest) PreviewResult {
	out := PreviewResult{RequestID: rand.Text()}
	ctx, cancel := s.operation(ctx)
	defer cancel()
	value, err := s.takeToken(request.Token, "import", cancel)
	if err != nil {
		out.Error = safeError(err)
		return out
	}
	defer s.finish(request.Token)
	file, err := os.Open(value.path)
	if err != nil {
		out.Error = safeError(err)
		return out
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(value.info, info) || info.Size() != value.info.Size() || !info.ModTime().Equal(value.info.ModTime()) {
		out.Error = safeError(ErrFileChanged)
		return out
	}
	data, err := io.ReadAll(io.LimitReader(file, maxFileBytes+1))
	if err != nil {
		out.Error = safeError(err)
		return out
	}
	items, err := parseOPML(ctx, data)
	if err != nil {
		out.Error = safeError(err)
		return out
	}
	if s.repository == nil {
		out.Error = safeError(database.ErrInvalidInput)
		return out
	}
	plan, err := s.repository.PlanOPML(ctx, items)
	if err != nil {
		out.Error = safeError(err)
		return out
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire()
	if ctx.Err() != nil {
		out.Error = safeError(ctx.Err())
		return out
	}
	if len(s.previews) >= 4 {
		out.Error = &app.AppError{Code: "Conflict", Message: "导入预览过多，请先完成或取消已有预览。", Retryable: true}
		return out
	}
	id := rand.Text()
	s.previews[id] = preview{token: request.Token, plan: plan, name: filepath.Base(value.path), expires: s.now().Add(10 * time.Minute)}
	out.OK = true
	out.Data = &PreviewData{ID: id, Name: filepath.Base(value.path), Items: plan.Items}
	return out
}
func (s *Service) importOPML(ctx context.Context, request ImportRequest) ImportResult {
	out := ImportResult{RequestID: rand.Text()}
	ctx, cancel := s.operation(ctx)
	defer cancel()
	s.mu.Lock()
	s.expire()
	value, ok := s.previews[request.PreviewID]
	if ok && len(s.processing) < 8 {
		delete(s.previews, request.PreviewID)
		s.processing[request.PreviewID] = cancel
	} else {
		ok = false
	}
	s.mu.Unlock()
	if !ok || s.repository == nil {
		out.Error = safeError(ErrExpired)
		return out
	}
	defer s.finish(request.PreviewID)
	report, err := s.repository.ImportOPML(ctx, value.plan, request.Selected, request.ApplyExisting)
	if err != nil {
		out.Error = safeError(err)
		return out
	}
	if s.changed != nil {
		s.changed()
	}
	result := &Report{Report: report}
	if request.Refresh && len(report.FeedIDs) > 0 {
		if s.queue != nil && s.queue(report.FeedIDs) == nil {
			result.RefreshQueued = true
		} else {
			result.RefreshError = "导入已保存，刷新暂未排入队列，可稍后手动刷新。"
		}
	}
	out.OK = true
	out.Data = result
	return out
}
func sameTarget(value grant) bool {
	dir, err := os.Stat(filepath.Dir(value.path))
	if err != nil || !os.SameFile(value.dirInfo, dir) {
		return false
	}
	current, err := os.Lstat(value.path)
	if value.info == nil {
		return os.IsNotExist(err)
	}
	return err == nil && current.Mode().IsRegular() && os.SameFile(value.info, current) && current.Size() == value.info.Size() && current.ModTime().Equal(value.info.ModTime())
}
func (s *Service) writeFile(ctx context.Context, value grant, data []byte) error {
	if !sameTarget(value) {
		return ErrFileChanged
	}
	file, err := os.CreateTemp(filepath.Dir(value.path), ".fealdhord-opml-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer func() {
		if err := os.Remove(temporary); err != nil && !os.IsNotExist(err) {
			slog.Warn("OPML 临时文件清理失败")
		}
	}()
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if s.ctx.Err() != nil {
		return context.Canceled
	}
	if !sameTarget(value) {
		return ErrFileChanged
	}
	if s.beforeCommit != nil {
		if err = s.beforeCommit(); err != nil {
			return err
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if s.ctx.Err() != nil {
		return context.Canceled
	}
	if value.info == nil {
		return os.Link(temporary, value.path)
	}
	return os.Rename(temporary, value.path)
}
func (s *Service) exportOPML(ctx context.Context, request TokenRequest) FileResult {
	out := FileResult{RequestID: rand.Text()}
	ctx, cancel := s.operation(ctx)
	defer cancel()
	value, err := s.takeToken(request.Token, "export", cancel)
	if err != nil {
		out.Error = safeError(err)
		return out
	}
	defer s.finish(request.Token)
	if s.repository == nil {
		out.Error = safeError(database.ErrInvalidInput)
		return out
	}
	snapshot, err := s.repository.OPMLSnapshot(ctx)
	if err != nil {
		out.Error = safeError(err)
		return out
	}
	data, err := encodeOPML(ctx, snapshot)
	if err == nil {
		err = s.writeFile(ctx, value, data)
	}
	if err != nil {
		out.Error = safeError(err)
		return out
	}
	out.OK = true
	out.Data = &FileData{Name: filepath.Base(value.path), Bytes: len(data)}
	return out
}
func (s *Service) finish(key string) { s.mu.Lock(); delete(s.processing, key); s.mu.Unlock() }
func (s *Service) discard(request DiscardRequest) ActionResult {
	s.mu.Lock()
	delete(s.tokens, request.Token)
	delete(s.previews, request.PreviewID)
	for id, p := range s.previews {
		if p.token == request.Token && request.Token != "" {
			delete(s.previews, id)
		}
	}
	callbacks := []context.CancelFunc{}
	for _, key := range []string{request.Token, request.PreviewID} {
		if cancel := s.processing[key]; cancel != nil {
			callbacks = append(callbacks, cancel)
		}
	}
	s.mu.Unlock()
	for _, cancel := range callbacks {
		cancel()
	}
	return ActionResult{OK: true, RequestID: rand.Text()}
}
