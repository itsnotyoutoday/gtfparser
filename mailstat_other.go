//go:build !unix

package gtfparser

import "os"

func statAtime(fi os.FileInfo) (int64, bool) { return 0, false }
