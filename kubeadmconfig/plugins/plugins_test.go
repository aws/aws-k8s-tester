package plugins

import (
	"fmt"
	"testing"
)

func TestCreateInstallStart(t *testing.T) {
	s1, err := CreateInstallStart("1.10.9")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(s1)
}
