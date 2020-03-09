package jobspi

import (
	"fmt"
	"testing"

	"github.com/aws/aws-k8s-tester/eksconfig"
)

func TestJobs(t *testing.T) {
	ts := &tester{
		cfg: Config{
			EKSConfig: &eksconfig.Config{
				AddOnJobPi: &eksconfig.AddOnJobPi{Namespace: "hello"},
			},
			Completes: 1000,
			Parallels: 100,
		},
	}
	_, b, err := ts.createObject()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(b))
}
