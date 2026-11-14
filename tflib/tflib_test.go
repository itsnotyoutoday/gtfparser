package tflib

import (
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/itsnotyoutoday/gtfparser"
)

type h struct{}

func (h) Echo(string)           {}
func (h) Send(_, _ string) bool { return true }

func TestSetupLoadsStdlib(t *testing.T) {
	if _, err := fs.Stat(FS, "stdlib.tf"); err != nil {
		t.Fatal(err)
	}
	in := gtfparser.New(h{})
	dir, err := Setup(in, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := in.LoadFile(filepath.Join(dir, "stdlib.tf")); err != nil {
		t.Fatal(err)
	}
}
