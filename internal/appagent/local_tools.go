package appagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coolerks/Fealdhord/internal/agentrun"
	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/invopop/jsonschema"
)

var localTools = []Tool{
	{Name: "read", Description: "读取文本文件和原始行号。支持工作区路径和 skill:// 路径，自动跳过二进制。"},
	{Name: "write", Description: "创建或覆盖工作区文件，并补齐缺失的父目录。", RequiresApproval: true},
	{Name: "edit", Description: "精确替换现有文件文本；默认要求唯一匹配，replaceAll 可替换全部。", RequiresApproval: true},
	{Name: "shell", Description: "经单次桌面审批运行命令。返回退出码和输出，用于列目录、搜索和运行脚本。", RequiresApproval: true},
}

type localWriteInput struct {
	Path    string  `json:"path" jsonschema:"required"`
	Content *string `json:"content" jsonschema:"required"`
}

type localEditInput struct {
	Path       string  `json:"path" jsonschema:"required"`
	OldText    string  `json:"oldText" jsonschema:"required"`
	NewText    *string `json:"newText" jsonschema:"required"`
	ReplaceAll bool    `json:"replaceAll,omitempty"`
}

type fileTarget struct {
	root, home, relative, path string
}

func resolveFileTarget(ctx context.Context, name string) (fileTarget, error) {
	if name == "" || strings.ContainsRune(name, 0) {
		return fileTarget{}, errors.New("ValidationError")
	}
	workspace := agentrun.WorkspaceFrom(ctx)
	if workspace.OutputDir != "" {
		if !filepath.IsAbs(name) {
			if !filepath.IsLocal(name) {
				return fileTarget{}, errors.New("PathNotAllowed")
			}
			name = filepath.Join(workspace.OutputDir, name)
		}
		if relative, ok := withinDirectory(workspace.OutputDir, name); ok && relative != "." {
			return fileTarget{workspace.OutputDir, workspace.HomeDir, relative, filepath.Clean(name)}, nil
		}
		return fileTarget{}, errors.New("PathNotAllowed")
	}
	if !filepath.IsAbs(name) {
		if !filepath.IsLocal(name) {
			return fileTarget{}, errors.New("PathNotAllowed")
		}
		for _, root := range workspace.Roots {
			test, err := openLocalRoot(root)
			if err == nil {
				test.Close()
				name = filepath.Join(root, name)
				break
			}
		}
	}
	for _, root := range workspace.Roots {
		if relative, ok := withinDirectory(root, name); ok && relative != "." {
			return fileTarget{root, "", relative, filepath.Clean(name)}, nil
		}
	}
	return fileTarget{}, errors.New("PathNotAllowed")
}

func resolveReadTarget(ctx context.Context, name string) (fileTarget, error) {
	if target, err := resolveFileTarget(ctx, name); err == nil {
		return target, nil
	}
	if name == "" || strings.ContainsRune(name, 0) || !filepath.IsAbs(name) {
		return fileTarget{}, errors.New("PathNotAllowed")
	}
	clean := filepath.Clean(name)
	workspace := agentrun.WorkspaceFrom(ctx)
	for _, allowed := range workspace.ReadOnlyPaths {
		if allowed == "" {
			continue
		}
		info, err := os.Stat(allowed)
		if err != nil {
			continue
		}
		if info.IsDir() {
			if relative, ok := withinDirectory(allowed, clean); ok && relative != "." {
				return fileTarget{root: allowed, relative: relative, path: clean}, nil
			}
		} else if clean == filepath.Clean(allowed) {
			return fileTarget{root: filepath.Dir(clean), relative: filepath.Base(clean), path: clean}, nil
		}
	}
	return fileTarget{}, errors.New("PathNotAllowed")
}

func (target fileTarget) open(create bool) (*os.Root, error) {
	if target.home != "" {
		return openOutputRoot(target.home, target.root, create)
	}
	real, err := filepath.EvalSymlinks(target.root)
	if err != nil || filepath.Clean(real) != filepath.Clean(target.root) {
		return nil, errors.New("DirectoryUnavailable")
	}
	return openLocalRoot(target.root)
}

type skillRun struct {
	catalog  map[string]Skill
	selected map[string]bool
	opened   map[string]bool
}

