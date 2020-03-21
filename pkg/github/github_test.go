package github

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"go.uber.org/zap"
)

func TestQuery(t *testing.T) {
	r1, err := Query(zap.NewExample(), "latest")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", r1)

	r2, err := Query(zap.NewExample(), r1.TagName)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", r2)

	if !reflect.DeepEqual(*r1, *r2) {
		t.Fatalf("expected %+v, got %+v", r1, r2)
	}

	fpath, err := Download(zap.NewExample(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("downloaed to", fpath)
	os.RemoveAll(fpath)
}
