package fileutil

import (
	"fmt"
	"io/ioutil"
	"os"
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

// Exist returns true if a file or directory exists.
func Exist(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}
