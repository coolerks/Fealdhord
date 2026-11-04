package settings

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"time"

	"github.com/coolerks/Fealdhord/internal/app"
	"github.com/coolerks/Fealdhord/internal/database"
)

type Repository interface {
	LoadPreferences(context.Context) (database.PreferenceRecord, error)
	SavePreferences(context.Context, string, int64, int64) (bool, error)
}

type Snapshot struct {
	Preferences Preferences `json:"preferences"`
	Revision    int64       `json:"revision"`
}

type UpdateRequest struct {
	Preferences Preferences `json:"preferences"`
	Revision    int64       `json:"revision"`
}

type Result struct {
	OK        bool          `json:"ok"`
	Data      *Snapshot     `json:"data,omitempty"`
	Error     *app.AppError `json:"error,omitempty"`
	RequestID string        `json:"request_id"`
}

// Service 是设置的唯一绑定入口；所有写入由 revision 原子检查保护。
type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func failure(code, message string, retryable bool) Result {
	return Result{RequestID: rand.Text(), Error: &app.AppError{Code: code, Message: message, Retryable: retryable}}
}

func (s *Service) Get(ctx context.Context) Result {
	if s.repository == nil {
		return failure("DatabaseError", "本地设置暂时无法读取，请检查数据目录权限后重新启动。", true)
	}
	record, err := s.repository.LoadPreferences(ctx)
	if err != nil {
		return failure("DatabaseError", "无法读取本地设置，请重试。", true)
	}
	if record.SchemaVersion != 1 {
		return failure("ConfigurationError", "设置来自较新的应用版本，请更新应用。", false)
	}
	prefs := Defaults()
	if err := json.Unmarshal([]byte(record.JSON), &prefs); err != nil {
		return failure("ConfigurationError", "本地设置格式损坏，请恢复备份。", false)
	}
	prefs.normalize()
	return Result{OK: true, Data: &Snapshot{Preferences: prefs, Revision: record.Revision}, RequestID: rand.Text()}
}

func (s *Service) Update(ctx context.Context, req UpdateRequest) Result {
	if req.Revision < 0 {
		return failure("ValidationError", "设置版本无效。", false)
	}
	if err := req.Preferences.Validate(); err != nil {
		return failure("ValidationError", err.Error(), false)
	}
	if s.repository == nil {
		return failure("DatabaseError", "本地设置暂时无法保存，请重新启动。", true)
	}
	encoded, err := json.Marshal(req.Preferences)
	if err != nil || len(encoded) > 65536 {
		return failure("ValidationError", "设置内容超出允许范围。", false)
	}
	// 防止旧应用覆盖未来 schema；CAS 保证检查后的并发更新不会丢失。
	record, err := s.repository.LoadPreferences(ctx)
	if err != nil {
		return failure("DatabaseError", "无法读取本地设置，请重试。", true)
	}
	if record.SchemaVersion != 1 {
		return failure("ConfigurationError", "设置来自较新的应用版本，请更新应用。", false)
	}
	if record.Revision != req.Revision {
		return failure("Conflict", "设置已在其他窗口更新，请重新载入。", true)
	}
	ok, err := s.repository.SavePreferences(ctx, string(encoded), req.Revision, time.Now().UTC().UnixMilli())
	if err != nil {
		return failure("DatabaseError", "设置保存失败，请重试。", true)
	}
	if !ok {
		return failure("Conflict", "设置已在其他窗口更新，请重新载入。", true)
	}
	return Result{OK: true, Data: &Snapshot{Preferences: req.Preferences, Revision: req.Revision + 1}, RequestID: rand.Text()}
}
