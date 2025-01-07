package util

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

var ErrFileNotFoundInPath = errors.New("file not found in $PATH")

// LookPath finds a file on the PATH.
// It uses a similar process to exec.LookPath, but can find regular files.
func LookPath(file string) (string, error) {
	path := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			// Unix shell semantics: path element "" means "."
			dir = "."
		}
		path := filepath.Join(dir, file)
		if err := checkFile(path); err == nil {
			return path, nil
		}
	}
	return "", ErrFileNotFoundInPath
}

func checkFile(file string) error {
	d, err := os.Stat(file)
	if err != nil {
		return err
	}
	m := d.Mode()
	if m.IsDir() {
		return syscall.EISDIR
	}
	return nil
}
