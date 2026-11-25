package appagent

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/agentrun"
	"github.com/coolerks/Fealdhord/internal/ai"
)

func invokeLocal(t *testing.T, ctx context.Context, tools []agentrun.ReadTool, name string, input any) string {
	t.Helper()
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools {
		if tool.Definition.Name == name {
			result, err := tool.Read(ctx, ai.ToolCall{ArgumentsJSON: string(data)})
			if err != nil {
				t.Fatal(err)
			}
			return result
		}
	}
	t.Fatalf("missing tool %s", name)
	return ""
}

func TestReadLocalPageContinuesByLineAndBytes(t *testing.T) {
	linePage, err := readLocalPage(t.Context(), strings.NewReader(strings.Repeat("短行\n", 2200)), localReadInput{}, localReadBytes)
	if err != nil || linePage.NextOffset != 2001 || !strings.Contains(linePage.Text, "2000\t短行") {
		t.Fatalf("invalid line page: next=%d, err=%v", linePage.NextOffset, err)
	}
	document := strings.Repeat("界<", 30000) + "\n末行"
	first, err := readLocalPage(t.Context(), strings.NewReader(document), localReadInput{}, localReadBytes)
	data, _ := json.Marshal(first)
	if err != nil || !first.Truncated || first.NextOffset != 1 || first.NextLineByteOffset == 0 || len(data) > localReadBytes {
		t.Fatalf("long line not paginated: serialized=%d, cursor=%d/%d, err=%v", len(data), first.NextOffset, first.NextLineByteOffset, err)
	}
	next, err := readLocalPage(t.Context(), strings.NewReader(document), localReadInput{Offset: first.NextOffset, LineByteOffset: first.NextLineByteOffset}, localReadBytes)
	if err != nil || !strings.HasPrefix(next.Text, "1\t") || next.NextLineByteOffset <= first.NextLineByteOffset {
		t.Fatal("continuation did not advance", err)
	}
}

func TestLocalFileToolsKeepOutputLazyAndScoped(t *testing.T) {
	home := t.TempDir()
	output := filepath.Join(home, ".Fealdhord", "会话文件", "2026-11-25", "current")
	ctx := agentrun.WithWorkspace(agentrun.WithApprovalMode(t.Context(), agentrun.ApprovalBypass), agentrun.Workspace{HomeDir: home, OutputDir: output})
	platform := NewPlatform(NewRegistry(), t.TempDir())
	tools, _, err := platform.RunTools("current", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	invokeLocal(t, ctx, tools, "read", map[string]any{"path": "note"})
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("read created default directory", err)
	}
	if result := invokeLocal(t, ctx, tools, "write", map[string]any{"path": "nested/note", "content": "一行\n一行"}); strings.Contains(result, "error") {
		t.Fatal(result)
	}
	if result := invokeLocal(t, ctx, tools, "edit", map[string]any{"path": "nested/note", "oldText": "一行", "newText": "更新"}); !strings.Contains(result, "TextNotUnique") {
		t.Fatal(result)
	}
	if result := invokeLocal(t, ctx, tools, "edit", map[string]any{"path": "nested/note", "oldText": "一行", "newText": "更新", "replaceAll": true}); !strings.Contains(result, `"replacements":2`) {
		t.Fatal(result)
	}
	if result := invokeLocal(t, ctx, tools, "read", map[string]any{"path": "nested/note", "offset": 2, "limit": 1}); !strings.Contains(result, `2\t更新`) {
		t.Fatal(result)
	}
	if err := os.WriteFile(filepath.Join(output, "binary.txt"), []byte{0xff, 0, 0xfe}, 0600); err != nil {
		t.Fatal(err)
	}
	if result := invokeLocal(t, ctx, tools, "read", map[string]any{"path": "binary.txt"}); result != `{"status":"binary"}` {
		t.Fatal(result)
	}
	if result := invokeLocal(t, ctx, tools, "write", map[string]any{"path": "../sibling", "content": "outside"}); !strings.Contains(result, "PathNotAllowed") {
		t.Fatal(result)
	}
}

