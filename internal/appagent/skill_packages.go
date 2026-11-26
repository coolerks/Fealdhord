package appagent

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"
)

const (
	maxPackageZip      = 64 << 20
	maxPackageFile     = 64 << 20
	maxPackageExpanded = 256 << 20
	maxPackageEntries  = 2048
)

type skillImport struct {
	skill   Skill
	dir     string
	expires time.Time
	prior   [32]byte
}

func validSkillRevision(id string) bool {
	return len(id) == 26 && strings.IndexFunc(id, func(r rune) bool {
		return (r < 'A' || r > 'Z') && (r < '2' || r > '7')
	}) < 0
}

func validPackagePath(name string) bool {
	if !fs.ValidPath(name) || name == "." || len(name) > 1024 || strings.ContainsAny(name, "\\:<>\"|?*") {
		return false
	}
	parts := strings.Split(name, "/")
	if len(parts) > 16 {
		return false
	}
	for _, part := range parts {
		base, _, _ := strings.Cut(strings.ToUpper(part), ".")
		if strings.HasSuffix(part, ".") || strings.TrimSpace(part) != part || base == "CON" || base == "PRN" ||
			base == "NUL" || base == "AUX" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) &&
			base[3] >= '0' && base[3] <= '9') || strings.IndexFunc(part, func(r rune) bool { return r < 32 }) >= 0 {
			return false
		}
	}
	return true
}

