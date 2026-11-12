//go:build (!darwin && !linux && !windows) || ((darwin || linux) && !cgo)

package desktop

import "unsafe"

func writeClipboardPNG(unsafe.Pointer, []byte) bool { return false }
