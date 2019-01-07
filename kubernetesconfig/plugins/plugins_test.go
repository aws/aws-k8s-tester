package plugins

import (
	"fmt"
	"testing"
)

func TestCreateInstall(t *testing.T) {
	s1 := CreateInstall()
	fmt.Println(s1)
}