func (a *Platform) skillDetails(name string) (Skill, error) {
	if a.dir == "" || !skillName.MatchString(name) {
		return Skill{}, errors.New("InvalidSkill")
	}
	skill, err := readSkill(filepath.Join(a.dir, "skills", name+".json"))
	if err != nil {
		return Skill{}, errors.New("InvalidSkill")
	}
	if skill.Revision != "" {
		root := a.skillDirectory(skill)
		err := filepath.WalkDir(root, func(filename string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			relative, err := filepath.Rel(root, filename)
			if err != nil || !entry.Type().IsRegular() {
				return errors.New("InvalidSkill")
			}
			if relative == "SKILL.md" {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			skill.Resources = append(skill.Resources, SkillResource{Path: filepath.ToSlash(relative), Size: info.Size()})
			return nil
		})
		if err != nil {
			return Skill{}, errors.New("InvalidSkill")
		}
	}
	for i := range skill.Resources {
		resource := &skill.Resources[i]
		if resource.Text != "" {
			resource.Size = int64(len(resource.Text))
			resource.Text = ""
		}
	}
	return skill, nil
}

func (a *Platform) setSkillEnabled(name string, enabled bool) (Skill, error) {
	a.skillEdit.Lock()
	defer a.skillEdit.Unlock()
	if a.dir == "" || !skillName.MatchString(name) {
		return Skill{}, errors.New("InvalidSkill")
	}
	skill, err := readSkill(filepath.Join(a.dir, "skills", name+".json"))
	if err != nil {
		return Skill{}, errors.New("InvalidSkill")
	}
	skill.Enabled = enabled
	return a.storeSkill(skill)
}

func (a *Platform) previewSkillImport(ctx context.Context, filename string) (Skill, error) {
	info, err := os.Lstat(filename)
	if err != nil || !info.Mode().IsRegular() {
		return Skill{}, errors.New("SkillFileUnreadable")
	}
	if !strings.EqualFold(filepath.Ext(filename), ".zip") {
		return loadSkillFile(filename)
	}
	if info.Size() > maxPackageZip {
		return Skill{}, errors.New("SkillArchiveTooLarge")
	}
	if a.dir == "" {
		return Skill{}, errors.New("StorageError")
	}
	a.skillEdit.Lock()
	defer a.skillEdit.Unlock()
	for id, preview := range a.skillImports {
		if time.Now().After(preview.expires) {
			a.dropSkillImport(id)
		}
	}
	if len(a.skillImports) >= 4 {
		return Skill{}, errors.New("SkillImportLimit")
	}
	root := filepath.Join(a.dir, "skill-packages")
	if err := os.MkdirAll(root, 0700); err != nil {
		return Skill{}, errors.New("StorageError")
	}
	dir, err := os.MkdirTemp(root, "import-")
	if err != nil {
		return Skill{}, errors.New("StorageError")
	}
	skill, err := extractSkillPackage(ctx, filename, dir)
	if err != nil {
		_ = os.RemoveAll(dir)
		return Skill{}, err
	}
	skill.ImportID = rand.Text()
	if a.skillImports == nil {
		a.skillImports = make(map[string]skillImport)
	}
	prior, err := a.skillVersionStamp(skill.Name)
	if err != nil {
		_ = os.RemoveAll(dir)
		return Skill{}, errors.New("StorageError")
	}
	a.skillImports[skill.ImportID] = skillImport{skill: skill, dir: dir, expires: time.Now().Add(15 * time.Minute), prior: prior}
	return skill, nil
}

func extractSkillPackage(ctx context.Context, filename, dir string) (Skill, error) {
	invalid := errors.New("InvalidSkillPackage")
	file, err := os.Open(filename)
	if err != nil {
		return Skill{}, invalid
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return Skill{}, invalid
	}
	if info.Size() > maxPackageZip {
		return Skill{}, errors.New("SkillArchiveTooLarge")
	}
	archive, err := zip.NewReader(file, info.Size())
	if err != nil {
		return Skill{}, invalid
	}
	if len(archive.File) > maxPackageEntries {
		return Skill{}, errors.New("SkillArchiveEntriesExceeded")
	}
	var root string
	found := false
	for _, entry := range archive.File {
		name := strings.TrimSuffix(entry.Name, "/")
		if !validPackagePath(name) || (!entry.Mode().IsRegular() && !entry.Mode().IsDir()) {
			return Skill{}, invalid
		}
		if path.Base(name) == "SKILL.md" && entry.Mode().IsRegular() {
			if found || strings.Count(name, "/") > 1 {
				return Skill{}, invalid
			}
			found = true
			root = strings.TrimSuffix(name, "SKILL.md")
		}
	}
	if !found {
		return Skill{}, invalid
	}
	seen := make(map[string]bool)
	spellings := make(map[string]string)
	var skill Skill
	var resources []SkillResource
	var total int64
	for _, entry := range archive.File {
		if err := ctx.Err(); err != nil {
			return Skill{}, err
		}
		name := strings.TrimSuffix(entry.Name, "/")
		if entry.Mode().IsDir() && name == strings.TrimSuffix(root, "/") {
			continue
		}
		name, ok := strings.CutPrefix(name, root)
		if !ok || !validPackagePath(name) {
			return Skill{}, invalid
		}
		key := strings.ToLower(norm.NFC.String(name))
		if seen[key] {
			return Skill{}, invalid
		}
		seen[key] = true
		for candidate := name; candidate != "."; candidate = path.Dir(candidate) {
			folded := strings.ToLower(norm.NFC.String(candidate))
			if prior, ok := spellings[folded]; ok && prior != candidate {
				return Skill{}, invalid
			}
			spellings[folded] = candidate
		}
		destination := filepath.Join(dir, filepath.FromSlash(name))
		if entry.Mode().IsDir() {
			if err := os.MkdirAll(destination, 0700); err != nil {
				return Skill{}, invalid
			}
			continue
		}
		limit := uint64(maxPackageFile)
		sizeCode := "SkillArchiveFileTooLarge"
		if name == "SKILL.md" {
			limit, sizeCode = maxSkillText, "SkillTextTooLarge"
		}
		if entry.UncompressedSize64 > limit {
			return Skill{}, errors.New(sizeCode)
		}
		if entry.UncompressedSize64 > uint64(maxPackageExpanded-total) {
			return Skill{}, errors.New("SkillArchiveExpandedTooLarge")
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
			return Skill{}, invalid
		}
		n, err := copyPackageEntry(ctx, entry, destination, int64(limit))
		if err != nil || n != int64(entry.UncompressedSize64) {
			return Skill{}, invalid
		}
		total += n
		if name == "SKILL.md" {
			content, err := os.ReadFile(destination)
			if err != nil {
				return Skill{}, invalid
			}
			skill, err = parseSkill(string(content), false)
			if err != nil {
				return Skill{}, err
			}
		} else {
			resources = append(resources, SkillResource{Path: name, Size: n})
		}
	}
	skill.Resources = resources
	return skill, nil
}

func copyPackageEntry(ctx context.Context, entry *zip.File, filename string, limit int64) (int64, error) {
	source, err := entry.Open()
	if err != nil {
		return 0, err
	}
	defer source.Close()
	target, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600|entry.Mode().Perm()&0100)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(target, io.LimitReader(skillContextReader{ctx: ctx, Reader: source}, limit+1))
	return n, errors.Join(err, target.Close())
}

type skillContextReader struct {
	ctx context.Context
	io.Reader
}

func (r skillContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.Reader.Read(p)
}

func (a *Platform) finishSkillImport(input Skill) (Skill, error) {
	pending, ok := a.skillImports[input.ImportID]
	if !ok || time.Now().After(pending.expires) {
		a.dropSkillImport(input.ImportID)
		return Skill{}, errors.New("SkillImportExpired")
	}
	value, err := parseSkill(input.Markdown, input.Enabled)
	if err != nil {
		return Skill{}, err
	}
	stamp, err := a.skillVersionStamp(value.Name)
	if err != nil {
		return Skill{}, errors.New("StorageError")
	}
	if (value.Name == pending.skill.Name && stamp != pending.prior) || (value.Name != pending.skill.Name && stamp != [32]byte{}) {
		return Skill{}, errors.New("SkillChanged")
	}
	if value.Markdown != pending.skill.Markdown {
		if err := atomicFile(pending.dir, "SKILL.md", []byte(value.Markdown)); err != nil {
			return Skill{}, errors.New("StorageError")
		}
	}
	value.Revision, value.Resources = rand.Text(), pending.skill.Resources
	value.Icon, value.Color = input.Icon, input.Color
	destination := a.skillDirectory(value)
	if err := os.Rename(pending.dir, destination); err != nil {
		return Skill{}, errors.New("StorageError")
	}
	if _, err := a.storeSkill(value); err != nil {
		_ = os.Rename(destination, pending.dir)
		return Skill{}, err
	}
	delete(a.skillImports, input.ImportID)
	return value, nil
}

