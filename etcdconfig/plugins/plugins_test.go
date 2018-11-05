package plugins

import (
	"fmt"
	"testing"
)

func TestCreateInstallScript(t *testing.T) {
	s1, err := CreateInstallScript("v3.2.12")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(s1)
	s1, err = CreateInstallScript("3.2.12")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(s1)
	s2, err := CreateInstallScript("master")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(s2)
	s3, err := CreateInstallScript("9876")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(s3)
}
