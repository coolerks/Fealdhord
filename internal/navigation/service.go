// Package navigation 暴露分类管理、有效成员与全库导航，拒绝越权写入来源。
package navigation

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"

	"github.com/coolerks/Fealdhord/internal/app"
	"github.com/coolerks/Fealdhord/internal/database"
)

type Repository interface {
	Navigation(context.Context) (database.Navigation, error)
	SaveCategory(context.Context, database.CategoryInput) (string, error)
	PreviewCategoryDeletion(context.Context, string) (database.CategoryDeletion, error)
	DeleteCategoryChecked(context.Context, database.CategoryDeletion) error
	ArticleMemberships(context.Context, string) ([]database.MembershipInfo, error)
	SetMembership(context.Context, string, string, string, string, *float64) error
	ResetManualMembership(context.Context, string, string) error
}
type Service struct {
	repository Repository
	changed    func()
}

func NewService(repository Repository, changed func()) *Service {
	return &Service{repository: repository, changed: changed}
}

type IDRequest struct {
	ID string `json:"id"`
}
type Value struct {
	ID string `json:"id"`
}
type Result struct {
	OK        bool          `json:"ok"`
	Data      *Value        `json:"data,omitempty"`
	Error     *app.AppError `json:"error,omitempty"`
	RequestID string        `json:"request_id"`
}
type ListResult struct {
	OK        bool                 `json:"ok"`
	Data      *database.Navigation `json:"data,omitempty"`
	Error     *app.AppError        `json:"error,omitempty"`
	RequestID string               `json:"request_id"`
}
type PreviewResult struct {
	OK        bool                       `json:"ok"`
	Data      *database.CategoryDeletion `json:"data,omitempty"`
	Error     *app.AppError              `json:"error,omitempty"`
	RequestID string                     `json:"request_id"`
}
type MembershipPage struct {
	Items []database.MembershipInfo `json:"items"`
}
type MembershipResult struct {
	OK        bool            `json:"ok"`
	Data      *MembershipPage `json:"data,omitempty"`
	Error     *app.AppError   `json:"error,omitempty"`
	RequestID string          `json:"request_id"`
}
type MembershipRequest struct {
	ArticleID  string `json:"article_id"`
	CategoryID string `json:"category_id"`
	State      string `json:"state"`
}

func safeError(err error) *app.AppError {
	code, message, retry := "DatabaseError", "分类操作失败，请重试。", true
	switch {
	case errors.Is(err, database.ErrConflict):
		code, message = "Conflict", "分类已变化，请重新载入。"
	case errors.Is(err, database.ErrNotFound):
		code, message, retry = "NotFound", "目标已不存在。", false
	case errors.Is(err, database.ErrInvalidInput):
		code, message, retry = "ValidationError", "分类参数无效。", false
	case errors.Is(err, context.Canceled):
		code, message, retry = "Cancelled", "操作已取消。", false
	case strings.Contains(err.Error(), "category cycle"):
		code, message, retry = "ValidationError", "分类不能移到自身或后代。", false
	case strings.Contains(err.Error(), "category depth"):
		code, message, retry = "ValidationError", "分类层级最多五层。", false
	case strings.Contains(err.Error(), "UNIQUE constraint"):
		code, message = "Conflict", "同一上级下的分类名称重复，请先改名。"
	case strings.Contains(err.Error(), "FOREIGN KEY constraint"):
		code, message = "Conflict", "关联目标已变化，请重新载入。"
	}
	return &app.AppError{Code: code, Message: message, Retryable: retry}
}
func (s *Service) List(ctx context.Context) ListResult {
	out := ListResult{RequestID: rand.Text()}
	if s.repository == nil {
		out.Error = safeError(database.ErrNotFound)
		return out
	}
	data, err := s.repository.Navigation(ctx)
	if err != nil {
		out.Error = safeError(err)
		return out
	}
	out.OK = true
	out.Data = &data
	return out
}
func (s *Service) result(id string, err error) Result {
	out := Result{RequestID: rand.Text()}
	if err != nil {
		out.Error = safeError(err)
		return out
	}
	out.OK = true
	out.Data = &Value{ID: id}
	if s.changed != nil {
		s.changed()
	}
	return out
}
func (s *Service) Save(ctx context.Context, input database.CategoryInput) Result {
	if s.repository == nil {
		return s.result("", database.ErrNotFound)
	}
	id, err := s.repository.SaveCategory(ctx, input)
	return s.result(id, err)
}
func (s *Service) PreviewDelete(ctx context.Context, input IDRequest) PreviewResult {
	out := PreviewResult{RequestID: rand.Text()}
	if s.repository == nil || input.ID == "" || len(input.ID) > 256 {
		out.Error = safeError(database.ErrInvalidInput)
		return out
	}
	data, err := s.repository.PreviewCategoryDeletion(ctx, input.ID)
	if err != nil {
		out.Error = safeError(err)
		return out
	}
	out.OK = true
	out.Data = &data
	return out
}
func (s *Service) Remove(ctx context.Context, input database.CategoryDeletion) Result {
	if s.repository == nil || input.ID == "" || len(input.ID) > 256 {
		return s.result("", database.ErrInvalidInput)
	}
	return s.result(input.ID, s.repository.DeleteCategoryChecked(ctx, input))
}
func (s *Service) Memberships(ctx context.Context, input IDRequest) MembershipResult {
	out := MembershipResult{RequestID: rand.Text()}
	if s.repository == nil || input.ID == "" || len(input.ID) > 256 {
		out.Error = safeError(database.ErrInvalidInput)
		return out
	}
	data, err := s.repository.ArticleMemberships(ctx, input.ID)
	if err != nil {
		out.Error = safeError(err)
		return out
	}
	out.OK = true
	out.Data = &MembershipPage{Items: data}
	return out
}
func (s *Service) SetMembership(ctx context.Context, input MembershipRequest) Result {
	if s.repository == nil || input.ArticleID == "" || input.CategoryID == "" || len(input.ArticleID) > 256 || len(input.CategoryID) > 256 {
		return s.result("", database.ErrInvalidInput)
	}
	var err error
	switch input.State {
	case "auto":
		err = s.repository.ResetManualMembership(ctx, input.ArticleID, input.CategoryID)
	case "included", "excluded":
		err = s.repository.SetMembership(ctx, input.ArticleID, input.CategoryID, "manual", input.State, nil)
	default:
		err = database.ErrInvalidInput
	}
	return s.result(input.ArticleID, err)
}
