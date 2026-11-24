package appagent

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

type MCPScope struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

func scopeSupports(scope MCPScope, name string) bool {
	if scope.Kind == "" {
		return true
	}
	switch name {
	case "article.list", "article.search", "article.get", "article.get_metadata", "article.extract",
		"article.mark_read", "article.mark_unread", "article.favorite", "article.unfavorite":
		return true
	case "feed.get":
		return scope.Kind == "feed"
	case "category.get":
		return scope.Kind == "category"
	default:
		return false
	}
}

func (r *Registry) scopedTool(tool Tool, scope MCPScope) Tool {
	if scope.Kind == "" {
		return tool
	}
	invoke := tool.invoke
	if tool.Name == "article.list" || tool.Name == "article.search" {
		tool.invoke = func(ctx context.Context, raw json.RawMessage) (any, error) {
			var input map[string]json.RawMessage
			if json.Unmarshal(raw, &input) != nil {
				return nil, errors.New("ValidationError")
			}
			input["scope"], _ = json.Marshal(database.ArticleScope{Kind: scope.Kind, ID: scope.ID})
			args, _ := json.Marshal(input)
			return invoke(ctx, args)
		}
		return tool
	}
	verify := func(ctx context.Context, raw json.RawMessage) error {
		switch tool.Name {
		case "feed.get", "category.get":
			var input id
			if json.Unmarshal(raw, &input) != nil || input.ID != scope.ID {
				return errors.New("PermissionDenied")
			}
		default:
			var targets struct {
				ID  string   `json:"id"`
				IDs []string `json:"ids"`
			}
			if json.Unmarshal(raw, &targets) != nil {
				return errors.New("ValidationError")
			}
			articles := slices.Clone(targets.IDs)
			if targets.ID != "" {
				articles = append(articles, targets.ID)
			}
			if len(articles) == 0 || len(articles) > 100 || r.scopeDB == nil {
				return errors.New("PermissionDenied")
			}
			for _, articleID := range articles {
				count, err := r.scopeDB.ArticleInToolScope(ctx, sqlc.ArticleInToolScopeParams{
					ArticleID: articleID, ScopeKind: scope.Kind, ScopeID: scope.ID,
				})
				if err != nil || count != 1 {
					return errors.New("PermissionDenied")
				}
			}
		}
		return ctx.Err()
	}
	tool.verifyScope = verify
	tool.invoke = func(ctx context.Context, raw json.RawMessage) (any, error) {
		if err := verify(ctx, raw); err != nil {
			return nil, err
		}
		return invoke(ctx, raw)
	}
	return tool
}
