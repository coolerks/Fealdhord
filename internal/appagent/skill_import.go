package appagent

import (
	"archive/zip"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const maxSkillText = 256 << 10
const maxSkillResources = 12 << 10

type SkillResource struct {
	Path string `json:"path"`
	Text string `json:"text"`
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
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxSkillText+maxSkillResources {
		return Skill{}, errors.New("InvalidSkill")
	}
	if strings.EqualFold(filepath.Ext(filename), ".zip") {
		return unpackSkill(filename)
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
	return parseSkill(string(data), false)
}

func unpackSkill(filename string) (Skill, error) {
	invalid := errors.New("InvalidSkillPackage")
	archive, err := zip.OpenReader(filename)
	if err != nil {
		return Skill{}, invalid
	}
	defer archive.Close()
	if len(archive.File) > 32 {
		return Skill{}, invalid
	}
	root, found := "", false
	for _, file := range archive.File {
		name := strings.TrimSuffix(file.Name, "/")
		if !fs.ValidPath(name) || strings.Contains(name, "\\") || (file.Mode().Type() != 0 && !file.FileInfo().IsDir()) ||
			len(strings.Split(name, "/")) > 5 {
			return Skill{}, invalid
		}
		if path.Base(name) == "SKILL.md" && file.Mode().IsRegular() {
			if found || strings.Count(name, "/") > 1 {
				return Skill{}, invalid
			}
			root, found = strings.TrimSuffix(name, "SKILL.md"), true
		}
	}
	if !found {
		return Skill{}, invalid
	}
	var value Skill
	var resources []SkillResource
	for _, file := range archive.File {
		if file.FileInfo().IsDir() {
			continue
		}
		name, belongs := strings.CutPrefix(file.Name, root)
		if !belongs || name != "SKILL.md" && !skillResourcePath(name) {
			return Skill{}, invalid
		}
		limit := maxSkillResources
		if name == "SKILL.md" {
			limit = maxSkillText
		}
		if file.UncompressedSize64 > uint64(limit) {
			return Skill{}, invalid
		}
		entry, err := file.Open()
		if err != nil {
			return Skill{}, invalid
		}
		data, err := io.ReadAll(io.LimitReader(entry, int64(limit)+1))
		entry.Close()
		if err != nil || len(data) > limit {
			return Skill{}, invalid
		}
		if name == "SKILL.md" {
			value, err = parseSkill(string(data), false)
			if err != nil {
				return Skill{}, err
			}
		} else {
			resources = append(resources, SkillResource{Path: name, Text: string(data)})
		}
	}
	if !validateSkillResources(resources) {
		return Skill{}, invalid
	}
	value.Resources = resources
	return value, nil
}
