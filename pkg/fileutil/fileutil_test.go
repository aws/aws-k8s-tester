package fileutil

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
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

	fmt.Println(IsDirWriteable(os.TempDir()))

	defer os.RemoveAll("hello")
	if err = CopyAppend(p, "hello"); err != nil {
		t.Fatal(err)
	}
	if err = CopyAppend(p, "hello"); err != nil {
		t.Fatal(err)
	}

	d, err = ioutil.ReadFile("hello")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(d), "hello world") != 2 {
		t.Fatalf("unexpected 'hello world' count, %s", string(d))
	}
}
