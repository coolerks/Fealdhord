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
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

type Group struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s *Service) CreateGroup(ctx context.Context, name string, folders []string) (Group, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 320 || !utf8.ValidString(name) || len(folders) > 32 {
		return Group{}, modelError("ValidationError")
	}
	var roots []string
	var identities []os.FileInfo
	for _, folder := range folders {
		if !filepath.IsAbs(folder) || !utf8.ValidString(folder) {
			return Group{}, modelError("ChatFolderUnavailable")
		}
		root, err := filepath.EvalSymlinks(filepath.Clean(folder))
		if err != nil {
			return Group{}, modelError("ChatFolderUnavailable")
		}
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			return Group{}, modelError("ChatFolderUnavailable")
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
	group := Group{ID: rand.Text(), Name: name}
	if err := s.store.CreateChatGroup(ctx, sqlc.InsertChatGroupParams{
		ID: group.ID, Name: name, CreatedAt: time.Now().UnixMilli(),
	}, roots); err != nil {
		return Group{}, modelError("StorageError")
	}
	return group, nil
}

func (s *Service) Groups(ctx context.Context) ([]Group, error) {
	rows, err := s.store.ChatQueries().ListChatGroups(ctx)
	if err != nil {
		return nil, modelError("StorageError")
	}
	groups := make([]Group, 0, len(rows))
	for _, row := range rows {
		groups = append(groups, Group{ID: row.ID, Name: row.Name})
	}
	return groups, nil
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
	if _, err := s.store.ChatQueries().GetChatGroup(ctx, groupID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Conversation{}, modelError("ChatGroupUnavailable")
		}
		return Conversation{}, modelError("StorageError")
	}
	return s.createConversation(ctx, Scope{Kind: "global"}, title, sql.NullString{String: groupID, Valid: true})
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
