//go:build !darwin && !windows

package desktop

import "unsafe"

func readClipboardFilePaths(_ unsafe.Pointer) []string { return nil }
