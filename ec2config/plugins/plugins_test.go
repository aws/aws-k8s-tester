package plugins

import (
	"fmt"
	"testing"
)

func Test_createInstall(t *testing.T) {
	s1, err := createInstallGo(goInfo{
		UserName:  "ubuntu",
		GoVersion: "1.11.3",
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

	// -----------------------------------------------------------------------------------------------------------------
	// For 'install-csi', get expected string when provided valid input for branch.
	testBranch := "test-branch"
	testPR := "42"
	testGitHubAccount := "test-github-account"
	getFullResults := func(result string) string {
		return headerBash + result + fmt.Sprintf("\n\necho %s\n\n", READY)
	}

	expectedInstallCSIGitHubAccount := func() string {
		expectedResult, err := createSetEnvVar(envVar{
			Name:  "CSI_GITHUB_ACCOUNT",
			Value: testGitHubAccount,
		})
		if err != nil {
			t.Fatal(err)
		}
		return expectedResult
	}()

	getExpectedInstallCSI := func(isPR bool) string {
		testBranchOrPR := ""
		if isPR {
			testBranchOrPR = testPR
		} else {
			testBranchOrPR = testBranch
		}

		expectedResult, err := createInstallGit(gitInfo{
			GitRepo:       "aws-ebs-csi-driver",
			GitClonePath:  "${GOPATH}/src/github.com/${CSI_GITHUB_ACCOUNT}",
			GitCloneURL:   "https://github.com/${CSI_GITHUB_ACCOUNT}/aws-ebs-csi-driver.git",
			IsPR:          isPR,
			GitBranch:     testBranchOrPR,
			InstallScript: `[[ "${CSI_GITHUB_ACCOUNT}" != "kubernetes-sigs" ]] && mv ../../${CSI_GITHUB_ACCOUNT}/ ../../kubernetes-sigs && cd ../../kubernetes-sigs/aws-ebs-csi-driver; make aws-ebs-csi-driver && sudo cp ./bin/aws-ebs-csi-driver /usr/local/bin/aws-ebs-csi-driver`,
		})
		if err != nil {
			t.Fatal(err)
		}

		return expectedResult
	}

	// Gets expected results when using a branch
	actualResult, err := Create(
		"ubuntu",
		[]string{
			fmt.Sprintf("install-csi-github-account-%s", testGitHubAccount),
			fmt.Sprintf("install-csi-%s", testBranch),
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedResult := getFullResults(expectedInstallCSIGitHubAccount + getExpectedInstallCSI(false))

	if actualResult != expectedResult {
		t.Fatalf("EXPECTED:\n%s\nGOT:\n%s", actualResult, expectedResult)
	}

	// Gets expected results when using a PR
	actualResult, err = Create(
		"ubuntu",
		[]string{
			fmt.Sprintf("install-csi-github-account-%s", testGitHubAccount),
			fmt.Sprintf("install-csi-%s", testPR),
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedResult = getFullResults(expectedInstallCSIGitHubAccount + getExpectedInstallCSI(true))

	if actualResult != expectedResult {
		t.Fatalf("EXPECTED:\n%s\nGOT:\n%s", actualResult, expectedResult)
	}
}
