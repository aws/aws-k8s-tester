package plugins

import (
	"fmt"
	"testing"
)

func Test_createInstall(t *testing.T) {
	s1, err := createInstallGo(goInfo{
		UserName:  "ubuntu",
		GoVersion: "1.11.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(s1)

	s2, err := createInstallGit(gitInfo{
		GitName:       "kubernetes-sigs",
		GitRepoName:   "aws-ebs-csi-driver",
		IsPR:          false,
		GitBranch:     "master",
		InstallScript: "go install -v ./cmd/aws-ebs-csi-driver",
	})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(s2)

	s3, err := createInstallGit(gitInfo{
		GitName:     "kubernetes-sigs",
		GitRepoName: "aws-alb-ingress-controller",
		IsPR:        true,
		GitBranch:   "123",
		InstallScript: `GO111MODULE=on go mod vendor -v
make server
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(s3)
}

func TestPlugins(t *testing.T) {
	script, err := Get(
		"ubuntu",
		[]string{
			"install-csi-101",
			"update-ubuntu",
			"install-go1.11.1",
			"install-wrk",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(script)
}
