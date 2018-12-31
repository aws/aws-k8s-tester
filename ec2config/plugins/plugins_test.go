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
			"install-csi-101/github-account-kubernetes-sigs",
			"update-amazon-linux-2",
			"install-go-1.11.3",
			"install-wrk",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(script)

	expectedErrorForInstallCSI := func(actualNum int) string {
		return fmt.Sprintf("expected two strings (GitHub account and branch/PR) but got %v", actualNum)
	}

	// For 'install-csi', expect error when not providing both a PR/branch and a GitHub account.
	_, err = Create(
		"ubuntu",
		[]string{
			"install-csi-kubernetes-sigs",
		},
	)
	if err == nil {
		t.Fatal("expected error with 'install-csi' but did not receive one")
	}
	if err.Error() != expectedErrorForInstallCSI(1) {
		t.Fatal("got different error with 'install-csi' than expected: ", err)
	}

	// For 'install-csi', expect error when providing more than a PR/branch and a GitHub account.
	_, err = Create(
		"ubuntu",
		[]string{
			"install-csi-master/github-account-test-github-account/github-account-second-github-account",
		},
	)
	if err == nil {
		t.Fatal("expected error with 'install-csi' but did not receive one")
	}
	if err.Error() != expectedErrorForInstallCSI(3) {
		t.Fatal("got different error with 'install-csi' than expected: ", err)
	}

	// -----------------------------------------------------------------------------------------------------------------
	// For 'install-csi', get expected string when provided valid input for branch.
	testBranch := "test-branch"
	testPR := "42"
	testGitHubAccount := "test-github-account"
	getFullResults := func(result string) string {
		return headerBash + result + fmt.Sprintf("\n\necho %s\n\n", READY)
	}

	getExpectedResult := func(isPR bool) string {
		testBranchOrPR := ""
		if isPR {
			testBranchOrPR = testPR
		} else {
			testBranchOrPR = testBranch
		}

		expectedResult, err := createInstallGit(gitInfo{
			GitRepo:       "aws-ebs-csi-driver",
			GitClonePath:  fmt.Sprintf("${GOPATH}/src/github.com/%s", testGitHubAccount),
			GitCloneURL:   fmt.Sprintf("https://github.com/%s/aws-ebs-csi-driver.git", testGitHubAccount),
			IsPR:          isPR,
			GitBranch:     testBranchOrPR,
			InstallScript: `make aws-ebs-csi-driver && sudo cp ./bin/aws-ebs-csi-driver /usr/local/bin/aws-ebs-csi-driver`,
		})
		if err != nil {
			t.Fatal(err)
		}

		return getFullResults(expectedResult)
	}

	// Gets expected results when using a branch
	actualResult, err := Create(
		"ubuntu",
		[]string{
			fmt.Sprintf("install-csi-%s/github-account-%s", testBranch, testGitHubAccount),
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedResult := getExpectedResult(false)

	if actualResult != expectedResult {
		t.Fatalf("EXPECTED:\n%s\nGOT:\n%s", actualResult, expectedResult)
	}

	// Gets expected results when using a PR
	actualResult, err = Create(
		"ubuntu",
		[]string{
			fmt.Sprintf("install-csi-%s/github-account-%s", testPR, testGitHubAccount),
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedResult = getExpectedResult(true)

	if actualResult != expectedResult {
		t.Fatalf("EXPECTED:\n%s\nGOT:\n%s", actualResult, expectedResult)
	}
}
