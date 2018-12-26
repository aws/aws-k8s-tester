package plugins

import (
	"fmt"
	"testing"
)

func TestCreateInstall(t *testing.T) {
	s1, err := CreateInstall("1.13.0")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(s1)
}
