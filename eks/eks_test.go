package eks

import (
	"testing"

	"github.com/aws/aws-k8s-tester/eksconfig"
)

func TestAA(t *testing.T) {
	ts, err := New(eksconfig.NewDefault())
	if err != nil {
		panic(err)
	}
	_ = ts
}
