package i18n

import (
	"syscall"
	"unsafe"
)

func osLocale() string {
	buf := make([]uint16, 85) // LOCALE_NAME_MAX_LENGTH
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultLocaleName")
	if r, _, _ := proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))); r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}
