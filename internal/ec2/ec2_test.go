package ec2

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"syscall"
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

	md.cfg.UserData = base64.StdEncoding.EncodeToString([]byte(`
echo "Hello World!" > /home/ubuntu/sample.txt
`))
	if err = md.Create(); err != nil {
		t.Fatal(err)
	}

	fmt.Println(md.GenerateSSHCommands())

	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)
	fmt.Println("received:", (<-notifier).String())
	signal.Stop(notifier)

	if err = md.Delete(); err != nil {
		t.Fatal(err)
	}
}
