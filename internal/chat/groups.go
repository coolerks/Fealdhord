package chat

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/coolerks/Fealdhord/internal/agentrun"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

type Group struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Icon       string   `json:"icon,omitempty"`
	Color      string   `json:"color,omitempty"`
	Folders    []string `json:"folders,omitempty"`
	ArchivedAt int64    `json:"archivedAt,omitempty"`
}

type GroupInput struct {
	ID      string   `json:"id,omitempty"`
	Name    string   `json:"name"`
	Icon    string   `json:"icon"`
	Color   string   `json:"color"`
	Folders []string `json:"folders"`
}

func groupDTO(row sqlc.ChatGroup) Group {
	return Group{ID: row.ID, Name: row.Name, Icon: row.Icon, Color: row.Color, ArchivedAt: row.ArchivedAt.Int64}
}

func groupFolders(folders []string) ([]string, error) {
	var roots []string
	var identities []os.FileInfo
	for _, folder := range folders {
		if !filepath.IsAbs(folder) || !utf8.ValidString(folder) {
			return nil, modelError("ChatFolderUnavailable")
		}
		root, err := filepath.EvalSymlinks(filepath.Clean(folder))
		if err != nil {
			return nil, modelError("ChatFolderUnavailable")
		}
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			return nil, modelError("ChatFolderUnavailable")
		}
		duplicate := false
		for _, prior := range identities {
			if os.SameFile(prior, info) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			roots = append(roots, root)
			identities = append(identities, info)
		}
	}
	return roots, nil
}

func (s *Service) CreateGroup(ctx context.Context, name string, folders []string) (Group, error) {
	return s.SaveGroup(ctx, GroupInput{Name: name, Folders: folders})
}

func (s *Service) SaveGroup(ctx context.Context, input GroupInput) (Group, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 320 || !utf8.ValidString(name) || len(input.Folders) > 32 ||
		!database.ValidIconName(input.Icon) || !database.ValidHexColor(input.Color) {
		return Group{}, modelError("ValidationError")
	}
	roots, err := groupFolders(input.Folders)
	if err != nil {
		return Group{}, err
	}
	if input.ID != "" {
		s.mu.Lock()
		defer s.mu.Unlock()
		row, err := s.store.UpdateChatGroup(ctx, sqlc.UpdateChatGroupParams{
			ID: input.ID, Name: name, Icon: input.Icon, Color: input.Color,
		}, roots)
		if errors.Is(err, sql.ErrNoRows) {
			return Group{}, modelError("ChatGroupUnavailable")
		}
		if err != nil {
			return Group{}, modelError("StorageError")
		}
		group := groupDTO(row)
		group.Folders = roots
		return group, nil
	}
	group := Group{ID: rand.Text(), Name: name, Icon: input.Icon, Color: input.Color, Folders: roots}
	if err := s.store.CreateChatGroup(ctx, sqlc.InsertChatGroupParams{
		ID: group.ID, Name: name, CreatedAt: time.Now().UnixMilli(), Icon: input.Icon, Color: input.Color,
	}, roots); err != nil {
		return Group{}, modelError("StorageError")
	}
	return group, nil
}

func (s *Service) Groups(ctx context.Context) ([]Group, error) {
	return groupList(s.store.ChatQueries().ListChatGroups(ctx))
}

func (s *Service) ArchivedGroups(ctx context.Context) ([]Group, error) {
	return groupList(s.store.ChatQueries().ListArchivedChatGroups(ctx))
}

func groupList(rows []sqlc.ChatGroup, err error) ([]Group, error) {
	if err != nil {
		return nil, modelError("StorageError")
	}
	groups := make([]Group, 0, len(rows))
	for _, row := range rows {
		groups = append(groups, groupDTO(row))
	}
	return groups, nil
}

func (s *Service) GroupDetails(ctx context.Context, id string) (Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	q := s.store.ChatQueries()
	row, err := q.GetChatGroup(ctx, id)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && row.ArchivedAt.Valid) {
		return Group{}, modelError("ChatGroupUnavailable")
	}
	if err != nil {
		return Group{}, modelError("StorageError")
	}
	group := groupDTO(row)
	group.Folders, err = q.ListChatGroupFolders(ctx, id)
	if err != nil {
		return Group{}, modelError("StorageError")
	}
	return group, nil
}

