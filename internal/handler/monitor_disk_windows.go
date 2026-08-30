//go:build windows

package handler

import (
	"syscall"
	"unsafe"
)

var getDiskFreeSpaceExW = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")

func diskSpace() (uint64, uint64) {
	path, _ := syscall.UTF16PtrFromString(".")
	var available, total, free uint64
	r, _, _ := getDiskFreeSpaceExW.Call(uintptr(unsafe.Pointer(path)), uintptr(unsafe.Pointer(&available)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&free)))
	if r == 0 {
		return 0, 0
	}
	return total, free
}

func diskTotalGB() float64 { total, _ := diskSpace(); return float64(total) / 1024 / 1024 / 1024 }
func diskFreeGB() float64  { _, free := diskSpace(); return float64(free) / 1024 / 1024 / 1024 }
