package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-k8s-tester/etcdconfig"
	"github.com/aws/aws-k8s-tester/etcdtester"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"

	"github.com/spf13/cobra"
	"go.etcd.io/etcd/clientv3"
)

func newTest() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Run etcd tests",
	}
	cmd.AddCommand(
		newTestStatus(),
	)
	return cmd
}

func newTestStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Run etcd status tests from a bastion EC2",
		Run:   testStatusFunc,
	}
}

func testStatusFunc(cmd *cobra.Command, args []string) {
	if !fileutil.Exist(path) {
		fmt.Fprintf(os.Stderr, "cannot find configuration %q\n", path)
		os.Exit(1)
	}

	cfg, err := etcdconfig.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration %q (%v)\n", path, err)
		os.Exit(1)
	}

	c := etcdtester.Cluster{
		Members: make(map[string]etcdtester.Member),
	}
	for id, v := range cfg.ClusterState {
		ep := v.AdvertiseClientURLs
		mm := etcdtester.Member{
			ID:        id,
			ClientURL: ep,
			Status:    "",
			OK:        false,
		}
		cli, err := clientv3.New(clientv3.Config{
			Endpoints: []string{ep},
		})
		if err != nil {
			mm.Status = fmt.Sprintf("status check for %q failed %v", ep, err)
			mm.OK = false
		} else {
			defer cli.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			sresp, serr := cli.Status(ctx, ep)
			cancel()
			if serr != nil {
				mm.Status = fmt.Sprintf("status check for %q failed %v", ep, serr)
				mm.OK = false
			} else {
				mm.Status = fmt.Sprintf("status check for %q: %+v", ep, sresp)
				mm.OK = true
			}
		}
		c.Members[id] = mm
	}
	d, err := json.Marshal(c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal %+v (%v)\n", c, err)
		os.Exit(1)
	}
	fmt.Println(string(d))
}
