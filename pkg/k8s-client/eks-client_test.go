package k8sclient

import (
	"fmt"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestParseVersion(t *testing.T) {
	body := `	{
				"major": "1",
				"minor": "16+",
				"gitVersion": "v1.16.8-eks-e16311",
				"gitCommit": "e163110a04dcb2f39c3325af96d019b4925419eb",
				"gitTreeState": "clean",
				"buildDate": "2020-03-27T22:37:12Z",
				"goVersion": "go1.13.8",
				"compiler": "gc",
				"platform": "linux/amd64"
			}`
	ver, err := parseVersion(zap.NewExample(), strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", ver)
}