func TestManagedSkillZipPreservesBinaryAndDisclosesOnlyRequestedFiles(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "package.zip")
	file, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	large := make([]byte, 320<<10)
	rand.Read(large)
	for path, content := range map[string][]byte{
		"fixture/SKILL.md":        []byte("---\nname: fixture\ndescription: example\n---\nunique instructions"),
		"fixture/references/help": []byte("reference-private"),
		"fixture/scripts/run.sh":  []byte("echo test"),
		"fixture/assets/blob.txt": large,
	} {
		entry, err := archive.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filename); err != nil || info.Size() <= 268<<10 {
		t.Fatal("fixture below previous size limit")
	}
	dataDir := t.TempDir()
	platform := NewPlatform(NewRegistry(), dataDir)
	preview, err := platform.previewSkillImport(t.Context(), filename)
	if err != nil || len(preview.Resources) != 3 || preview.ImportID == "" {
		t.Fatalf("invalid preview %+v: %v", preview, err)
	}
	stale, err := platform.previewSkillImport(t.Context(), filename)
	if err != nil {
		t.Fatal(err)
	}
	preview.Enabled = true
	installed, err := platform.saveSkillInput(preview)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := platform.saveSkillInput(stale); err == nil || err.Error() != "SkillChanged" {
		t.Fatalf("stale preview replaced current package: %v", err)
	}
	platform.cancelSkillImport(stale.ImportID)
	binary, err := os.ReadFile(filepath.Join(platform.skillDirectory(installed), "assets", "blob.txt"))
	if err != nil || string(binary) != string(large) {
		t.Fatal("binary asset did not survive installation", err)
	}
	reopened := NewPlatform(NewRegistry(), dataDir)
	tools, prompt, err := reopened.RunTools("id", nil, false)
	if err != nil || !strings.Contains(prompt, "skill://fixture/SKILL.md") || strings.Contains(prompt, "unique instructions") || strings.Contains(prompt, "reference-private") {
		t.Fatalf("eager content %q %v", prompt, err)
	}
	read := func(path string) string {
		return invokeLocal(t, t.Context(), tools, "read", map[string]string{"path": "skill://fixture/" + path})
	}
	if result := read("references/help"); !strings.Contains(result, "SkillNotActivated") {
		t.Fatal(result)
	}
	if result := read("SKILL.md"); !strings.Contains(result, "unique instructions") || strings.Contains(result, "reference-private") {
		t.Fatal(result)
	}
	if result := read("references/help"); !strings.Contains(result, "reference-private") {
		t.Fatal(result)
	}
	if result := read("assets/blob.txt"); result != `{"status":"binary"}` {
		t.Fatal(result)
	}
}

func TestLocalShellRequiresExactApprovalEvenInBypass(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := agentrun.WithWorkspace(agentrun.WithApprovalMode(t.Context(), agentrun.ApprovalBypass), agentrun.Workspace{Roots: []string{dir}})
	platform := NewPlatform(NewRegistry(), t.TempDir())
	tools, _, err := platform.RunTools("id", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	var shell agentrun.ReadTool
	for _, tool := range tools {
		if tool.Definition.Name == "shell" {
			shell = tool
		}
	}
	command := "printf 'shell-result'"
	if runtime.GOOS == "windows" {
		command = "Write-Output shell-result"
	}
	input, _ := json.Marshal(localShellInput{Command: command, TimeoutMS: 4000})
	done := make(chan string, 1)
	go func() { result, _ := shell.Read(ctx, ai.ToolCall{ArgumentsJSON: string(input)}); done <- result }()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(3 * time.Second)
	defer timeout.Stop()
	var action PendingAction
	for action.ID == "" {
		select {
		case result := <-done:
			t.Fatalf("shell ran before approval: %s", result)
		case <-timeout.C:
			t.Fatal("no shell approval")
		case <-ticker.C:
			if requests := platform.PendingActions(); len(requests) == 1 {
				action = requests[0]
			}
		}
	}
	var approved localShellInput
	if err := json.Unmarshal([]byte(action.Arguments), &approved); err != nil || approved.Command != command || approved.Workdir != dir {
		t.Fatalf("incorrect approval %+v %v", approved, err)
	}
	if !platform.DecideAction(action.ID, true) || platform.DecideAction(action.ID, true) {
		t.Fatal("approval not single use")
	}
	if result := <-done; !strings.Contains(result, `"exitCode":0`) || !strings.Contains(result, "shell-result") {
		t.Fatal(result)
	}
}

func TestLocalShellStopsAtDeadline(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := agentrun.WithWorkspace(t.Context(), agentrun.Workspace{Roots: []string{dir}})
	platform := NewPlatform(NewRegistry(), t.TempDir())
	command := "sleep 3; printf 'completed'"
	if runtime.GOOS == "windows" {
		command = "Start-Sleep -Seconds 3; Write-Output completed"
	}
	result, err := platform.runLocalShell(ctx, &skillRun{}, localShellInput{Command: command, TimeoutMS: 100},
		func(context.Context, json.RawMessage) error { return nil })
	if err != nil || !result.TimedOut || strings.Contains(result.Stdout, "completed") {
		t.Fatalf("invalid timeout outcome: %+v %v", result, err)
	}
}
