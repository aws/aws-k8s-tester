// Package fileutil implements file utilities.
package fileutil

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

// WriteTempFile writes data to a temporary file.
func WriteTempFile(d []byte) (path string, err error) {
	var f *os.File
	f, err = ioutil.TempFile(os.TempDir(), fmt.Sprintf("%X", time.Now().UTC().UnixNano()))
	if err != nil {
		return "", err
	}
	path = f.Name()
	_, err = f.Write(d)
	f.Close()
	return path, err
}

// WriteToTempDir writes data to a temporary directory.
func WriteToTempDir(p string, d []byte) (path string, err error) {
	path = filepath.Join(os.TempDir(), p)
	var f *os.File
	f, err = os.Create(path)
	if err != nil {
		return "", err
	}
	path = f.Name()
	_, err = f.Write(d)
	f.Close()
	return path, err
}

// Exist returns true if a file or directory exists.
func Exist(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}

// Copy copies a file.
func Copy(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("mkdirall: %v", err)
	}

	r, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open(%q): %v", src, err)
	}
	defer r.Close()

	w, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create(%q): %v", dst, err)
	}
	defer w.Close()

	if _, err = io.Copy(w, r); err != nil {
		return err
	}
	if err := w.Sync(); err != nil {
		return err
	}
	if _, err := w.Seek(0, 0); err != nil {
		return err
	}
	return nil
}
