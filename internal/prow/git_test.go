package prow

import (
	"fmt"
	"testing"
)

func TestFetchGit(t *testing.T) {
	t.Skip()

	k8s, err := FetchGit("https://github.com/kubernetes/kubernetes", "master")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(k8s)

	testInfra, err := FetchGit("https://github.com/kubernetes/test-infra", "master")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(testInfra)
}
