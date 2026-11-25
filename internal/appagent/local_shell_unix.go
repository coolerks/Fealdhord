//go:build !windows

package appagent

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func platformShells() []shellSpec {
	return []shellSpec{
		{id: automaticShell, label: "sh", paths: []string{"/bin/sh"}},
		{id: "sh", label: "sh", paths: []string{"/bin/sh", "/usr/bin/sh"}, name: "sh"},
		{id: "bash", label: "Bash", paths: []string{"/bin/bash", "/usr/bin/bash", "/opt/homebrew/bin/bash", "/usr/local/bin/bash"}, name: "bash"},
		{id: "zsh", label: "Zsh", paths: []string{"/bin/zsh", "/usr/bin/zsh", "/opt/homebrew/bin/zsh", "/usr/local/bin/zsh"}, name: "zsh"},
		{id: "fish", label: "Fish", paths: []string{"/usr/bin/fish", "/opt/homebrew/bin/fish", "/usr/local/bin/fish"}, name: "fish"},
	}
}

func localShellCommand(ctx context.Context, shell ShellOption, command string) *exec.Cmd {
	if shell.ID == "pwsh" || shell.ID == "powershell" {
		return powerShellCommand(ctx, shell.Path, command)
	}
	return dashCShellCommand(ctx, shell, command)
}

func startLocalShell(cmd *exec.Cmd) (func(), error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return func() { _ = cmd.Cancel() }, nil
}
