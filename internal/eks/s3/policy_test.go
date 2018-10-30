package s3

import (
	"fmt"
	"strings"
	"testing"
)

func Test_createAccessLogPolicy(t *testing.T) {
	p := createAccessLogPolicy("607362164682", "aws-k8s-tester-20180926")
	fmt.Println(p)
	if !strings.Contains(p, "607362164682") {
		t.Fatalf("expected '607362164682', got %q", p)
	}
}
