package http

import (
	"fmt"
	"os"
	"testing"

	humanize "github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

func TestGet(t *testing.T) {
	t.Skip()

	d, err := Read(zap.NewExample(), os.Stdout, "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonEC2/current/us-west-2/index.json")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("read", humanize.Bytes(uint64(len(d))))
}
