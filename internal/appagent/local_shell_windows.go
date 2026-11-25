package appagent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func platformShells() []shellSpec {
	system := filepath.Join(os.Getenv("SystemRoot"), "System32")
	powershell := filepath.Join(system, "WindowsPowerShell", "v1.0", "powershell.exe")
	return []shellSpec{
		{id: automaticShell, label: "Windows PowerShell", paths: []string{powershell}},
		{id: "powershell", label: "Windows PowerShell", paths: []string{powershell}},
		{id: "pwsh", label: "PowerShell 7", paths: []string{filepath.Join(os.Getenv("ProgramFiles"), "PowerShell", "7", "pwsh.exe")}, name: "pwsh.exe"},
		{id: "cmd", label: "命令提示符 (CMD)", paths: []string{filepath.Join(system, "cmd.exe")}},
	}
}

func localShellCommand(ctx context.Context, shell ShellOption, command string) *exec.Cmd {
	switch shell.ID {
	case automaticShell, "powershell", "pwsh":
		return powerShellCommand(ctx, shell.Path, command)
	case "cmd":
		cmd := exec.CommandContext(ctx, shell.Path)
		// CMD strips the first and last quotes with /s; Go's default
		// argument quoting would change quotes inside the command.
		cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `"` + shell.Path + `" /d /s /c "` + command + `"`}
		return cmd
	}
	return dashCShellCommand(ctx, shell, command)
}

func startLocalShell(cmd *exec.Cmd) (func(), error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags = windows.CREATE_NO_WINDOW | windows.CREATE_SUSPENDED
	cmd.Cancel = func() error {
		_ = windows.TerminateJobObject(job, 1)
		return cmd.Process.Kill()
	}
	if err := cmd.Start(); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_SUSPEND_RESUME, false, uint32(cmd.Process.Pid))
	if err == nil {
		err = windows.AssignProcessToJobObject(job, process)
		if err == nil {
			status, _, _ := windows.NewLazySystemDLL("ntdll.dll").NewProc("NtResumeProcess").Call(uintptr(process))
			if status != 0 {
				err = syscall.EINVAL
			}
		}
		windows.CloseHandle(process)
	}
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		windows.CloseHandle(job)
		return nil, err
	}
	return func() { _ = windows.CloseHandle(job) }, nil
}
