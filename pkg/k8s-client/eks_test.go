package k8sclient

import (
	"fmt"
	"testing"

	"go.uber.org/zap"
)

func TestParseVersion(t *testing.T) {
	body := []byte(`	{
				"major": "1",
				"minor": "17+",
				"gitVersion": "v1.17.6-eks-db76ccf",
				"gitCommit": "db76ccfa14cf55a34024a0a573ed3f8631f40aad",
				"gitTreeState": "clean",
				"buildDate": "2020-05-21T23:51:40Z",
				"goVersion": "go1.13.9",
				"compiler": "gc",
				"platform": "linux/amd64"
			}`)
	ver, err := parseVersion(zap.NewExample(), body)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", ver)
}
