package desktop

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	clipboardShell32  = windows.NewLazySystemDLL("shell32.dll")
	getClipboardData  = clipboardUser32.NewProc("GetClipboardData")
	isClipboardFormat = clipboardUser32.NewProc("IsClipboardFormatAvailable")
	dragQueryFileW    = clipboardShell32.NewProc("DragQueryFileW")
)

func readClipboardFilePaths(window unsafe.Pointer) []string {
	const cfHDrop = 15
	avail, _, _ := isClipboardFormat.Call(cfHDrop)
	if avail == 0 {
		return nil
	}
	opened, _, _ := openClipboard.Call(uintptr(window))
	if opened == 0 {
		return nil
	}
	defer closeClipboard.Call()
	handle, _, _ := getClipboardData.Call(cfHDrop)
	if handle == 0 {
		return nil
	}
	count, _, _ := dragQueryFileW.Call(handle, ^uintptr(0), 0, 0)
	if count == 0 || count > 64 {
		return nil
	}
	paths := make([]string, 0, count)
	for i := uintptr(0); i < count; i++ {
		length, _, _ := dragQueryFileW.Call(handle, i, 0, 0)
		if length == 0 || length > 4096 {
			continue
		}
		buf := make([]uint16, length+1)
		n, _, _ := dragQueryFileW.Call(handle, i, uintptr(unsafe.Pointer(&buf[0])), length+1)
		if n > 0 {
			paths = append(paths, syscall.UTF16ToString(buf[:n]))
		}
	}
	return paths
}
