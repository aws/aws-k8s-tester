package k8s

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"time"

	etcd_client "github.com/aws/aws-k8s-tester/pkg/etcd-client"
	k8s_object "github.com/aws/aws-k8s-tester/pkg/k8s-object"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"go.etcd.io/etcd/clientv3"
	"go.uber.org/zap"
	"sigs.k8s.io/yaml" // must use "sigs.k8s.io/yaml"
)

var (
	listElectionPfx     string
	listElectionTimeout time.Duration
	listDoneKey         string

	listPfxs          []string
	listBatchLimit    int64
	listBatchInterval time.Duration
	listOutput        string
)

var (
	now                    = time.Now()
	ts                     = fmt.Sprintf("%d%02d%02d", now.Year(), now.Month(), now.Hour())
	defaultListElectionPfx = fmt.Sprintf("__etcd_utils_k8s_list_election_%s", ts)
	defaultListDoneKey     = fmt.Sprintf("__etcd_utils_k8s_list_done_%s", ts)
)

var defaultListPfxs = []string{
	"/registry/daemonsets",
	"/registry/deployments",
	"/registry/replicasets",
	"/registry/networkpolicies",
	"/registry/podsecuritypolicy",
}

func newListCommand() *cobra.Command {
	ac := &cobra.Command{
		Use:   "list",
		Run:   listFunc,
		Short: "List all resources",
		Long: `
etcd-utils k8s \
  --endpoints=${ETCD_ENDPOINT} \
  --enable-prompt=false \
  list \
  --prefixes /registry/daemonsets,/registry/deployments,/registry/replicasets,/registry/networkpolicies,/registry/podsecuritypolicy \
  --output /tmp/etcd_utils_k8s_list.csv

`,
	}

	ac.PersistentFlags().StringVar(&listElectionPfx, "election-prefix", defaultListElectionPfx, "Prefix to campaign for")
	ac.PersistentFlags().DurationVar(&listElectionTimeout, "election-timeout", 30*time.Second, "Campaign timeout")
	ac.PersistentFlags().StringVar(&listDoneKey, "done-key", defaultListDoneKey, "Key to write once list is done")

	ac.PersistentFlags().StringSliceVar(&listPfxs, "prefixes", defaultListPfxs, "Prefixes to list")
	ac.PersistentFlags().Int64Var(&listBatchLimit, "batch-limit", 200, "etcd list call batch")
	ac.PersistentFlags().DurationVar(&listBatchInterval, "batch-interval", 5*time.Second, "etcd list call batch interval")
	ac.PersistentFlags().StringVar(&listOutput, "output", "", "Output path (.json or .yaml)")

	return ac
}

// ListResults defines the "etcd-utils k8s list" results.
type ListResults struct {
	Results []Result `json:"results"`
}

// Result defines the "etcd-utils k8s list" result.
type Result struct {
	Prefix     string `json:"prefix"`
	Kind       string `json:"kind"`
	APIVersion string `json:"api-version"`
	Count      int    `json:"count"`
}

type Results []Result

func (rs Results) Len() int { return len(rs) }

func (rs Results) Less(i, j int) bool {
	r1 := rs[i]
	r2 := rs[j]
	if r1.Prefix == r2.Prefix {
		if r1.Kind == r2.Kind {
			if r1.APIVersion == r2.APIVersion {
				return r1.Count < r2.Count // sort by count
			}
			return r1.APIVersion < r2.APIVersion // sort by api version
		}
		return r1.Kind < r2.Kind // sort by kind
	}
	return r1.Prefix < r2.Prefix // sort by prefix
}

func (rs Results) Swap(i, j int) {
	t := rs[i]
	rs[i] = rs[j]
	rs[j] = t
}

func listFunc(cmd *cobra.Command, args []string) {
	ext := filepath.Ext(listOutput)
	if ext != ".json" && ext != ".yaml" {
		panic(fmt.Sprintf("invalid file extension '--output=%s'", listOutput))
	}

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

	e, err := etcd_client.New(etcd_client.Config{
		Logger:           lg,
		EtcdClientConfig: clientv3.Config{LogConfig: &lcfg, Endpoints: endpoints},
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

	counts := make(map[Result]int)
	for _, pfx := range listPfxs {
		kvs, err = e.List(pfx, listBatchLimit, listBatchInterval)
		if err != nil {
			lg.Warn("failed to list", zap.Error(err))
		}
		if len(kvs) > 0 {
			for _, kv := range kvs {
				tv, err := k8s_object.ExtractTypeMeta(kv.Value)
				if err != nil {
					lg.Warn("failed to extract type metadata", zap.Error(err))
					continue
				}
				lg.Info("resource", zap.String("kind", tv.Kind), zap.String("api-version", tv.APIVersion))
				counts[Result{
					Prefix:     pfx,
					Kind:       tv.Kind,
					APIVersion: tv.APIVersion,
				}]++
			}
		} else {
			counts[Result{
				Prefix:     pfx,
				Kind:       "none",
				APIVersion: "none",
			}] = 0
		}
	}
	rs := ListResults{}
	for k, v := range counts {
		k.Count = v
		rs.Results = append(rs.Results, k)
	}
	sort.Sort(Results(rs.Results))

	lg.Info("writing", zap.String("path", listOutput))
	var data []byte
	switch ext {
	case ".json":
		data, err = json.Marshal(rs)
	case ".yaml":
		data, err = yaml.Marshal(rs)
	}
	if err != nil {
		lg.Fatal("failed to marshal", zap.Error(err))
	}
	if err := ioutil.WriteFile(listOutput, data, 0777); err != nil {
		lg.Fatal("failed to write", zap.Error(err))
	}
	lg.Info("wrote", zap.String("path", listOutput))

	err = e.Put(10*time.Second, listDoneKey, "done", time.Hour)
	if err != nil {
		panic(err)
	}
	println()
	fmt.Println("'etcd-utils k8s list' success")
	println()
}
