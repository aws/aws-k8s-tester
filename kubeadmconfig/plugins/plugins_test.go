package plugins

import (
	"fmt"
	"testing"
)

func TestCreateInstallStart(t *testing.T) {
	s1, err := CreateInstall("1.13.4")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(s1)
}
