//go:build windows

package console

import (
	"os"
	"syscall"
)

// PrepareHeadless attaches to the parent console when running headless modes from cmd.
func PrepareHeadless() error {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	attach := kernel32.NewProc("AttachConsole")
	r1, _, _ := attach.Call(uintptr(0xFFFFFFFF)) // ATTACH_PARENT_PROCESS
	if r1 == 0 {
		return nil // no parent console
	}
	out, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err == nil {
		os.Stdout = out
		os.Stderr = out
	}
	in, err := os.OpenFile("CONIN$", os.O_RDONLY, 0)
	if err == nil {
		os.Stdin = in
	}
	return nil
}
