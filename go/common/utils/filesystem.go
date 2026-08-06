package utils

import (
	"io/fs"
	"os"
	"path/filepath"
)

// FileSystem abstracts file operations for easier testing.
type FileSystem interface {
	// ReadFile reads a file from disk.
	ReadFile(filename string) ([]byte, error)
	// WriteFile writes a file to disk.
	WriteFile(filename string, data []byte, perm fs.FileMode) error
	// Chmod changes the mode of a file.
	Chmod(name string, mode fs.FileMode) error
	// Rename renames a file.
	Rename(oldpath, newpath string) error
	// WalkDir walks a directory tree.
	WalkDir(root string, fn fs.WalkDirFunc) error
	// Abs returns the absolute path of a file.
	Abs(path string) (string, error)
	Stat(name string) (fs.FileInfo, error)
}

// OSFileSystem is the default implementation that wraps standard library functions.
type OSFileSystem struct{}

// ReadFile reads a file from disk.
func (OSFileSystem) ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

// WriteFile writes a file to disk.
func (OSFileSystem) WriteFile(filename string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(filename, data, perm)
}

// Chmod changes the mode of a file.
func (OSFileSystem) Chmod(name string, mode fs.FileMode) error {
	return os.Chmod(name, mode)
}

// Rename renames a file.
func (OSFileSystem) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

// WalkDir walks a directory tree.
func (OSFileSystem) WalkDir(root string, fn fs.WalkDirFunc) error {
	return filepath.WalkDir(root, fn)
}

// Abs returns the absolute path of a file.
func (OSFileSystem) Abs(path string) (string, error) {
	return filepath.Abs(path)
}

// Stat returns the FileInfo structure describing the named file.
func (OSFileSystem) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}
