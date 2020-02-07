package jobs

import (
	"fmt"
	"testing"
)

func TestJobs(t *testing.T) {
	ts := &tester{
		cfg: Config{
			JobName:   JobNameEcho,
			Completes: 1000,
			Parallels: 100,
			EchoSize:  100,
		},
	}
	_, b, err := ts.createObject()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(b))
}
