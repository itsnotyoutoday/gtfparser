// Package tflib embeds the unmodified TinyFugue library (lib/tf/*.tf) so every
// client of the engine ships the same one. The engine loads library files
// from the filesystem, so Setup writes the embedded copy to a directory
// (a content-addressed cache directory by default) and points an Interp at it.
package tflib

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/itsnotyoutoday/gtfparser"
)

//go:embed lib/*
var embedded embed.FS

// FS is the embedded library, rooted at the library directory (stdlib.tf, ...).
var FS fs.FS = func() fs.FS { s, _ := fs.Sub(embedded, "lib"); return s }()

// Version is a short hash of the embedded content (changes when the library does).
func Version() string {
	h := sha256.New()
	names, _ := fs.Glob(FS, "*")
	sort.Strings(names)
	for _, n := range names {
		b, _ := fs.ReadFile(FS, n)
		h.Write([]byte(n))
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

// Extract writes the library into dir (created if needed) and returns dir.
func Extract(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	names, err := fs.Glob(FS, "*")
	if err != nil {
		return "", err
	}
	for _, n := range names {
		b, err := fs.ReadFile(FS, n)
		if err != nil {
			return "", err
		}
		p := filepath.Join(dir, path.Base(n))
		if old, err := os.ReadFile(p); err == nil && string(old) == string(b) {
			continue
		}
		if err := os.WriteFile(p, b, 0o644); err != nil {
			return "", err
		}
	}
	return dir, nil
}

// CacheDir extracts the library under the user cache directory (or the OS
// temp directory) and returns it.
func CacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return Extract(filepath.Join(base, "gtfparser", "tflib-"+Version()))
}

// Setup makes in search the embedded library: it extracts to dir (CacheDir
// when empty), prepends it to in.LibDirs and returns the directory. The
// caller then loads stdlib.tf (in.LoadFile(filepath.Join(dir, "stdlib.tf"))).
func Setup(in *gtfparser.Interp, dir string) (string, error) {
	var err error
	if dir == "" {
		dir, err = CacheDir()
	} else {
		dir, err = Extract(dir)
	}
	if err != nil {
		return "", err
	}
	in.LibDirs = append([]string{dir}, in.LibDirs...)
	return dir, nil
}
