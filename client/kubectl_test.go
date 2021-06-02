package client

import (
	"fmt"
	"testing"

	"go.uber.org/zap"
)

func TestKubectl(t *testing.T) {
	t.Skip()

	err := installKubectl(zap.NewExample(), DefaultKubectlPath(), DefaultKubectlDownloadURL())
	fmt.Println(err)
}
