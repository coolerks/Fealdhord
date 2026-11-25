package appagent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/coolerks/Fealdhord/internal/agentrun"
	"github.com/coolerks/Fealdhord/internal/ai"
)

func TestDefaultShellPreferencePersistsAndPinsRunInterpreter(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	platform := NewPlatform(NewRegistry(), dir)
	initial, err := platform.ShellSettings()
	if err != nil || initial.DefaultShell != automaticShell {
		t.Fatalf("initial shell: %+v %v", initial, err)
	}
	id := "bash"
	command := `values=("selected shell"); printf '%s' "${values[0]}"`
	if runtime.GOOS == "windows" {
		id = "cmd"
		command = `echo "selected shell"`
	}
	index := slices.IndexFunc(initial.Options, func(option ShellOption) bool { return option.ID == id })
	if index < 0 {
		t.Fatalf("expected %s to be a supported option", id)
	}
	selected := initial.Options[index]
	if selected.Path == "" {
		t.Skipf("%s is not installed", selected.Label)
	}
	if _, err := platform.SaveDefaultShell(id); err != nil {
		t.Fatal(err)
	}
	if _, err := platform.SaveDefaultShell("unknown"); err == nil {
		t.Fatal("accepted an unknown shell")
	}
	reopened := NewPlatform(NewRegistry(), dir)
	current, err := reopened.ShellSettings()
	if err != nil || current.DefaultShell != id {
		t.Fatalf("did not reopen saved shell: %+v %v", current, err)
	}
	tools := reopened.localChatTools("test", &skillRun{})
	index = slices.IndexFunc(tools, func(tool agentrun.ReadTool) bool { return tool.Definition.Name == "shell" })
	if index < 0 || !strings.Contains(tools[index].Definition.Description, selected.Label) {
		t.Fatalf("shell definition does not describe %s", selected.Label)
	}
	if _, err := reopened.SaveDefaultShell(automaticShell); err != nil {
		t.Fatal(err)
	}
	ctx := agentrun.WithWorkspace(t.Context(), agentrun.Workspace{Roots: []string{dir}})
	action, done := waitForShellApproval(t, reopened, ctx, tools[index], command)
	var request localShellApproval
	if err := json.Unmarshal([]byte(action.Arguments), &request); err != nil || request.Shell != selected.Path || request.Command != command {
		t.Fatalf("approval does not match selected shell: %+v %v", request, err)
	}
	if !reopened.DecideAction(action.ID, true) {
		t.Fatal("could not approve command in selected shell")
	}
	if result := <-done; !strings.Contains(result, `"exitCode":0`) || !strings.Contains(result, "selected shell") {
		t.Fatalf("selected shell did not run its command: %s", result)
	}
}

func TestUnavailableSavedShellDoesNotFallBack(t *testing.T) {
	platform := NewPlatform(NewRegistry(), t.TempDir())
	if err := atomicJSON(platform.dir, shellSettingsFile, shellPreference{DefaultShell: "other-platform"}); err != nil {
		t.Fatal(err)
	}
	settings, err := platform.ShellSettings()
	if err != nil || settings.DefaultShell != "other-platform" {
		t.Fatalf("unavailable preference was lost: %+v %v", settings, err)
	}
	tools := platform.localChatTools("test", &skillRun{})
	for _, tool := range tools {
		if tool.Definition.Name != "shell" {
			continue
		}
		result, err := tool.Read(t.Context(), ai.ToolCall{ArgumentsJSON: `{"command":"echo unexpected"}`})
		if err != nil || !strings.Contains(result, "ShellUnavailable") || len(platform.PendingActions()) != 0 {
			t.Fatalf("expected an immediate unavailable result, got %s %v", result, err)
		}
		return
	}
	t.Fatal("shell tool was absent")
}

func TestCustomShellPathPersistsAndRunsOnlyAfterApproval(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "shell with spaces")
	enteredPath := "~/shell with spaces"
	command := "printf result"
	wantOutput := "custom shell: result"
	if runtime.GOOS == "windows" {
		path = filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
		enteredPath = path
		command = `echo "custom shell result"`
		wantOutput = "custom shell result"
	} else {
		t.Setenv("HOME", dir)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf 'custom shell: '\nexec /bin/sh \"$@\"\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	platform := NewPlatform(NewRegistry(), dir)
	saved, err := platform.SaveCustomShell(enteredPath)
	if err != nil || saved.DefaultShell != customShell || saved.CustomPath != path || !saved.CustomAvailable {
		t.Fatalf("failed to save custom shell: %+v %v", saved, err)
	}
	reopened := NewPlatform(NewRegistry(), dir)
	if _, err := reopened.SaveCustomShell("relative/path"); err == nil || err.Error() != "InvalidShellPath" {
		t.Fatalf("expected relative path to be rejected, got %v", err)
	}
	if _, err := reopened.SaveCustomShell(filepath.Join(dir, "missing.exe")); err == nil || err.Error() != "ShellUnavailable" {
		t.Fatalf("expected missing file to be rejected, got %v", err)
	}
	if runtime.GOOS != "windows" {
		notExecutable := filepath.Join(dir, "not-executable")
		if err := os.WriteFile(notExecutable, []byte("#!/bin/sh\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := reopened.SaveCustomShell(notExecutable); err == nil || err.Error() != "ShellUnavailable" {
			t.Fatalf("expected nonexecutable file to be rejected, got %v", err)
		}
	}
	tools := reopened.localChatTools("test", &skillRun{})
	index := slices.IndexFunc(tools, func(tool agentrun.ReadTool) bool { return tool.Definition.Name == "shell" })
	if index < 0 || !strings.Contains(tools[index].Definition.Description, filepath.Base(path)) {
		t.Fatal("tool definition did not identify the custom shell")
	}
	preset, err := reopened.SaveDefaultShell(automaticShell)
	if err != nil || preset.DefaultShell != automaticShell || preset.CustomPath != path {
		t.Fatalf("switching to a preset lost the saved custom path: %+v %v", preset, err)
	}
	ctx := agentrun.WithWorkspace(t.Context(), agentrun.Workspace{Roots: []string{dir}})
	action, done := waitForShellApproval(t, reopened, ctx, tools[index], command)
	var request localShellApproval
	if err := json.Unmarshal([]byte(action.Arguments), &request); err != nil || request.Shell != path || request.Workdir != dir || request.Command != command {
		t.Fatalf("approval did not show the custom shell, directory and exact command: %+v %v", request, err)
	}
	if !reopened.DecideAction(action.ID, true) {
		t.Fatal("could not approve command in custom shell")
	}
	if result := <-done; !strings.Contains(result, `"exitCode":0`) || !strings.Contains(result, wantOutput) {
		t.Fatalf("custom shell did not run its command: %s", result)
	}
}
