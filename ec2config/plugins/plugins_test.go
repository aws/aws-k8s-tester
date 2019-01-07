package plugins

import (
	"fmt"
	"testing"
)

func Test_createInstall(t *testing.T) {
	s1, err := createInstallGoAmazonLinux2(goInfo{
		UserName:  "ubuntu",
		GoVersion: "1.11.4",
	})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(s1)

	s2, err := createInstallGit(gitInfo{
		GitRepo:       "aws-ebs-csi-driver",
		GitClonePath:  "${GOPATH}/src/github.com/kubernetes-sigs",
		GitCloneURL:   "https://github.com/kubernetes-sigs/aws-ebs-csi-driver.git",
		IsPR:          false,
		GitBranch:     "master",
		InstallScript: "go install -v ./cmd/aws-ebs-csi-driver",
	})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(s2)

	s3, err := createInstallGit(gitInfo{
		GitRepo:      "aws-alb-ingress-controller",
		GitClonePath: "${GOPATH}/src/github.com/kubernetes-sigs",
		GitCloneURL:  "https://github.com/kubernetes-sigs/aws-alb-ingress-controller.git",
		IsPR:         true,
		GitBranch:    "123",
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
	script, err := Create(
		"ubuntu",
		"",
		[]string{
			"install-csi-101",
			"update-amazon-linux-2",
			"install-go-1.11.3",
			"install-wrk",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(script)
}
