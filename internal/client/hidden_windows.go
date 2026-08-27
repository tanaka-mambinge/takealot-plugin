//go:build windows

package client

import (
	"fmt"
	"syscall"
	"unsafe"
)

const fileAttributeHidden = 0x2

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	setFileAttributesW = kernel32.NewProc("SetFileAttributesW")
)

func markHidden(path string) error {
	widePath, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	result, _, callErr := setFileAttributesW.Call(uintptr(unsafe.Pointer(widePath)), fileAttributeHidden)
	if result == 0 {
		return fmt.Errorf("set Windows hidden attribute: %w", callErr)
	}
	return nil
}
