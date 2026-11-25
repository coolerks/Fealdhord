package appagent

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"unicode/utf8"
)

const maxSkillText = 256 << 10
const maxSkillResources = 12 << 10

type SkillResource struct {
	Path string `json:"path"`
	Text string `json:"text,omitempty"`
	Size int64  `json:"size,omitempty"`
}

func skillResourcePath(name string) bool {
	extension := strings.ToLower(path.Ext(name))
	return fs.ValidPath(name) && !strings.Contains(name, "\\") && len(name) <= 160 &&
		(strings.HasPrefix(name, "references/") || strings.HasPrefix(name, "assets/")) &&
		(extension == ".md" || extension == ".txt" || extension == ".json")
}

func validateSkillResources(resources []SkillResource) bool {
	if len(resources) > 12 {
		return false
	}
	total := 0
	seen := make(map[string]bool, len(resources))
	for _, resource := range resources {
		if !skillResourcePath(resource.Path) || !utf8.ValidString(resource.Text) || seen[resource.Path] {
			return false
		}
		total += len(resource.Text)
		if total > maxSkillResources {
			return false
		}
		seen[resource.Path] = true
	}
	return true
}

func loadSkillFile(filename string) (Skill, error) {
	info, err := os.Lstat(filename)
	if err != nil || !info.Mode().IsRegular() {
		return Skill{}, errors.New("SkillFileUnreadable")
	}
	if info.Size() > maxSkillText {
		return Skill{}, errSkillTextTooLarge
	}
	file, err := os.Open(filename)
	if err != nil {
		return Skill{}, errors.New("StorageError")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxSkillText+1))
	if err != nil {
		return Skill{}, errors.New("StorageError")
	}
	if len(data) > maxSkillText {
		return Skill{}, errSkillTextTooLarge
	}
	return parseSkill(string(data), false)
}
