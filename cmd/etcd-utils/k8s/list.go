package k8s

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"

	etcdclient "github.com/aws/aws-k8s-tester/pkg/etcd-client"
	k8sobject "github.com/aws/aws-k8s-tester/pkg/k8s-object"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"go.etcd.io/etcd/clientv3"
)

var (
	listElectionPfx     string
	listElectionTimeout time.Duration
	listBatch           int64
	listInterval        time.Duration
	listPfx             string
	listCSVOutput       string
	listDoneKey         string
)

var defaultListCSVOutput string

func init() {
	defaultListCSVOutput = filepath.Join(os.TempDir(), fmt.Sprintf("etcd-utils-k8s-list-%d.csv", time.Now().UnixNano()))
}

func newListCommand() *cobra.Command {
	ac := &cobra.Command{
		Use:   "list",
		Run:   listFunc,
		Short: "List all resources",
		Long: `
etcd-utils k8s \
  --endpoints http://localhost:2379 \
  list \
  --election-prefix __etcd_utils_k8s_list \
  --election-timeout 30s \
  --batch 10 \
  --interval 5s \
  --prefix /registry/deployments \
  --csv-output /tmp/etcd-utils-k8s-list.output.csv \
  --done-key __etcd_utils_k8s_list_done

`,
	}
	ac.PersistentFlags().StringVar(&listElectionPfx, "election-prefix", "__etcd_utils_k8s_list", "Prefix to campaign for")
	ac.PersistentFlags().DurationVar(&listElectionTimeout, "election-timeout", 30*time.Second, "Campaign timeout")
	ac.PersistentFlags().Int64Var(&listBatch, "batch", 10, "etcd list call batch")
	ac.PersistentFlags().DurationVar(&listInterval, "interval", 5*time.Second, "etcd list call batch interval")
	ac.PersistentFlags().StringVar(&listPfx, "prefix", "/registry/deployments", "Prefix to list")
	ac.PersistentFlags().StringVar(&listCSVOutput, "csv-output", defaultListCSVOutput, "CSV path to output data")
	ac.PersistentFlags().StringVar(&listDoneKey, "done-key", "__etcd_utils_k8s_list_done", "Key to write once list is done")
	return ac
}

func listFunc(cmd *cobra.Command, args []string) {
	fmt.Printf("\n\n************************\nstarting 'etcd-utils k8s list'\n\n")

	if enablePrompt {
		prompt := promptui.Select{
			Label: "Ready to list resources, should we continue?",
			Items: []string{
				"No, stop it!",
				"Yes, let's run!",
			},
		}
		idx, _, err := prompt.Run()
		if err != nil {
			panic(err)
		}
		if idx != 1 {
			return
		}
	}

	e, err := etcdclient.New(etcdclient.Config{
		EtcdClientConfig: clientv3.Config{Endpoints: endpoints},
		ListBath:         listBatch,
		ListInterval:     listInterval,
	})
	if err != nil {
		panic(err)
	}
	defer func() {
		e.Close()
	}()

	ok, err := e.Campaign(listElectionPfx, listElectionTimeout)
	if err != nil {
		panic(err)
	}
	if !ok {
		fmt.Println("lost campaign")
		return
	}
	kvs, err := e.Get(5*time.Second, listDoneKey)
	if err != nil {
		fmt.Println("failed to get done key", err)
		return
	}
	if len(kvs) > 0 {
		fmt.Println("done key already written; skipping", kvs)
		return
	}

	kvs, err = e.List(listPfx, listBatch, listInterval)
	if err != nil {
		panic(err)
	}

	f, err := os.OpenFile(listCSVOutput, os.O_RDWR|os.O_TRUNC, 0777)
	if err != nil {
		f, err = os.Create(listCSVOutput)
		if err != nil {
			panic(err)
		}
	}
	defer f.Close()
	wr := csv.NewWriter(f)

	fmt.Printf("\nwriting to %q\n\n", listCSVOutput)

	for _, kv := range kvs {
		tv, err := k8sobject.ExtractTypeMeta(kv.Value)
		row := []string{string(kv.Key), tv.Kind, tv.APIVersion, fmt.Sprintf("%v", err)}
		err = wr.Write(row)
		fmt.Printf("%q", row)
		if err != nil {
			fmt.Printf(" (failed %v)", err)
		}
		println()
	}

	wr.Flush()
	fmt.Printf("\nsaved to CSV %q\n\n", listCSVOutput)

	err = e.Put(10*time.Second, listDoneKey, "done")
	if err != nil {
		panic(err)
	}

	println()
	fmt.Println("'etcd-utils k8s list' success")
}