func (r *skillRun) allows(tool string) bool {
	for name, skill := range r.catalog {
		if skill.Tools == nil || (!r.selected[name] && !r.opened[name]) {
			continue
		}
		found := false
		for _, declared := range skill.Tools {
			if strings.ReplaceAll(declared, ".", "_") == tool {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (a *Platform) resolveRunSkill(run *skillRun, name string) (Skill, error) {
	value, ok := run.catalog[name]
	if !ok || !skillName.MatchString(name) {
		return Skill{}, errors.New("InvalidSkill")
	}
	current, err := readSkill(filepath.Join(a.dir, "skills", name+".json"))
	if err != nil || !current.Enabled || current.Revision != value.Revision || (value.Revision == "" && current.Markdown != value.Markdown) {
		return Skill{}, errors.New("InvalidSkill")
	}
	return current, nil
}

func (a *Platform) readRunSkill(ctx context.Context, run *skillRun, input localReadInput) (localReadPage, error) {
	name, relative, found := strings.Cut(strings.TrimPrefix(input.Path, "skill://"), "/")
	if !found || !validPackagePath(relative) {
		return localReadPage{}, errors.New("PathNotAllowed")
	}
	value, err := a.resolveRunSkill(run, name)
	if err != nil {
		return localReadPage{}, err
	}
	budget := agentrun.LocalOutputBudget(ctx, localReadBytes)
	if relative == "SKILL.md" {
		page, err := readLocalPage(ctx, strings.NewReader(value.Markdown), input, budget)
		if err == nil && page.Text != "" {
			run.opened[name] = true
		}
		return page, err
	}
	if !run.opened[name] {
		return localReadPage{}, errors.New("SkillNotActivated")
	}
	if value.Revision == "" {
		for _, resource := range value.Resources {
			if resource.Path == relative {
				return readLocalPage(ctx, strings.NewReader(resource.Text), input, budget)
			}
		}
		return localReadPage{}, errors.New("FileUnavailable")
	}
	root, err := openLocalRoot(a.skillDirectory(value))
	if err != nil {
		return localReadPage{}, err
	}
	defer root.Close()
	file, err := regularLocalFile(root, filepath.FromSlash(relative))
	if err != nil {
		return localReadPage{}, err
	}
	defer file.Close()
	return readLocalPage(ctx, file, input, budget)
}

type localApproval func(context.Context, json.RawMessage) error

func buildLocalTool[I any](a *Platform, conversationID string, tool Tool, invoke func(context.Context, I, localApproval) (any, error)) agentrun.ReadTool {
	reflector := jsonschema.Reflector{DoNotReference: true, ExpandedStruct: true, RequiredFromJSONSchemaTags: true}
	schema, _ := json.Marshal(reflector.Reflect(new(I)))
	principal := "会话 " + conversationID
	return agentrun.ReadTool{
		Definition:       ai.ToolDefinition{Name: tool.Name, Description: tool.Description, InputSchema: schema},
		MaxResultBytes:   localReadBytes,
		PrivateActivity:  true,
		RequiresApproval: tool.RequiresApproval,
		Read: func(ctx context.Context, call ai.ToolCall) (result string, outErr error) {
			start := time.Now()
			var callErr error
			defer func() { a.registry.audit(principal, tool.Name, toolAuditStatus(result, callErr), start) }()
			var input I
			raw := json.RawMessage(call.ArgumentsJSON)
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.DisallowUnknownFields()
			if len(raw) > 64<<10 || !strings.HasPrefix(strings.TrimSpace(string(raw)), "{") ||
				decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF {
				callErr = errors.New("ValidationError")
				return chatToolError(ctx, callErr)
			}
			authorize := func(ctx context.Context, override json.RawMessage) error {
				if err := ctx.Err(); err != nil {
					return err
				}
				if override == nil {
					override = raw
				}
				if tool.Name == "shell" {
					mode := agentrun.ApprovalMode(ctx)
					if mode == agentrun.ApprovalDeny || (mode == agentrun.ApprovalCustom && agentrun.ToolPermission(ctx, toolPermissionID(tool)) == agentrun.ApprovalDeny) {
						return errors.New("PermissionDenied")
					}
					return a.registry.requestApproval(ctx, principal, conversationID, tool.Name, override)
				}
				return a.registry.authorizeTool(ctx, principal, conversationID, tool, override)
			}
			out, err := invoke(ctx, input, authorize)
			callErr = err
			if err != nil {
				// Do not propagate OS errors, which can include absolute paths.
				if strings.ContainsAny(err.Error(), " /\\:") {
					err = errors.New("LocalToolError")
				}
				return chatToolError(ctx, err)
			}
			data, err := json.Marshal(out)
			if err != nil || len(data) > localReadBytes {
				callErr = errors.New("ToolResultLimit")
				return chatToolError(ctx, callErr)
			}
			return string(data), nil
		},
	}
}

func (a *Platform) localChatTools(conversationID string, run *skillRun) []agentrun.ReadTool {
	shell, shellErr := a.resolveLocalShell()
	shellTool := localTools[3]
	if shellErr == nil {
		shellTool.Description += " 当前命令解释器为 " + shell.Label + "。请使用该 Shell 的语法。"
	} else {
		shellTool.Description += " 当前 Shell 不可用，请让用户检查工具权限设置。"
	}
	return []agentrun.ReadTool{
		buildLocalTool(a, conversationID, localTools[0], func(ctx context.Context, input localReadInput, authorize localApproval) (any, error) {
			if err := authorize(ctx, nil); err != nil {
				return nil, err
			}
			if strings.HasPrefix(input.Path, "skill://") {
				return a.readRunSkill(ctx, run, input)
			}
			if !run.allows("read") {
				return nil, errors.New("ToolNotAllowed")
			}
			target, err := resolveReadTarget(ctx, input.Path)
			if err != nil {
				return nil, err
			}
			root, err := target.open(false)
			if err != nil {
				return nil, err
			}
			defer root.Close()
			file, err := regularLocalFile(root, target.relative)
			if err != nil {
				return nil, err
			}
			defer file.Close()
			return readLocalPage(ctx, file, input, agentrun.LocalOutputBudget(ctx, localReadBytes))
		}),
		buildLocalTool(a, conversationID, localTools[1], func(ctx context.Context, input localWriteInput, authorize localApproval) (any, error) {
			if input.Content == nil || !run.allows("write") {
				return nil, errors.New("ToolNotAllowed")
			}
			target, err := resolveFileTarget(ctx, input.Path)
			if err != nil {
				return nil, err
			}
			var before os.FileInfo
			old, err := target.open(false)
			if err == nil {
				before, err = inspectWriteTarget(old, target.relative)
				old.Close()
				if err != nil {
					return nil, err
				}
			} else if target.home == "" {
				return nil, err
			}
			if err := authorize(ctx, nil); err != nil {
				return nil, err
			}
			a.localFileEdit.Lock()
			defer a.localFileEdit.Unlock()
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			root, err := target.open(true)
			if err != nil {
				return nil, err
			}
			defer root.Close()
			err = replaceLocalFile(ctx, root, target.relative, before, []byte(*input.Content))
			return map[string]any{"path": target.path, "bytes": len(*input.Content)}, err
		}),
		buildLocalTool(a, conversationID, localTools[2], func(ctx context.Context, input localEditInput, authorize localApproval) (any, error) {
			if !run.allows("edit") {
				return nil, errors.New("ToolNotAllowed")
			}
			if input.OldText == "" || input.NewText == nil {
				return nil, errors.New("ValidationError")
			}
			target, err := resolveFileTarget(ctx, input.Path)
			if err != nil {
				return nil, err
			}
			root, err := target.open(false)
			if err != nil {
				return nil, err
			}
			defer root.Close()
			before, err := inspectWriteTarget(root, target.relative)
			if err != nil || before == nil {
				return nil, errors.New("FileUnavailable")
			}
			if before.Size() > maxEditableFile {
				return nil, errors.New("FileTooLarge")
			}
			if err := authorize(ctx, nil); err != nil {
				return nil, err
			}
			a.localFileEdit.Lock()
			defer a.localFileEdit.Unlock()
			current, err := inspectWriteTarget(root, target.relative)
			if err != nil || !sameWriteTarget(before, current) {
				return nil, errors.New("FileChanged")
			}
			file, err := regularLocalFile(root, target.relative)
			if err != nil {
				return nil, err
			}
			data, count, err := editLocalText(ctx, file, input.OldText, *input.NewText, input.ReplaceAll)
			file.Close()
			if err != nil {
				return nil, err
			}
			err = replaceLocalFile(ctx, root, target.relative, before, data)
			return map[string]any{"path": target.path, "replacements": count}, err
		}),
		buildLocalTool(a, conversationID, shellTool, func(ctx context.Context, input localShellInput, authorize localApproval) (any, error) {
			if !run.allows("shell") {
				return nil, errors.New("ToolNotAllowed")
			}
			if shellErr != nil {
				return nil, shellErr
			}
			return a.runLocalShell(ctx, run, shell, input, authorize)
		}),
	}
}