func (a *Platform) skillDirectory(skill Skill) string {
	return filepath.Join(a.dir, "skill-packages", skill.Revision)
}

func (a *Platform) cancelSkillImport(id string) {
	a.skillEdit.Lock()
	defer a.skillEdit.Unlock()
	a.dropSkillImport(id)
}

func (a *Platform) dropSkillImport(id string) {
	if pending, ok := a.skillImports[id]; ok {
		_ = os.RemoveAll(pending.dir)
		delete(a.skillImports, id)
	}
}

func (a *Platform) cleanSkillPackages() {
	if a.dir == "" {
		return
	}
	root := filepath.Join(a.dir, "skill-packages")
	entries, _ := os.ReadDir(root)
	metadata, metaErr := os.ReadDir(filepath.Join(a.dir, "skills"))
	revisions := make(map[string]bool)
	valid := metaErr == nil || errors.Is(metaErr, os.ErrNotExist)
	for _, entry := range metadata {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		skill, err := readSkillMetadata(filepath.Join(a.dir, "skills", entry.Name()))
		if err != nil {
			valid = false
		}
		revisions[skill.Revision] = true
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() && (strings.HasPrefix(name, "import-") || (valid && validSkillRevision(name) && !revisions[name])) {
			_ = os.RemoveAll(filepath.Join(root, name))
		}
	}
}

func (a *Platform) skillVersionStamp(name string) ([32]byte, error) {
	filename := filepath.Join(a.dir, "skills", name+".json")
	file, err := os.Open(filename)
	if errors.Is(err, os.ErrNotExist) {
		return [32]byte{}, nil
	}
	if err != nil {
		return [32]byte{}, err
	}
	defer file.Close()
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(file, maxStoredSkill+1))
	if err != nil || n > maxStoredSkill {
		return [32]byte{}, errors.New("InvalidSkill")
	}
	return [32]byte(hash.Sum(nil)), nil
}

func (a *Platform) saveSkillVersion(value, prior Skill) (Skill, error) {
	if a.dir == "" || !validateSkillResources(prior.Resources) {
		return Skill{}, errors.New("StorageError")
	}
	packageRoot := filepath.Join(a.dir, "skill-packages")
	if err := os.MkdirAll(packageRoot, 0700); err != nil {
		return Skill{}, errors.New("StorageError")
	}
	value.Revision = rand.Text()
	destination := a.skillDirectory(value)
	if err := os.Mkdir(destination, 0700); err != nil {
		return Skill{}, errors.New("StorageError")
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(destination)
		}
	}()
	if prior.Revision != "" {
		source := a.skillDirectory(prior)
		err := filepath.WalkDir(source, func(filename string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(source, filename)
			if err != nil || relative == "." || relative == "SKILL.md" {
				return err
			}
			if entry.IsDir() {
				return os.MkdirAll(filepath.Join(destination, relative), 0700)
			}
			if !entry.Type().IsRegular() {
				return errors.New("InvalidSkill")
			}
			source, err := os.Open(filename)
			if err != nil {
				return err
			}
			defer source.Close()
			info, err := source.Stat()
			if err != nil || !info.Mode().IsRegular() || info.Size() > maxPackageFile {
				return errors.New("InvalidSkill")
			}
			target, err := os.OpenFile(filepath.Join(destination, relative), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600|info.Mode().Perm()&0100)
			if err != nil {
				return err
			}
			n, err := io.Copy(target, io.LimitReader(source, maxPackageFile+1))
			if n > maxPackageFile {
				err = errors.New("InvalidSkill")
			}
			return errors.Join(err, target.Close())
		})
		if err != nil {
			return Skill{}, errors.New("StorageError")
		}
	} else {
		// Preserve the original metadata for a rollback from a directory migration.
		if prior.Name != "" {
			old, err := json.Marshal(prior)
			if err != nil || atomicFile(packageRoot, "legacy-"+prior.Name+".json", old) != nil {
				return Skill{}, errors.New("StorageError")
			}
		}
		for _, resource := range prior.Resources {
			filename := filepath.Join(destination, filepath.FromSlash(resource.Path))
			if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil || os.WriteFile(filename, []byte(resource.Text), 0600) != nil {
				return Skill{}, errors.New("StorageError")
			}
			value.Resources = append(value.Resources, resource)
		}
	}
	if err := os.WriteFile(filepath.Join(destination, "SKILL.md"), []byte(value.Markdown), 0600); err != nil {
		return Skill{}, errors.New("StorageError")
	}
	saved, err := a.storeSkill(value)
	published = err == nil
	return saved, err
}
