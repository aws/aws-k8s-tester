package gotemplate

import (
	"bytes"
	"fmt"
	"path/filepath"
	"runtime"
	"text/template"
)

// Constants
const (
	LocalDirectoryTemplateGlob = "*.gotmpl"
)

// FromLocalDirectory concatenates all files in local directory matching ./**/*.gotmpl
func FromLocalDirectory(config interface{}) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	// Get Caller's directory
	_, file, _, _ := runtime.Caller(1)
	glob := filepath.Join(filepath.Dir(file), LocalDirectoryTemplateGlob)
	// Parse templates to buffer
	t, err := template.ParseGlob(glob)
	if err != nil {
		return nil, fmt.Errorf("while parsing template, %v", err)
	}
	if err := t.Execute(buf, config); err != nil {
		return nil, fmt.Errorf("while executing template, %v", err)
	}
	return buf, nil
}
