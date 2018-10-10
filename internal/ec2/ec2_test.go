package ec2

import (
	"encoding/base64"
	"fmt"
	"os"
	"reflect"
	"testing"

	ec2config "github.com/aws/awstester/internal/ec2/config"
)

func TestEC2(t *testing.T) {
	if os.Getenv("RUN_AWS_UNIT_TESTS") != "1" {
		t.Skip()
	}

	cfg := ec2config.NewDefault()

	ec, err := NewDeployer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	md, ok := ec.(*embedded)
	if !ok {
		t.Fatalf("expected '*embedded', got %v", reflect.TypeOf(ec))
	}

	md.cfg.Count = 10
	md.cfg.UserData = base64.StdEncoding.EncodeToString([]byte(`
#!/usr/bin/env bash
set -e

echo "Hello World!" > /home/ubuntu/sample.txt
`))

	if err = md.Create(); err != nil {
		t.Fatal(err)
	}

	fmt.Println(md.ShowSSHCommands())

	if err = md.Delete(); err != nil {
		t.Fatal(err)
	}
}
