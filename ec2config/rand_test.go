package ec2config

import (
	"fmt"
	"testing"
)

func TestRand(t *testing.T) {
	fmt.Println(randString(12))
}
