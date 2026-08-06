package utils

import (
	iofs "io/fs"
	"testing"
)

func TestAll(t *testing.T) {
	// Really don't care about return values.
	// Just want to make sure they compile.
	fs := OSFileSystem{}
	fs.ReadFile("")
	fs.WriteFile("", []byte{}, 0)
	fs.Chmod("", 0)
	fs.Rename("", "")
	fs.Abs("")
	fs.WalkDir("", func(path string, d iofs.DirEntry, err error) error {
		return nil
	})
	fs.Stat("")
}
