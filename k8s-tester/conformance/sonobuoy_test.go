package conformance

import (
	"fmt"
	"os"
	"testing"

	"go.uber.org/zap"
)

func TestSonobuoy(t *testing.T) {
	t.Skip()

	err := installSonobuoy(zap.NewExample(), DefaultSonobuoyPath(), DefaultSonobuoyDownloadURL())
	if err != nil {
		t.Skip(err)
	}

	if err := readResults(zap.NewExample(), os.Stderr, DefaultSonobuoyPath(), "test-data/test.tar.gz"); err != nil {
		t.Fatal(err)
	}

	os.RemoveAll("test-data/output")
	defer os.RemoveAll("test-data/output")
	logPath, xmlPath, err := untarResults(zap.NewExample(), "test-data/test.tar.gz", "test-data/output")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(logPath)
	fmt.Println(xmlPath)
}
