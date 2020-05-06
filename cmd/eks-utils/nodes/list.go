package nodes

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"time"

	etcd_client "github.com/aws/aws-k8s-tester/pkg/etcd-client"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	k8s_object "github.com/aws/aws-k8s-tester/pkg/k8s-object"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"go.etcd.io/etcd/clientv3"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml" // must use "sigs.k8s.io/yaml"
)

var (
	listEtcdEndpoints   []string
	listElectionPfx     string
	listElectionTimeout time.Duration
	listDoneKey         string

	listBatchLimit    int64
	listBatchInterval time.Duration
	listOutput        string
)

var (
	now                    = time.Now()
	ts                     = fmt.Sprintf("%d%02d%02d", now.Year(), now.Month(), now.Hour())
	defaultListElectionPfx = fmt.Sprintf("__eks_utils_nodes_list_election_%s", ts)
	defaultListDoneKey     = fmt.Sprintf("__eks_utils_nodes_list_done_%s", ts)
)

func newListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Run:   listFunc,
		Short: "Check nodes",
		Long: `
eks-utils nodes \
  --kubeconfig ~/.kube/config \
  list \
  --etcd-endpoints=${ETCD_ENDPOINT} \
  --output /tmp/etcd_utils_k8s_list.json

`,
	}

	cmd.PersistentFlags().StringSliceVar(&listEtcdEndpoints, "etcd-endpoints", []string{"localhost:2379"}, "etcd endpoints")
	cmd.PersistentFlags().StringVar(&listElectionPfx, "election-prefix", defaultListElectionPfx, "Prefix to campaign for")
	cmd.PersistentFlags().DurationVar(&listElectionTimeout, "election-timeout", 30*time.Second, "Campaign timeout")
	cmd.PersistentFlags().StringVar(&listDoneKey, "done-key", defaultListDoneKey, "Key to write once list is done")

	cmd.PersistentFlags().Int64Var(&listBatchLimit, "batch-limit", 30, "List batch limit (e.g. 30 items at a time)")
	cmd.PersistentFlags().DurationVar(&listBatchInterval, "batch-interval", 5*time.Second, "List interval")
	cmd.PersistentFlags().StringVar(&listOutput, "output", "", "Output path (.json or .yaml)")

	return cmd
}

// ListResults defines the "eks-utils nodes list" results.
type ListResults struct {
	Results []Result `json:"results"`
}

// Result defines the "eks-utils nodes list" result.
type Result struct {
	OSImage          string  `json:"os-image"`
	KubeletVersion   float64 `json:"kubelet-version"`
	KubeProxyVersion float64 `json:"kube-proxy-version"`
	Count            int     `json:"count"`
}

type Results []Result

func (rs Results) Len() int { return len(rs) }

func (rs Results) Less(i, j int) bool {
	r1 := rs[i]
	r2 := rs[j]
	if r1.OSImage == r2.OSImage {
		if r1.KubeletVersion == r2.KubeletVersion {
			if r1.KubeProxyVersion == r2.KubeProxyVersion {
				return r1.Count < r2.Count // sort by count
			}
			return r1.KubeProxyVersion < r2.KubeProxyVersion // sort by kube-proxy version
		}
		return r1.KubeletVersion < r2.KubeletVersion // sort by kubelet version
	}
	return r1.OSImage < r2.OSImage // sort by os image
}

func (rs Results) Swap(i, j int) {
	t := rs[i]
	rs[i] = rs[j]
	rs[j] = t
}

func listFunc(cmd *cobra.Command, args []string) {
	lcfg := logutil.GetDefaultZapLoggerConfig()
	lcfg.Level = zap.NewAtomicLevelAt(logutil.ConvertToZapLevel(logLevel))
	lg, err := lcfg.Build()
	if err != nil {
		panic(err)
	}

	if kubectlPath == "" {
		panic(errors.New("'kubectl' not found"))
	}
	ext := filepath.Ext(listOutput)
	if ext != ".json" && ext != ".yaml" {
		panic(fmt.Sprintf("invalid file extension '--output=%s'", listOutput))
	}

	lg.Info("starting 'eks-utils nodes list'")
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

	var e etcd_client.Etcd
	if len(listEtcdEndpoints) > 0 {
		e, err = etcd_client.New(etcd_client.Config{
			Logger:           lg,
			EtcdClientConfig: clientv3.Config{LogConfig: &lcfg, Endpoints: listEtcdEndpoints},
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
	}

	kcfg := &k8s_client.EKSConfig{
		Logger:            lg,
		KubeConfigPath:    kubeConfigPath,
		KubeConfigContext: kubeConfigContext,
		KubectlPath:       kubectlPath,
		EnablePrompt:      enablePrompt,
		Clients:           1,
		ClientQPS:         clientQPS,
		ClientBurst:       clientBurst,
		ClientTimeout:     30 * time.Second,
	}
	cli, err := k8s_client.NewEKS(kcfg)
	if err != nil {
		lg.Fatal("failed to create client", zap.Error(err))
	}

	var nodes []v1.Node
	nodes, err = cli.ListNodes(listBatchLimit, listBatchInterval)
	if err != nil {
		lg.Fatal("failed to list nodes", zap.Error(err))
	}

	counts := make(map[Result]int)
	for _, node := range nodes {
		nodeName := node.GetName()
		info := k8s_object.ParseNodeInfo(node.Status.NodeInfo)
		b, _ := json.Marshal(info)
		lg.Info("node", zap.String("name", nodeName), zap.String("info", string(b)))
		counts[Result{
			OSImage:          info.OSImage,
			KubeletVersion:   info.KubeletMinorVersionValue,
			KubeProxyVersion: info.KubeProxyMinorVersionValue,
		}]++
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

	if len(listEtcdEndpoints) > 0 {
		err = e.Put(10*time.Second, listDoneKey, "done", time.Hour)
		if err != nil {
			panic(err)
		}
	}
	lg.Info("'eks-utils nodes list' success")
}
