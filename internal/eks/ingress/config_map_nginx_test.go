package ingress

import (
	"fmt"
	"testing"
)

func TestCreateConfigMapNginx(t *testing.T) {
	s, err := CreateConfigMapNginx(10)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(s)
}
