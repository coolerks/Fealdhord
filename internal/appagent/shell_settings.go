package appagent

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
)

const shellSettingsFile = "shell-settings.json"
const automaticShell = "auto"

type ShellOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Path  string `json:"path,omitempty"`
}

type ShellSettings struct {
	DefaultShell string        `json:"defaultShell"`
	Options      []ShellOption `json:"options"`
}

type shellPreference struct {
	DefaultShell string `json:"defaultShell"`
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

func (a *Platform) defaultShell() (string, error) {
	if a.dir == "" {
		return automaticShell, nil
	}
	filename := filepath.Join(a.dir, shellSettingsFile)
	info, err := os.Stat(filename)
	if errors.Is(err, os.ErrNotExist) {
		return automaticShell, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1024 {
		return "", errors.New("StorageError")
	}
	data, err := os.ReadFile(filename)
	var preference shellPreference
	if err != nil || len(data) > 1024 || json.Unmarshal(data, &preference) != nil || preference.DefaultShell == "" {
		return "", errors.New("StorageError")
	}
	return preference.DefaultShell, nil
}

func listShellSettings(specs []shellSpec, id string) ShellSettings {
	settings := ShellSettings{DefaultShell: id, Options: make([]ShellOption, 0, len(specs)+1)}
	for _, spec := range specs {
		settings.Options = append(settings.Options, spec.option())
	}
	if !slices.ContainsFunc(settings.Options, func(option ShellOption) bool { return option.ID == id }) {
		settings.Options = append(settings.Options, ShellOption{ID: id, Label: "已保存的 Shell"})
	}
	return settings
}

func (a *Platform) ShellSettings() (ShellSettings, error) {
	id, err := a.defaultShell()
	if err != nil {
		return ShellSettings{}, err
	}
	return listShellSettings(platformShells(), id), nil
}

func (a *Platform) SaveDefaultShell(id string) (ShellSettings, error) {
	specs := platformShells()
	index := slices.IndexFunc(specs, func(spec shellSpec) bool { return spec.id == id })
	if index < 0 {
		return ShellSettings{}, errors.New("ValidationError")
	}
	settings := listShellSettings(specs, id)
	if id != automaticShell && settings.Options[index].Path == "" {
		return ShellSettings{}, errors.New("ShellUnavailable")
	}
	if a.dir == "" || atomicJSON(a.dir, shellSettingsFile, shellPreference{DefaultShell: id}) != nil {
		return ShellSettings{}, errors.New("StorageError")
	}
	return settings, nil
}

func (a *Platform) resolveLocalShell() (ShellOption, error) {
	id, err := a.defaultShell()
	if err != nil {
		return ShellOption{}, err
	}
	specs := platformShells()
	index := slices.IndexFunc(specs, func(spec shellSpec) bool { return spec.id == id })
	if index < 0 {
		return ShellOption{}, errors.New("ShellUnavailable")
	}
	option := specs[index].option()
	if option.Path == "" {
		return ShellOption{}, errors.New("ShellUnavailable")
	}
	return option, nil
}
