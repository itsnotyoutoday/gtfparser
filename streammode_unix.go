//go:build unix

package gtfparser

import (
	"os"
	"syscall"
)

// streamModeChar is the tfopen() mode letter of an open stream: r, w or a
// (append is recognized by the descriptor's O_APPEND flag).
func streamModeChar(f *os.File, writable bool) byte {
	if !writable {
		return 'r'
	}
	flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, f.Fd(), syscall.F_GETFL, 0)
	if errno == 0 && int(flags)&syscall.O_APPEND != 0 {
		return 'a'
	}
	return 'w'
}
