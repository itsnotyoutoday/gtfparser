//go:build !unix

package gtfparser

import "os"

func streamModeChar(f *os.File, writable bool) byte {
	if !writable {
		return 'r'
	}
	return 'w'
}
