package appagent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/coolerks/Fealdhord/internal/agentrun"
	"github.com/coolerks/Fealdhord/internal/ai"
	"go.yaml.in/yaml/v4"
)

type Skill struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Markdown    string          `json:"markdown"`
	Enabled     bool            `json:"enabled"`
	Tools       []string        `json:"tools"`
	Resources   []SkillResource `json:"resources,omitempty"`
	Revision    string          `json:"revision,omitempty"`
	ImportID    string          `json:"importId,omitempty"`
}

// JSON can expand each input byte and also stores the parsed front matter.
const maxStoredSkill = 12*maxSkillText + 6*maxSkillResources + (32 << 10)

var errSkillTextTooLarge = errors.New("SkillTextTooLarge")

var skillName = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$`)

func parseSkill(markdown string, enabled bool) (Skill, error) {
	var manifest struct {
		Name         string   `yaml:"name"`
		Description  string   `yaml:"description"`
		Tools        []string `yaml:"tools"`
		AllowedTools string   `yaml:"allowed-tools"`
	}
	if !utf8.ValidString(markdown) {
		return Skill{}, errors.New("SkillEncodingInvalid")
	}
	markdown = strings.ReplaceAll(strings.TrimPrefix(markdown, "\uFEFF"), "\r\n", "\n")
	if len(markdown) > maxSkillText {
		return Skill{}, errSkillTextTooLarge
	}
	input, ok := strings.CutPrefix(markdown, "---\n")
	front, body, found := strings.Cut(input, "\n---\n")
	if !ok || !found || yaml.Unmarshal([]byte(front), &manifest) != nil {
		return Skill{}, errors.New("SkillFrontMatterInvalid")
	}
	if !skillName.MatchString(manifest.Name) {
		return Skill{}, errors.New("SkillNameInvalid")
	}
	if strings.TrimSpace(manifest.Description) == "" || utf8.RuneCountInString(manifest.Description) > 1024 {
		return Skill{}, errors.New("SkillDescriptionInvalid")
	}
	if strings.TrimSpace(body) == "" {
		return Skill{}, errors.New("SkillBodyRequired")
	}
	return Skill{Name: manifest.Name, Description: manifest.Description, Markdown: markdown, Enabled: enabled,
		Tools: append(manifest.Tools, strings.Fields(manifest.AllowedTools)...)}, nil
}

func (a *Platform) Skills() []Skill {
	values := []Skill{}
	if a.dir == "" {
		return values
	}
	dir := filepath.Join(a.dir, "skills")
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		value, err := readSkillMetadata(filepath.Join(dir, entry.Name()))
		if err == nil {
			values = append(values, value)
		}
	}
	return values
}

func (a *Platform) saveSkill(markdown string, enabled bool) (Skill, error) {
	return a.saveSkillInput(Skill{Markdown: markdown, Enabled: enabled})
}

func (a *Platform) saveSkillInput(input Skill) (Skill, error) {
	a.skillEdit.Lock()
	defer a.skillEdit.Unlock()
	if input.ImportID != "" {
		return a.finishSkillImport(input)
	}
	markdown, enabled := input.Markdown, input.Enabled
	value, err := parseSkill(markdown, enabled)
	if err != nil {
		return Skill{}, err
	}
	prior, err := readSkill(filepath.Join(a.dir, "skills", value.Name+".json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Skill{}, errors.New("StorageError")
	}
	if input.Revision != "" && input.Revision != prior.Revision {
		return Skill{}, errors.New("SkillChanged")
	}
	if prior.Revision != "" && prior.Markdown == value.Markdown {
		value.Revision = prior.Revision
		return a.storeSkill(value)
	}
	if prior.Revision == "" && input.Resources != nil {
		for _, resource := range input.Resources {
			if resource.Text != "" {
				prior.Resources = input.Resources
				break
			}
		}
	}
	return a.saveSkillVersion(value, prior)
}

func (a *Platform) storeSkill(value Skill) (Skill, error) {
	if a.dir == "" {
		return Skill{}, errors.New("StorageError")
	}
	stored := value
	if stored.Revision != "" {
		stored.Markdown, stored.Resources = "", nil
	}
	data, err := json.Marshal(stored)
	if err != nil || len(data) > maxStoredSkill {
		return Skill{}, errors.New("InvalidSkillPackage")
	}
	if err = atomicFile(filepath.Join(a.dir, "skills"), value.Name+".json", data); err != nil {
		return Skill{}, errors.New("StorageError")
	}
	return value, nil
}

func (a *Platform) removeSkill(name string) error {
	a.skillEdit.Lock()
	defer a.skillEdit.Unlock()
	if !skillName.MatchString(name) {
		return errors.New("InvalidSkill")
	}
	if err := os.Remove(filepath.Join(a.dir, "skills", name+".json")); err != nil {
		return errors.New("StorageError")
	}
	return nil
}

func (a *Platform) RunTools(conversationID string, requested []string, disable bool) ([]agentrun.ReadTool, string, error) {
	if len(requested) > 3 {
		return nil, "", errors.New("InvalidSkill")
	}
	run := &skillRun{catalog: make(map[string]Skill), selected: make(map[string]bool), opened: make(map[string]bool)}
	skills := a.Skills()
	for _, skill := range skills {
		if skill.Enabled {
			run.catalog[skill.Name] = skill
		}
	}
	for _, name := range requested {
		if _, ok := run.catalog[name]; !ok || run.selected[name] {
			return nil, "", errors.New("InvalidSkill")
		}
		run.selected[name] = true
	}
	var instructions strings.Builder
	if disable {
		for _, name := range requested {
			value, err := a.resolveRunSkill(run, name)
			if err != nil {
				return nil, "", err
			}
			_, body, _ := strings.Cut(value.Markdown, "\n---\n")
			if instructions.Len()+len(body) <= 8<<10 {
				instructions.WriteString("\n\n[用户选择的 Skill]\n" + body)
			}
		}
		return nil, instructions.String(), nil
	}
	type entry struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Path        string `json:"path"`
		Selected    bool   `json:"selected,omitempty"`
	}
	var catalog []entry
	bytes := 0
	add := func(skill Skill, selected bool) {
		value := entry{skill.Name, skill.Description, "skill://" + skill.Name + "/SKILL.md", selected}
		data, _ := json.Marshal(value)
		if selected || bytes+len(data) <= 8<<10 {
			catalog = append(catalog, value)
			bytes += len(data) + 1
		} else {
			delete(run.catalog, skill.Name)
		}
	}
	for _, name := range requested {
		add(run.catalog[name], true)
	}
	for _, skill := range skills {
		if skill.Enabled && !run.selected[skill.Name] {
			add(skill, false)
		}
	}
	if len(catalog) > 0 {
		data, _ := json.Marshal(catalog)
		instructions.WriteString("\n可用 Skill 目录（名称、描述和路径）：")
		instructions.Write(data)
		instructions.WriteString("\n用 read 读取适用 Skill 的 SKILL.md，优先使用用户选中的 Skill。需要参考资料时，再用同一 skill:// 路径读取。技能说明可以指导任务流程，工具权限仅由本轮授权决定。")
	}
	var tools []agentrun.ReadTool
	for _, tool := range a.ChatTools(conversationID) {
		if !run.allows(tool.Definition.Name) {
			continue
		}
		read := tool.Read
		name := tool.Definition.Name
		tool.Read = func(ctx context.Context, call ai.ToolCall) (string, error) {
			if !run.allows(name) {
				return chatToolError(ctx, errors.New("ToolNotAllowed"))
			}
			return read(ctx, call)
		}
		tools = append(tools, tool)
	}
	for _, tool := range a.localChatTools(conversationID, run) {
		if tool.Definition.Name == "read" || run.allows(tool.Definition.Name) {
			tools = append(tools, tool)
		}
	}
	return tools, instructions.String(), nil
}

func readSkill(path string) (Skill, error) {
	value, err := readSkillMetadata(path)
	if err != nil || value.Revision == "" {
		return value, err
	}
	markdown, err := loadSkillFile(filepath.Join(filepath.Dir(filepath.Dir(path)), "skill-packages", value.Revision, "SKILL.md"))
	if err != nil || markdown.Name != value.Name {
		return Skill{}, errors.New("InvalidSkill")
	}
	value.Markdown = markdown.Markdown
	return value, nil
}

func readSkillMetadata(path string) (Skill, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return Skill{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxStoredSkill {
		return Skill{}, errors.New("InvalidSkill")
	}
	file, err := os.Open(path)
	if err != nil {
		return Skill{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxStoredSkill+1))
	var stored Skill
	if err != nil || len(data) > maxStoredSkill || json.Unmarshal(data, &stored) != nil {
		return Skill{}, errors.New("InvalidSkill")
	}
	if stored.Revision != "" {
		if !validSkillRevision(stored.Revision) || !skillName.MatchString(stored.Name) || stored.Description == "" {
			return Skill{}, errors.New("InvalidSkill")
		}
		return stored, nil
	}
	value, err := parseSkill(stored.Markdown, stored.Enabled)
	if err != nil || !validateSkillResources(stored.Resources) {
		return Skill{}, errors.New("InvalidSkill")
	}
	value.Resources = stored.Resources
	return value, nil
}
