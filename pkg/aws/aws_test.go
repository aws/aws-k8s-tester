package aws

import (
	"fmt"
	"testing"
)

func TestRegions(t *testing.T) {
	rg, err := Regions("aws")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range rg {
		fmt.Println(v.ID(), v.Description())
	}
}
