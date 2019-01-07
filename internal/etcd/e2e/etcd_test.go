package e2e

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/aws/aws-k8s-tester/etcdconfig"
	"github.com/aws/aws-k8s-tester/internal/etcd"

	"github.com/blang/semver"
)

/*
RUN_AWS_TESTS=1 go test -v -timeout 2h -run TestETCD
*/
func TestETCD(t *testing.T) {
	if os.Getenv("RUN_AWS_TESTS") != "1" {
		t.Skip()
	}

	cfg := etcdconfig.NewDefault()
	tester, err := etcd.NewTester(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if err = tester.Create(); err != nil {
		tester.Terminate()
		t.Fatal(err)
	}

	fmt.Printf("EC2 SSH:\n%s\n\n", cfg.EC2.SSHCommands())
	fmt.Printf("EC2Bastion SSH:\n%s\n\n", cfg.EC2Bastion.SSHCommands())

	fmt.Printf("Cluster: %+v\n", tester.Cluster())
	fmt.Printf("ClusterStatus: %+v\n", tester.ClusterStatus())
	presp, err := tester.MemberList()
	fmt.Printf("MemberList before member remove: %+v (error: %v)\n", presp, err)

	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-time.After(10 * time.Second):
	case sig := <-notifier:
		fmt.Fprintf(os.Stderr, "received %s\n", sig)
	}

	id := ""
	for k := range tester.Cluster().Members {
		id = k
		break
	}
	if err = tester.MemberRemove(id); err != nil {
		t.Error(err)
	}

	notifier = make(chan os.Signal, 1)
	signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-time.After(10 * time.Second):
	case sig := <-notifier:
		fmt.Fprintf(os.Stderr, "received %s\n", sig)
	}

	presp, err = tester.MemberList()
	fmt.Printf("MemberList after member remove: %+v (error: %v)\n", presp, err)

	if len(cfg.ClusterState) != 2 {
		t.Errorf("len(cfg.ClusterState) expected 2, got %d", len(cfg.ClusterState))
	}
	if cfg.ClusterSize != 2 {
		t.Errorf("cfg.ClusterSize expected 2, got %d", cfg.ClusterSize)
	}
	if len(cfg.EC2.Instances) != 2 {
		t.Errorf("len(cfg.EC2.Instances) expected 2, got %d", len(cfg.EC2.Instances))
	}
	if cfg.EC2.ClusterSize != 2 {
		t.Errorf("cfg.EC2.ClusterSize expected 2, got %d", cfg.EC2.ClusterSize)
	}

	notifier = make(chan os.Signal, 1)
	signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-time.After(cfg.WaitBeforeDown):
	case sig := <-notifier:
		fmt.Fprintf(os.Stderr, "received %s\n", sig)
	}

	if err = tester.Put("foo", "bar"); err != nil {
		t.Errorf("failed write %v", err)
	}
	versionToUgradeTxt := "3.3.10"
	versionToUgrade, verr := semver.Make(versionToUgradeTxt)
	if verr != nil {
		t.Fatal(err)
	}
	if err = tester.MemberAdd(versionToUgradeTxt); err != nil {
		t.Errorf("failed to add member %v", err)
	}

	idsToUpgrade := []string{}
	status := tester.ClusterStatus()
	for id, st := range status.Members {
		v, perr := semver.Make(st.Version)
		if perr != nil {
			t.Error(perr)
			break
		}
		if v.LT(versionToUgrade) {
			idsToUpgrade = append(idsToUpgrade, id)
		}
	}

	fmt.Println("upgrading:", idsToUpgrade)
	for _, id := range idsToUpgrade {
		if err = tester.Restart(id, versionToUgradeTxt); err != nil {
			t.Error(err)
		}
	}

	if err = tester.Terminate(); err != nil {
		t.Fatal(err)
	}
}
