package logutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/aws/aws-k8s-tester/pkg/fileutil"
)

func TestMultiWriter(t *testing.T) {
	tmpPath := fileutil.GetTempFilePath() + ".log"
	defer os.RemoveAll(tmpPath)

	lg, wr, logFile, err := NewWithStderrWriter("info", []string{tmpPath})
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()

	lg.Info("hi")
	fmt.Fprintf(wr, "hello %q\n", "test")
	fmt.Fprintf(wr, "hello %q\n", "test")

	b, err := ioutil.ReadFile(tmpPath)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(b))
}
