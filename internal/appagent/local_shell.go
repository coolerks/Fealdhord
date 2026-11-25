package appagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/coolerks/Fealdhord/internal/agentrun"
)

type localShellInput struct {
	Command   string `json:"command" jsonschema:"required"`
	Workdir   string `json:"workdir,omitempty"`
	TimeoutMS int    `json:"timeoutMs,omitempty"`
}

type localShellResult struct {
	ExitCode  int    `json:"exitCode"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Truncated bool   `json:"truncated"`
	TimedOut  bool   `json:"timedOut,omitempty"`
}

type shellBuffer struct {
	buffer    bytes.Buffer
	capacity  int
	truncated bool
}

func (b *shellBuffer) Write(p []byte) (int, error) {
	available := min(len(p), b.capacity-b.buffer.Len())
	b.truncated = b.truncated || available < len(p)
	_, _ = b.buffer.Write(p[:available])
	return len(p), nil
}

func (a *Platform) shellDirectory(ctx context.Context, run *skillRun, directory string) (string, error) {
	if strings.HasPrefix(directory, "skill://") {
		name, relative, _ := strings.Cut(strings.TrimPrefix(directory, "skill://"), "/")
		value, err := a.resolveRunSkill(run, name)
		if err != nil || !run.opened[name] || value.Revision == "" {
			return "", errors.New("SkillNotActivated")
		}
		base := a.skillDirectory(value)
		if relative == "" {
			return base, nil
		}
		if !validPackagePath(relative) {
			return "", errors.New("PathNotAllowed")
		}
		root, err := openLocalRoot(base)
		if err != nil {
			return "", err
		}
		defer root.Close()
		info, err := root.Stat(filepath.FromSlash(relative))
		if err != nil || !info.IsDir() {
			return "", errors.New("DirectoryUnavailable")
		}
		return filepath.Join(base, filepath.FromSlash(relative)), nil
	}
	workspace := agentrun.WorkspaceFrom(ctx)
	if workspace.OutputDir != "" {
		if directory == "" {
			root, err := openOutputRoot(workspace.HomeDir, workspace.OutputDir, false)
			if err != nil {
				return os.TempDir(), nil
			}
			root.Close()
			return workspace.OutputDir, nil
		}
		if !filepath.IsAbs(directory) {
			if !filepath.IsLocal(directory) {
				return "", errors.New("PathNotAllowed")
			}
			directory = filepath.Join(workspace.OutputDir, directory)
		}
		if relative, ok := withinDirectory(workspace.OutputDir, directory); ok {
			root, err := openOutputRoot(workspace.HomeDir, workspace.OutputDir, false)
			if err != nil {
				return "", err
			}
			defer root.Close()
			info, err := root.Stat(relative)
			if err != nil || !info.IsDir() {
				return "", errors.New("DirectoryUnavailable")
			}
			return filepath.Clean(directory), nil
		}
		return "", errors.New("PathNotAllowed")
	}
	if directory != "" && !filepath.IsAbs(directory) && !filepath.IsLocal(directory) {
		return "", errors.New("PathNotAllowed")
	}
	for _, base := range workspace.Roots {
		root, err := (fileTarget{root: base}).open(false)
		if err != nil {
			continue
		}
		name := directory
		if name == "" {
			name = "."
		}
		if !filepath.IsAbs(name) {
			name = filepath.Join(base, name)
		}
		relative, allowed := withinDirectory(base, name)
		var usable bool
		if allowed {
			info, err := root.Stat(relative)
			usable = err == nil && info.IsDir()
		}
		root.Close()
		if usable {
			return filepath.Clean(name), nil
		}
		if directory != "" && !filepath.IsAbs(directory) {
			break
		}
	}
	return "", errors.New("DirectoryUnavailable")
}

func shellEnvironment() []string {
	names := []string{"PATH", "LANG", "LC_ALL", "LC_CTYPE", "HOME", "TMPDIR", "TEMP", "TMP", "SystemRoot", "WINDIR", "USERPROFILE", "PATHEXT", "ComSpec"}
	env := make([]string, 0, len(names))
	for _, name := range names {
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	return env
}

func (a *Platform) runLocalShell(ctx context.Context, run *skillRun, input localShellInput, authorize localApproval) (localShellResult, error) {
	var result localShellResult
	if strings.TrimSpace(input.Command) == "" || strings.ContainsRune(input.Command, 0) || input.TimeoutMS < 0 || input.TimeoutMS > 300000 {
		return result, errors.New("ValidationError")
	}
	directory, err := a.shellDirectory(ctx, run, input.Workdir)
	if err != nil {
		return result, err
	}
	pathBefore, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return result, errors.New("DirectoryUnavailable")
	}
	info, err := os.Stat(pathBefore)
	if err != nil || !info.IsDir() {
		return result, errors.New("DirectoryUnavailable")
	}
	display, _ := json.Marshal(localShellInput{Command: input.Command, Workdir: pathBefore, TimeoutMS: input.TimeoutMS})
	if err := authorize(ctx, display); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	current, err := a.shellDirectory(ctx, run, input.Workdir)
	if err != nil {
		return result, err
	}
	pathAfter, err := filepath.EvalSymlinks(current)
	if err != nil || pathAfter != pathBefore {
		return result, errors.New("DirectoryChanged")
	}
	after, err := os.Stat(pathAfter)
	if err != nil || !os.SameFile(info, after) {
		return result, errors.New("DirectoryChanged")
	}

	timeout := 120 * time.Second
	if input.TimeoutMS > 0 {
		timeout = time.Duration(input.TimeoutMS) * time.Millisecond
	}
	callCtx, stop := context.WithTimeout(ctx, timeout)
	defer stop()
	cmd := localShellCommand(callCtx, input.Command)
	cmd.Dir, cmd.Env = pathAfter, shellEnvironment()
	// Six JSON bytes may be needed per output byte.
	capacity := (agentrun.LocalOutputBudget(ctx, localReadBytes) - 256) / 12
	stdout, stderr := shellBuffer{capacity: capacity}, shellBuffer{capacity: capacity}
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	cmd.WaitDelay = time.Second
	finish, err := startLocalShell(cmd)
	if err != nil {
		return result, errors.New("ShellStartFailed")
	}
	defer finish()
	err = cmd.Wait()
	result.Stdout = strings.ToValidUTF8(stdout.buffer.String(), "�")
	result.Stderr = strings.ToValidUTF8(stderr.buffer.String(), "�")
	result.Truncated = stdout.truncated || stderr.truncated
	result.ExitCode = cmd.ProcessState.ExitCode()
	result.TimedOut = errors.Is(callCtx.Err(), context.DeadlineExceeded)
	var exit *exec.ExitError
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if err != nil && !result.TimedOut && !errors.As(err, &exit) && !errors.Is(err, exec.ErrWaitDelay) {
		return result, errors.New("ShellOutcomeUnknown")
	}
	return result, nil
}
