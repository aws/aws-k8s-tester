package plugins

import (
	"fmt"
	"testing"
)

func TestPlugins(t *testing.T) {
	ss, err := Get(
		"update-ubuntu",
		"install-go1.11.1-ubuntu",
		"install-wrk",
		"install-csi-101",
	)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(ss)
}
