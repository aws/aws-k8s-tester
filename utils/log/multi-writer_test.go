package log

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-k8s-tester/utils/file"
	"github.com/aws/aws-k8s-tester/utils/spinner"
)

func TestMultiWriter(t *testing.T) {
	tmpPath := file.GetTempFilePath() + ".log"
	defer os.RemoveAll(tmpPath)

	lg, wr, logFile, err := NewWithStderrWriter("info", []string{tmpPath})
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()

	lg.Info("hi")
	fmt.Fprintf(wr, "hello %q\n", "test")
	fmt.Fprintf(wr, "hello %q\n", "test")

	go func() {
		time.Sleep(2 * time.Second)
		lg.Info("here")
	}()
	s := spinner.New(wr, "Wating...\n")
	s.Restart()
	println()
	defer s.Stop()
	time.Sleep(5 * time.Second)
	s.Restart()

	b, err := ioutil.ReadFile(tmpPath)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("output:", string(b))
}
