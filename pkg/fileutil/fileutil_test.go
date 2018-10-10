package fileutil

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"
)

func TestWriteTempFile(t *testing.T) {
	txt := []byte("hello world")
	p, err := WriteTempFile(txt)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	if !Exist(p) {
		t.Fatalf("%q expected to exist", p)
	}

	d, err := ioutil.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(txt, d) {
		t.Fatalf("expected %q, got %q", string(txt), string(d))
	}
}
