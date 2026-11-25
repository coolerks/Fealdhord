package appagent

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

const (
	shellSettingsFile = "shell-settings.json"
	maxShellSettings  = 256 << 10
	automaticShell    = "auto"
	customShell       = "custom"
)

type ShellOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Path  string `json:"path,omitempty"`
}

type ShellSettings struct {
	DefaultShell    string        `json:"defaultShell"`
	Options         []ShellOption `json:"options"`
	CustomPath      string        `json:"customPath,omitempty"`
	CustomAvailable bool          `json:"customAvailable,omitempty"`
}

type shellPreference struct {
	DefaultShell string `json:"defaultShell"`
	CustomPath   string `json:"customPath,omitempty"`
}

type shellSpec struct {
	id, label string
	paths     []string
	name      string // Search PATH after the standard install locations.
}

func (spec shellSpec) option() ShellOption {
	option := ShellOption{ID: spec.id, Label: spec.label}
	for _, path := range spec.paths {
		if filepath.IsAbs(path) {
			if executable, err := exec.LookPath(path); err == nil {
				option.Path = executable
				return option
			}
		}
	}
	if spec.name != "" {
		if executable, err := exec.LookPath(spec.name); err == nil && filepath.IsAbs(executable) {
			option.Path = executable
		}
	}
	return option
}

func (a *Platform) loadShellPreference() (shellPreference, error) {
	defaults := shellPreference{DefaultShell: automaticShell}
	if a.dir == "" {
		return defaults, nil
	}
	filename := filepath.Join(a.dir, shellSettingsFile)
	info, err := os.Stat(filename)
	if errors.Is(err, os.ErrNotExist) {
		return defaults, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxShellSettings {
		return shellPreference{}, errors.New("StorageError")
	}
	data, err := os.ReadFile(filename)
	var preference shellPreference
	if err != nil || len(data) > maxShellSettings || json.Unmarshal(data, &preference) != nil || preference.DefaultShell == "" {
		return shellPreference{}, errors.New("StorageError")
	}
	return preference, nil
}

func listShellSettings(specs []shellSpec, preference shellPreference) ShellSettings {
	_, err := resolveCustomShell(preference.CustomPath)
	settings := ShellSettings{
		DefaultShell: preference.DefaultShell, Options: make([]ShellOption, 0, len(specs)+1),
		CustomPath: preference.CustomPath, CustomAvailable: err == nil,
	}
	for _, spec := range specs {
		settings.Options = append(settings.Options, spec.option())
	}
	if preference.DefaultShell != customShell && !slices.ContainsFunc(settings.Options, func(option ShellOption) bool { return option.ID == preference.DefaultShell }) {
		settings.Options = append(settings.Options, ShellOption{ID: preference.DefaultShell, Label: "已保存的 Shell"})
	}
	return settings
}

func (a *Platform) ShellSettings() (ShellSettings, error) {
	preference, err := a.loadShellPreference()
	if err != nil {
		return ShellSettings{}, err
	}
	return listShellSettings(platformShells(), preference), nil
}

func (a *Platform) SaveDefaultShell(id string) (ShellSettings, error) {
	specs := platformShells()
	index := slices.IndexFunc(specs, func(spec shellSpec) bool { return spec.id == id })
	if index < 0 {
		return ShellSettings{}, errors.New("ValidationError")
	}
	a.shellEdit.Lock()
	defer a.shellEdit.Unlock()
	preference, err := a.loadShellPreference()
	if err != nil {
		return ShellSettings{}, err
	}
	preference.DefaultShell = id
	settings := listShellSettings(specs, preference)
	if id != automaticShell && settings.Options[index].Path == "" {
		return ShellSettings{}, errors.New("ShellUnavailable")
	}
	if err := a.saveShellPreference(preference); err != nil {
		return ShellSettings{}, err
	}
	return settings, nil
}

func (a *Platform) SaveCustomShell(path string) (ShellSettings, error) {
	path = strings.TrimSpace(path)
	if strings.ContainsAny(path, "\x00\r\n") {
		return ShellSettings{}, errors.New("InvalidShellPath")
	}
	if len(path) >= 2 && path[0] == '~' && os.IsPathSeparator(path[1]) {
		home, err := os.UserHomeDir()
		if err != nil {
			return ShellSettings{}, errors.New("InvalidShellPath")
		}
		path = filepath.Join(home, path[2:])
	}
	option, err := resolveCustomShell(filepath.Clean(path))
	if err != nil {
		return ShellSettings{}, err
	}
	preference := shellPreference{DefaultShell: customShell, CustomPath: option.Path}
	a.shellEdit.Lock()
	defer a.shellEdit.Unlock()
	if err := a.saveShellPreference(preference); err != nil {
		return ShellSettings{}, err
	}
	return listShellSettings(platformShells(), preference), nil
}

func (a *Platform) saveShellPreference(preference shellPreference) error {
	data, err := json.Marshal(preference)
	if err != nil || len(data) > maxShellSettings || a.dir == "" || atomicFile(a.dir, shellSettingsFile, data) != nil {
		return errors.New("StorageError")
	}
	return nil
}

func resolveCustomShell(path string) (ShellOption, error) {
	if !filepath.IsAbs(path) || strings.ContainsAny(path, "\x00\r\n") {
		return ShellOption{}, errors.New("InvalidShellPath")
	}
	if runtime.GOOS == "windows" {
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".exe" && ext != ".com" {
			return ShellOption{}, errors.New("InvalidShellPath")
		}
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return ShellOption{}, errors.New("ShellUnavailable")
	}
	executable, err := exec.LookPath(path)
	if err != nil {
		return ShellOption{}, errors.New("ShellUnavailable")
	}
	name := filepath.Base(executable)
	id := strings.TrimSuffix(strings.ToLower(name), ".exe")
	if id == "" || id == automaticShell {
		id = customShell
	}
	return ShellOption{ID: id, Label: strconv.Quote(name), Path: executable}, nil
}

func (a *Platform) resolveLocalShell() (ShellOption, error) {
	preference, err := a.loadShellPreference()
	if err != nil {
		return ShellOption{}, err
	}
	if preference.DefaultShell == customShell {
		option, err := resolveCustomShell(preference.CustomPath)
		if err != nil {
			return ShellOption{}, errors.New("ShellUnavailable")
		}
		return option, nil
	}
	specs := platformShells()
	index := slices.IndexFunc(specs, func(spec shellSpec) bool { return spec.id == preference.DefaultShell })
	if index < 0 {
		return ShellOption{}, errors.New("ShellUnavailable")
	}
	option := specs[index].option()
	if option.Path == "" {
		return ShellOption{}, errors.New("ShellUnavailable")
	}
	return option, nil
}
