package cronjobs

import (
	"fmt"
	"testing"

	"github.com/aws/aws-k8s-tester/eksconfig"
)

func TestJobs(t *testing.T) {
	ts := &tester{
		cfg: Config{
			EKSConfig: &eksconfig.Config{
				AddOnCronJobs: &eksconfig.AddOnCronJobs{
					Namespace: "hello",
					Schedule:  "*/10 * * * *",
					Completes: 1000,
					Parallels: 100,
					EchoSize:  10,
				},
			},
		},
	}
	_, b, err := ts.createCronJobs()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(b))
}