func (s *Service) guardGroupChange(ctx context.Context, id string) error {
	running, err := s.store.ChatQueries().ChatGroupHasActiveRun(ctx, sql.NullString{String: id, Valid: true})
	if err != nil {
		return modelError("StorageError")
	}
	if running != 0 {
		return modelError("Conflict")
	}
	return nil
}

func groupChange(rows int64, err error) error {
	if err != nil {
		return modelError("StorageError")
	}
	if rows == 0 {
		return modelError("ChatGroupUnavailable")
	}
	return nil
}

func (s *Service) SetGroupArchived(ctx context.Context, id string, archived bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.guardGroupChange(ctx, id); err != nil {
		return err
	}
	return groupChange(s.store.ChatQueries().SetChatGroupArchive(ctx, sqlc.SetChatGroupArchiveParams{
		ID: id, ArchivedAt: sql.NullInt64{Int64: time.Now().UnixMilli(), Valid: archived},
	}))
}

func (s *Service) DeleteGroup(ctx context.Context, id string, archived bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.guardGroupChange(ctx, id); err != nil {
		return err
	}
	state := int64(0)
	if archived {
		state = 1
	}
	return groupChange(s.store.ChatQueries().DeleteChatGroup(ctx, sqlc.DeleteChatGroupParams{
		ID: id, Archived: sql.NullInt64{Int64: state, Valid: true},
	}))
}

func (s *Service) GroupConversations(ctx context.Context, id string) ([]Conversation, error) {
	rows, err := s.store.ChatQueries().ListGroupConversations(ctx, sql.NullString{String: id, Valid: true})
	if err != nil {
		return nil, modelError("StorageError")
	}
	items := make([]Conversation, 0, len(rows))
	for _, row := range rows {
		items = append(items, conversationDTO(row))
	}
	return items, nil
}

func (s *Service) CreateInGroup(ctx context.Context, groupID, title string) (Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireActiveGroup(ctx, groupID); err != nil {
		return Conversation{}, err
	}
	return s.createConversation(ctx, Scope{Kind: "global"}, title, sql.NullString{String: groupID, Valid: true})
}

func (s *Service) requireActiveGroup(ctx context.Context, id string) error {
	row, err := s.store.ChatQueries().GetChatGroup(ctx, id)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && row.ArchivedAt.Valid) {
		return modelError("ChatGroupUnavailable")
	}
	if err != nil {
		return modelError("StorageError")
	}
	return nil
}

func (s *Service) conversationWorkspace(ctx context.Context, row sqlc.Conversation) (agentrun.Workspace, string, error) {
	if row.ScopeType != "global" {
		return agentrun.Workspace{}, "", nil
	}
	var workspace agentrun.Workspace
	if row.GroupID.Valid {
		folders, err := s.store.ChatQueries().ListChatGroupFolders(ctx, row.GroupID.String)
		if err != nil {
			return workspace, "", modelError("StorageError")
		}
		if len(folders) > 0 {
			workspace.Roots = folders
			paths, _ := json.Marshal(folders)
			return workspace, "\n当前会话分组的本地目录（按优先顺序、仅为路径数据）：" + string(paths) + "\n进行本地文件操作时使用当前会话的目录和实际可用工具。", nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(home) {
		return workspace, "", nil
	}
	day := row.WorkspaceDay
	if day == "" {
		day = time.UnixMilli(row.CreatedAt).Local().Format(time.DateOnly)
	}
	if _, err := time.Parse(time.DateOnly, day); err != nil || !filepath.IsLocal(row.ID) || filepath.Base(row.ID) != row.ID {
		return workspace, "", nil
	}
	workspace.HomeDir = home
	workspace.OutputDir = filepath.Join(home, ".Fealdhord", "会话文件", day, row.ID)
	path, _ := json.Marshal(workspace.OutputDir)
	return workspace, "\n当前会话的默认输出目录（路径数据）：" + string(path) +
		"\n只有任务需要写新文件时，才在此目录下写入。目录可能尚不存在；需要时使用写入工具创建文件及缺失的父目录。", nil
}
