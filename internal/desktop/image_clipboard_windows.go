package desktop

import (
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	clipboardUser32         = windows.NewLazySystemDLL("user32.dll")
	clipboardKernel32       = windows.NewLazySystemDLL("kernel32.dll")
	registerClipboardFormat = clipboardUser32.NewProc("RegisterClipboardFormatW")
	openClipboard           = clipboardUser32.NewProc("OpenClipboard")
	closeClipboard          = clipboardUser32.NewProc("CloseClipboard")
	emptyClipboard          = clipboardUser32.NewProc("EmptyClipboard")
	setClipboardData        = clipboardUser32.NewProc("SetClipboardData")
	globalAlloc             = clipboardKernel32.NewProc("GlobalAlloc")
	globalLock              = clipboardKernel32.NewProc("GlobalLock")
	globalUnlock            = clipboardKernel32.NewProc("GlobalUnlock")
	globalFree              = clipboardKernel32.NewProc("GlobalFree")
	moveClipboardMemory     = clipboardKernel32.NewProc("RtlMoveMemory")
)

func writeClipboardPNG(window unsafe.Pointer, png []byte) bool {
	formatName, _ := windows.UTF16PtrFromString("PNG")
	format, _, _ := registerClipboardFormat.Call(uintptr(unsafe.Pointer(formatName)))
	if format == 0 {
		return false
	}
	const movableMemory = 0x0002
	memory, _, _ := globalAlloc.Call(movableMemory, uintptr(len(png)))
	if memory == 0 {
		return false
	}
	transferred := false
	defer func() {
		if !transferred {
			globalFree.Call(memory)
		}
	}()
	data, _, _ := globalLock.Call(memory)
	if data == 0 {
		return false
	}
	moveClipboardMemory.Call(data, uintptr(unsafe.Pointer(&png[0])), uintptr(len(png)))
	runtime.KeepAlive(png)
	globalUnlock.Call(memory)

	opened, _, _ := openClipboard.Call(uintptr(window))
	if opened == 0 {
		return false
	}
	defer closeClipboard.Call()
	cleared, _, _ := emptyClipboard.Call()
	if cleared == 0 {
		return false
	}
	result, _, _ := setClipboardData.Call(format, memory)
	transferred = result != 0
	return transferred
}
