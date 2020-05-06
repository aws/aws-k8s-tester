package randutil

import (
	"encoding/hex"
	"fmt"
	"testing"
)

func TestRand(t *testing.T) {
	fmt.Println(String(12))
	s := []byte("e1e2d4c72944d601ba3fe1d4413a1abb5124212c80e45b0b3708b9f81017f35b")
	encoded := hex.EncodeToString(s)
	b, err := hex.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(encoded)
	fmt.Println(string(b))

	fmt.Println(Hex(32))
	fmt.Println(hex.EncodeToString(Bytes(32)))
}
