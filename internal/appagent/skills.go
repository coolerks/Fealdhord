package appagent

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/coolerks/Fealdhord/internal/agentrun"
	"go.yaml.in/yaml/v4"
)

type Skill struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Markdown    string   `json:"markdown"`
	Enabled     bool     `json:"enabled"`
	Tools       []string `json:"tools"`
}

var skillName = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$`)

func parseSkill(markdown string, enabled bool) (Skill, error) {
	var manifest struct {
		Name         string   `yaml:"name"`
		Description  string   `yaml:"description"`
		Tools        []string `yaml:"tools"`
		AllowedTools string   `yaml:"allowed-tools"`
	}
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	input, ok := strings.CutPrefix(markdown, "---\n")
	front, body, found := strings.Cut(input, "\n---\n")
	if !ok || !found || len(markdown) > 256<<10 || !utf8.ValidString(markdown) || strings.TrimSpace(body) == "" ||
		yaml.Unmarshal([]byte(front), &manifest) != nil || !skillName.MatchString(manifest.Name) || strings.TrimSpace(manifest.Description) == "" ||
		len([]rune(manifest.Description)) > 1024 {
		return Skill{}, errors.New("InvalidSkill")
	}
	return Skill{Name: manifest.Name, Description: manifest.Description, Markdown: markdown, Enabled: enabled,
		Tools: append(manifest.Tools, strings.Fields(manifest.AllowedTools)...)}, nil
}

func (a *Platform) Skills() []Skill {
	values := []Skill{}
	dir := filepath.Join(a.dir, "skills")
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if info, err := entry.Info(); err != nil || info.Size() > 512<<10 {
			continue
		}
		value, err := readSkill(filepath.Join(dir, entry.Name()))
		if err == nil {
			values = append(values, value)
		}
	}
	return values
}

func (a *Platform) saveSkill(markdown string, enabled bool) (Skill, error) {
	value, err := parseSkill(markdown, enabled)
	if err != nil {
		return Skill{}, err
	}
	if err = atomicJSON(filepath.Join(a.dir, "skills"), value.Name+".json", value); err != nil {
		return Skill{}, errors.New("StorageError")
	}
	return value, nil
}

func (a *Platform) removeSkill(name string) error {
	if !skillName.MatchString(name) {
		return errors.New("InvalidSkill")
	}
	if err := os.Remove(filepath.Join(a.dir, "skills", name+".json")); err != nil {
		return errors.New("StorageError")
	}
	return nil
}

func (a *Platform) RunTools(conversationID string, requested []string, disable bool) ([]agentrun.ReadTool, string, error) {
	var tools []agentrun.ReadTool
	if !disable {
		tools = a.ChatTools(conversationID)
	}
	if len(requested) == 0 {
		return tools, "", nil
	}
	if len(requested) > 3 {
		return nil, "", errors.New("InvalidSkill")
	}
	var instructions strings.Builder
	var allowed map[string]bool
	for _, name := range requested {
		if !skillName.MatchString(name) {
			return nil, "", errors.New("InvalidSkill")
		}
		value, err := readSkill(filepath.Join(a.dir, "skills", name+".json"))
		if err != nil || !value.Enabled {
			return nil, "", errors.New("InvalidSkill")
		}
		_, body, _ := strings.Cut(value.Markdown, "\n---\n")
		instructions.WriteString("\n\n[用户选择的 Skill]\n" + body)
		current := make(map[string]bool, len(value.Tools))
		for _, name := range value.Tools {
			key := strings.ReplaceAll(name, ".", "_")
			if allowed == nil || allowed[key] {
				current[key] = true
			}
		}
		allowed = current
	}
	return filterTools(tools, allowed), instructions.String(), nil
}

func filterTools(tools []agentrun.ReadTool, allowed map[string]bool) []agentrun.ReadTool {
	out := tools[:0]
	for _, tool := range tools {
		if allowed[tool.Definition.Name] {
			out = append(out, tool)
		}
	}
	return out
}

func readSkill(path string) (Skill, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 512<<10 {
		return Skill{}, errors.New("InvalidSkill")
	}
	file, err := os.Open(path)
	if err != nil {
		return Skill{}, err
	}
	defer file.Close()
	var stored Skill
	if json.NewDecoder(io.LimitReader(file, 512<<10)).Decode(&stored) != nil {
		return Skill{}, errors.New("InvalidSkill")
	}
	return parseSkill(stored.Markdown, stored.Enabled)
}
