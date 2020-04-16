package k8s

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	etcdclient "github.com/aws/aws-k8s-tester/pkg/etcd-client"
	k8sobject "github.com/aws/aws-k8s-tester/pkg/k8s-object"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"go.etcd.io/etcd/clientv3"
	"go.uber.org/zap"
)

var (
	listElectionPfx     string
	listElectionTimeout time.Duration
	listBatch           int64
	listInterval        time.Duration
	listPfx             string

	listCSVIDs    []string
	listCSVOutput string

	listCSVAggregatedIDs    []string
	listCSVAggregatedOutput string

	listDoneKey string
)

var (
	defaultListCSVOutput           string
	defaultListCSVAggregatedOutput string
)

func init() {
	defaultListCSVOutput = filepath.Join(os.TempDir(), fmt.Sprintf("etcd-utils-k8s-list-%d.csv", time.Now().UnixNano()))
	defaultListCSVAggregatedOutput = filepath.Join(os.TempDir(), fmt.Sprintf("etcd-utils-k8s-list-aggregated-%d.csv", time.Now().UnixNano()))
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
  --csv-ids id1,id2 \
  --csv-output /tmp/etcd-utils-k8s-list.output.csv \
  --csv-aggregated-ids id1,id2 \
  --csv-aggregated-output /tmp/etcd-utils-k8s-list.output.aggregated.csv \
  --done-key __etcd_utils_k8s_list_done

`,
	}
	ac.PersistentFlags().StringVar(&listElectionPfx, "election-prefix", "__etcd_utils_k8s_list", "Prefix to campaign for")
	ac.PersistentFlags().DurationVar(&listElectionTimeout, "election-timeout", 30*time.Second, "Campaign timeout")
	ac.PersistentFlags().Int64Var(&listBatch, "batch", 10, "etcd list call batch")
	ac.PersistentFlags().DurationVar(&listInterval, "interval", 5*time.Second, "etcd list call batch interval")
	ac.PersistentFlags().StringVar(&listPfx, "prefix", "/registry/deployments", "Prefix to list")
	ac.PersistentFlags().StringSliceVar(&listCSVIDs, "csv-ids", []string{}, "IDs to prepend in each CSV entry")
	ac.PersistentFlags().StringVar(&listCSVOutput, "csv-output", defaultListCSVOutput, "CSV path to output data")
	ac.PersistentFlags().StringSliceVar(&listCSVAggregatedIDs, "csv-aggregated-ids", []string{}, "IDs to prepend in each aggregated  CSV entry")
	ac.PersistentFlags().StringVar(&listCSVAggregatedOutput, "csv-aggregated-output", defaultListCSVAggregatedOutput, "CSV path to output aggregated data by prefix")
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

	lcfg := logutil.GetDefaultZapLoggerConfig()
	lg, err := lcfg.Build()
	if err != nil {
		panic(err)
	}

	e, err := etcdclient.New(etcdclient.Config{
		Logger:           lg,
		EtcdClientConfig: clientv3.Config{LogConfig: &lcfg, Endpoints: endpoints},
		ListBath:         listBatch,
		ListInterval:     listInterval,
	})
	if err != nil {
		lg.Fatal("failed to create etcd instance")
	}
	defer func() {
		e.Close()
	}()

	ok, err := e.Campaign(listElectionPfx, listElectionTimeout)
	if err != nil {
		lg.Fatal("failed to campaign")
	}
	if !ok {
		lg.Warn("lost campaign; exiting")
		return
	}
	kvs, err := e.Get(5*time.Second, listDoneKey)
	if err != nil {
		lg.Warn("failed to get", zap.Error(err))
		return
	}
	if len(kvs) > 0 {
		lg.Info("done key already written; skipping", zap.String("key", fmt.Sprintf("%v", kvs)))
		return
	}

	kvs, err = e.List(listPfx, listBatch, listInterval)
	if err != nil {
		lg.Warn("failed to list", zap.Error(err))
	}

	f1, err := os.OpenFile(listCSVOutput, os.O_RDWR|os.O_TRUNC, 0777)
	if err != nil {
		f1, err = os.Create(listCSVOutput)
		if err != nil {
			lg.Warn("failed to create file", zap.Error(err))
		}
	}
	defer f1.Close()
	wr1 := csv.NewWriter(f1)

	lg.Info("writing to CSV", zap.Strings("ids", listCSVIDs), zap.String("path", listCSVOutput))
	kindToVers := make(map[string]map[string]int)
	for _, kv := range kvs {
		tv, err := k8sobject.ExtractTypeMeta(kv.Value)
		errMsg := fmt.Sprintf("%v", err)
		row := []string{string(kv.Key), tv.Kind, tv.APIVersion, errMsg}
		err = wr1.Write(append(listCSVIDs, row...))
		if err != nil {
			lg.Warn("failed to write to CSV", zap.Error(err))
		}
		if vv, ok := kindToVers[tv.Kind]; !ok {
			vv = make(map[string]int)
			vv[tv.APIVersion] = 1
			kindToVers[tv.Kind] = vv
		} else {
			v, ok := vv[tv.APIVersion]
			if ok {
				vv[tv.APIVersion] = v + 1
			} else {
				vv[tv.APIVersion] = 1
			}
			kindToVers[tv.Kind] = vv
		}
	}
	wr1.Flush()
	lg.Info("saved to CSV", zap.Strings("ids", listCSVIDs), zap.String("path", listCSVOutput))

	f2, err := os.OpenFile(listCSVAggregatedOutput, os.O_RDWR|os.O_TRUNC, 0777)
	if err != nil {
		f2, err = os.Create(listCSVAggregatedOutput)
		if err != nil {
			lg.Warn("failed to create file", zap.Error(err))
		}
	}
	defer f2.Close()
	wr2 := csv.NewWriter(f2)
	aggRows := make([][]string, 0)
	for k, v := range kindToVers {
		for ver, cnt := range v {
			aggRows = append(aggRows, []string{k, ver, fmt.Sprintf("%d", cnt)})
		}
	}
	sort.Sort(rows(aggRows))
	for _, row := range aggRows {
		err = wr2.Write(row)
		if err != nil {
			lg.Warn("failed to write to CSV", zap.Error(err))
		}
	}
	wr2.Flush()
	lg.Info("saved to CSV", zap.Strings("ids", listCSVIDs), zap.String("path", listCSVAggregatedOutput))

	err = e.Put(10*time.Second, listDoneKey, "done")
	if err != nil {
		panic(err)
	}

	println()
	fmt.Println("'etcd-utils k8s list' success")
	println()
}

type rows [][]string

func (rs rows) Len() int { return len(rs) }

func (rs rows) Less(i, j int) bool {
	r1 := rs[i]
	r2 := rs[j]
	return r1[1] < r2[1] // sort by api version
}

func (rs rows) Swap(i, j int) {
	t := rs[i]
	rs[i] = rs[j]
	rs[j] = t
}
