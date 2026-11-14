//go:build unix

package gtfparser

import (
	"os"
	"reflect"
	"syscall"
)

func statAtime(fi os.FileInfo) (int64, bool) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	v := reflect.ValueOf(*st)
	for _, n := range []string{"Atim", "Atimespec"} {
		if f := v.FieldByName(n); f.IsValid() {
			if s := f.FieldByName("Sec"); s.IsValid() {
				return s.Int(), true
			}
		}
	}
	return 0, false
}
