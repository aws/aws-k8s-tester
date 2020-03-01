package eks

import (
	"fmt"
	"testing"

	"go.uber.org/zap"
)

func Test_runCommand(t *testing.T) {
	out, err := runCommand(zap.NewExample(), "ls .")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(out))
}
