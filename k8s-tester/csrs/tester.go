// Package csrs a simple pi Pod with Job.
// Replace https://github.com/aws/aws-k8s-tester/tree/v1.5.9/eks/csrs.
package csrs

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"path"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-k8s-tester/client"
	k8s_tester "github.com/aws/aws-k8s-tester/k8s-tester/tester"
	"github.com/aws/aws-k8s-tester/utils/latency"
	"github.com/aws/aws-k8s-tester/utils/rand"
	"github.com/manifoldco/promptui"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	certificates_v1beta1 "k8s.io/api/certificates/v1beta1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	writeRequestsSuccessTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "csrs",
			Subsystem: "client",
			Name:      "write_requests_success_total",
			Help:      "Total number of successful write requests.",
		})
	writeRequestsFailureTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "csrs",
			Subsystem: "client",
			Name:      "write_requests_failure_total",
			Help:      "Total number of successful write requests.",
		})
	writeRequestLatencyMs = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "csrs",
			Subsystem: "client",
			Name:      "write_request_latency_milliseconds",
			Help:      "Bucketed histogram of client-side write request and response latency.",

			// lowest bucket start of upper bound 0.5 ms with factor 2
			// highest bucket start of 0.5 ms * 2^13 == 4.096 sec
			Buckets: prometheus.ExponentialBuckets(0.5, 2, 14),
		})
)

func init() {
	prometheus.MustRegister(writeRequestsSuccessTotal)
	prometheus.MustRegister(writeRequestsFailureTotal)
	prometheus.MustRegister(writeRequestLatencyMs)
}

type Config struct {
	Enable bool `json:"enable"`
	Prompt bool `json:"-"`

	Stopc     chan struct{} `json:"-"`
	Logger    *zap.Logger   `json:"-"`
	LogWriter io.Writer     `json:"-"`
	Client    client.Client `json:"-"`

	// MinimumNodes is the minimum number of Kubernetes nodes required for installing this addon.
	MinimumNodes int `json:"minimum_nodes"`

	// Objects is the desired number of objects to create.
	Objects int `json:"objects"`
	// InitialRequestConditionType is the initial CSR condition type
	// to simulate CSR condition.
	//
	// Valid values are:
	//   "k8s.io/api/certificates/v1beta1.CertificateApproved" == "Approved"
	//   "k8s.io/api/certificates/v1beta1.CertificateDenied" == "Denied"
	//   "Random"
	//   "Pending"
	//   ""
	//
	InitialRequestConditionType string `json:"initial_request_condition_type"`

	LatencySummary latency.Summary `json:"latency_summary" read-only:"true"`
}

func (cfg *Config) ValidateAndSetDefaults() error {
	if cfg.Objects == 0 {
		cfg.Objects = DefaultObjects
	}

	switch cfg.InitialRequestConditionType {
	case "Approved":
	case "Denied":
	case "Pending", "":
	case "Random":
	default:
		return fmt.Errorf("unknown InitialRequestConditionType %q", cfg.InitialRequestConditionType)
	}

	return nil
}

const (
	DefaultMinimumNodes                int    = 1
	DefaultObjects                     int    = 10 // 1000 objects generates 5 MB data to etcd
	DefaultInitialRequestConditionType string = "Pending"
)

func NewDefault() *Config {
	return &Config{
		Enable:                      false,
		Prompt:                      false,
		MinimumNodes:                DefaultMinimumNodes,
		Objects:                     DefaultObjects,
		InitialRequestConditionType: DefaultInitialRequestConditionType,
	}
}

func New(cfg *Config) k8s_tester.Tester {
	return &tester{
		cfg:            cfg,
		donec:          make(chan struct{}),
		donecCloseOnce: new(sync.Once),
	}
}

type tester struct {
	cfg            *Config
	donec          chan struct{}
	donecCloseOnce *sync.Once
}

var pkgName = path.Base(reflect.TypeOf(tester{}).PkgPath())

func Env() string {
	return "ADD_ON_" + strings.ToUpper(strings.Replace(pkgName, "-", "_", -1))
}

func (ts *tester) Name() string { return pkgName }

func (ts *tester) Enabled() bool { return ts.cfg.Enable }

func (ts *tester) Apply() error {
	if ok := ts.runPrompt("apply"); !ok {
		return errors.New("cancelled")
	}

	if ts.cfg.MinimumNodes > 0 {
		if nodes, err := client.ListNodes(ts.cfg.Client.KubernetesClient()); len(nodes) < ts.cfg.MinimumNodes || err != nil {
			return fmt.Errorf("failed to validate minimum nodes requirement %d (nodes %v, error %v)", ts.cfg.MinimumNodes, len(nodes), err)
		}
	}

	latencies := ts.startWrites()
	if len(latencies) == 0 {
		ts.cfg.Logger.Warn("no latency collected")
		return nil
	}

	ts.cfg.Logger.Info("sorting write latency results", zap.Int("total-data-points", latencies.Len()))
	now := time.Now()
	sort.Sort(latencies)
	ts.cfg.Logger.Info("sorted write latency results", zap.Int("total-data-points", latencies.Len()), zap.String("took", time.Since(now).String()))
	ts.cfg.LatencySummary.TestID = time.Now().UTC().Format(time.RFC3339Nano)
	ts.cfg.LatencySummary.P50 = latencies.PickP50()
	ts.cfg.LatencySummary.P90 = latencies.PickP90()
	ts.cfg.LatencySummary.P99 = latencies.PickP99()
	ts.cfg.LatencySummary.P999 = latencies.PickP999()
	ts.cfg.LatencySummary.P9999 = latencies.PickP9999()

	// https://pkg.go.dev/github.com/prometheus/client_golang/prometheus?tab=doc#Gatherer
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		ts.cfg.Logger.Warn("failed to gather prometheus metrics", zap.Error(err))
		return err
	}
	for _, mf := range mfs {
		if mf == nil {
			continue
		}
		switch *mf.Name {
		case "csrs_client_write_requests_success_total":
			gg := mf.Metric[0].GetGauge()
			ts.cfg.LatencySummary.SuccessTotal = gg.GetValue()
		case "csrs_client_write_requests_failure_total":
			gg := mf.Metric[0].GetGauge()
			ts.cfg.LatencySummary.FailureTotal = gg.GetValue()
		case "csrs_client_write_request_latency_milliseconds":
			ts.cfg.LatencySummary.Histogram, err = latency.ParseHistogram("milliseconds", mf.Metric[0].GetHistogram())
			if err != nil {
				return err
			}
		}
	}
	fmt.Fprintf(ts.cfg.LogWriter, "\n\nLatencySummary:\n%s\n", ts.cfg.LatencySummary.Table())

	return nil
}

func (ts *tester) Delete() error {
	if ok := ts.runPrompt("delete"); !ok {
		return errors.New("cancelled")
	}

	ts.donecCloseOnce.Do(func() {
		close(ts.donec)
	})

	var errs []string

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	return nil
}

func (ts *tester) runPrompt(action string) (ok bool) {
	if ts.cfg.Prompt {
		msg := fmt.Sprintf("Ready to %q resources, should we continue?", action)
		prompt := promptui.Select{
			Label: msg,
			Items: []string{
				"No, cancel it!",
				fmt.Sprintf("Yes, let's %q!", action),
			},
		}
		idx, answer, err := prompt.Run()
		if err != nil {
			panic(err)
		}
		if idx != 1 {
			fmt.Printf("cancelled %q [index %d, answer %q]\n", action, idx, answer)
			return false
		}
	}
	return true
}

func (ts *tester) startWrites() (latencies latency.Durations) {
	ts.cfg.Logger.Info("writing", zap.Int("objects", ts.cfg.Objects), zap.Int("object-size", ts.cfg.Objects))
	latencies = make(latency.Durations, 0, 20000)

	for i := 0; i < ts.cfg.Objects; i++ {
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("writes stopped")
			return
		case <-ts.donec:
			ts.cfg.Logger.Info("writes done")
			return
		default:
		}

		key := fmt.Sprintf("csr%d%s", i, rand.String(7))
		cd := createCond(i, "test via "+key, ts.cfg.InitialRequestConditionType)

		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), ts.cfg.Client.Config().ClientTimeout)
		_, err := ts.cfg.Client.KubernetesClient().
			CertificatesV1beta1().
			CertificateSigningRequests().
			Create(ctx, &certificates_v1beta1.CertificateSigningRequest{
				TypeMeta: meta_v1.TypeMeta{
					APIVersion: "certificates.k8s.io/v1beta1",
					Kind:       "CertificateSigningRequest",
				},
				ObjectMeta: meta_v1.ObjectMeta{
					Name:              key,
					GenerateName:      key,
					CreationTimestamp: meta_v1.Time{Time: time.Now().Add(-20 * time.Minute)},
				},
				Spec: certificates_v1beta1.CertificateSigningRequestSpec{
					Groups:  []string{"system:bootstrappers", "system:nodes", "system:authenticated"},
					Request: reqData,
					UID:     "heptio-authenticator-aws:280347406217:AROAUCRQB56EUYTYXXJKV",
					Usages: []certificates_v1beta1.KeyUsage{
						certificates_v1beta1.UsageDigitalSignature,
						certificates_v1beta1.UsageKeyEncipherment,
						certificates_v1beta1.UsageServerAuth,
					},
					Username: "system:node:ip-172-20-32-89.us-west-2.compute.internal",
				},
				Status: certificates_v1beta1.CertificateSigningRequestStatus{
					Certificate: nil,
					Conditions:  cd,
				},
			}, meta_v1.CreateOptions{})
		cancel()
		took := time.Since(start)
		tookMS := float64(took / time.Millisecond)
		writeRequestLatencyMs.Observe(tookMS)
		latencies = append(latencies, took)
		if err != nil {
			writeRequestsFailureTotal.Inc()
			ts.cfg.Logger.Warn("write csr failed", zap.Error(err))
		} else {
			writeRequestsSuccessTotal.Inc()
			if i%20 == 0 {
				ts.cfg.Logger.Info("wrote csr", zap.Int("iteration", i))
			}
		}
	}
	return latencies
}

var conds = []certificates_v1beta1.RequestConditionType{
	certificates_v1beta1.CertificateApproved,
	certificates_v1beta1.CertificateDenied,
	certificates_v1beta1.RequestConditionType(""),
}

func createCond(idx int, msg string, tp string) (cs []certificates_v1beta1.CertificateSigningRequestCondition) {
	cs = []certificates_v1beta1.CertificateSigningRequestCondition{
		{
			Reason:         "Test",
			Message:        msg,
			LastUpdateTime: meta_v1.NewTime(time.Now().Add(-time.Hour)),
		},
	}
	switch tp {
	case string(certificates_v1beta1.CertificateApproved):
		cs[0].Type = certificates_v1beta1.CertificateApproved
	case string(certificates_v1beta1.CertificateDenied):
		cs[0].Type = certificates_v1beta1.CertificateDenied
	case "Pending", "":
		cs = make([]certificates_v1beta1.CertificateSigningRequestCondition, 0)
	case "Random":
		cs[0].Type = conds[idx%3]
	}
	return cs
}

var reqData, _ = base64.StdEncoding.DecodeString("LS0tLS1CRUdJTiBDRVJUSUZJQ0FURSBSRVFVRVNULS0tLS0KTUlJQnJEQ0NBVk1DQVFBd1dERVZNQk1HQTFVRUNoTU1jM2x6ZEdWdE9tNXZaR1Z6TVQ4d1BRWURWUVFERXpaegplWE4wWlcwNmJtOWtaVHBwY0MweE56SXRNakF0TXpJdE9Ea3VkWE10ZDJWemRDMHlMbU52YlhCMWRHVXVhVzUwClpYSnVZV3d3V1RBVEJnY3Foa2pPUFFJQkJnZ3Foa2pPUFFNQkJ3TkNBQVJGSzI3L2w4U2NtMXF1K2xXbEs5WFoKUUtVM0grSnFENTZuSEFYOXBUQ25YVWRQaUppemRzc01QaSs2emtCU1I2MXVJcVRsdnNIcjkwbFNyU2tQeDd1aQpvSUdZTUlHVkJna3Foa2lHOXcwQkNRNHhnWWN3Z1lRd2dZRUdBMVVkRVFSNk1IaUNNbVZqTWkwMU5DMHhPRFV0Ck1qUTJMVEV5T0M1MWN5MTNaWE4wTFRJdVkyOXRjSFYwWlM1aGJXRjZiMjVoZDNNdVkyOXRod1NzRkNCWmh3UTIKdWZhQWhqWnplWE4wWlcwNmJtOWtaVHBwY0MweE56SXRNakF0TXpJdE9Ea3VkWE10ZDJWemRDMHlMbU52YlhCMQpkR1V1YVc1MFpYSnVZV3d3Q2dZSUtvWkl6ajBFQXdJRFJ3QXdSQUlnVTUrNEFkWVcvRm9kdDExMmgvRjV4RHFQClFJS1BJemk4TUJMSTBBaVE2cGtDSUdqOHZPNDlTQldJVlo2SnhJL1lENldrRVhXdlZEbFp4cjFlZmVMM0NIeEgKLS0tLS1FTkQgQ0VSVElGSUNBVEUgUkVRVUVTVC0tLS0tCg==")

/*
https://kubernetes.io/docs/tasks/tls/managing-tls-in-a-cluster/

$ cat <<EOF | cfssl genkey - | cfssljson -bare server
{
  "hosts": [
    "ec2-54-185-246-128.us-west-2.compute.amazonaws.com",
    "system:node:ip-172-20-32-89.us-west-2.compute.internal",
    "172.20.32.89",
    "54.185.246.128"
  ],
  "names": [
    {
      "O": "system:nodes"
    }
  ],
  "CN": "system:node:ip-172-20-32-89.us-west-2.compute.internal",
  "key": {
    "algo": "ecdsa",
    "size": 256
  }
}
EOF


$ openssl req -text -noout -in server.csr

Certificate Request:
    Data:
        Version: 1 (0x0)
        Subject: O = system:nodes, CN = system:node:ip-172-20-32-89.us-west-2.compute.internal
        Subject Public Key Info:
            Public Key Algorithm: id-ecPublicKey
                Public-Key: (256 bit)
                pub:
                    04:45:2b:6e:ff:97:c4:9c:9b:5a:ae:fa:55:a5:2b:
                    d5:d9:40:a5:37:1f:e2:6a:0f:9e:a7:1c:05:fd:a5:
                    30:a7:5d:47:4f:88:98:b3:76:cb:0c:3e:2f:ba:ce:
                    40:52:47:ad:6e:22:a4:e5:be:c1:eb:f7:49:52:ad:
                    29:0f:c7:bb:a2
                ASN1 OID: prime256v1
                NIST CURVE: P-256
        Attributes:
        Requested Extensions:
            X509v3 Subject Alternative Name:
                DNS:ec2-54-185-246-128.us-west-2.compute.amazonaws.com, IP Address:172.20.32.89, IP Address:54.185.246.128, URI:system:node:ip-172-20-32-89.us-west-2.compute.internal
    Signature Algorithm: ecdsa-with-SHA256
         30:44:02:20:53:9f:b8:01:d6:16:fc:5a:1d:b7:5d:76:87:f1:
         79:c4:3a:8f:40:82:8f:23:38:bc:30:12:c8:d0:08:90:ea:99:
         02:20:68:fc:bc:ee:3d:48:15:88:55:9e:89:c4:8f:d8:0f:a5:
         a4:11:75:af:54:39:59:c6:bd:5e:7d:e2:f7:08:7c:47

$ cat server.csr | base64 --wrap=0

$ echo "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURSBSRVFVRVNULS0tLS0KTUlJQnJEQ0NBVk1DQVFBd1dERVZNQk1HQTFVRUNoTU1jM2x6ZEdWdE9tNXZaR1Z6TVQ4d1BRWURWUVFERXpaegplWE4wWlcwNmJtOWtaVHBwY0MweE56SXRNakF0TXpJdE9Ea3VkWE10ZDJWemRDMHlMbU52YlhCMWRHVXVhVzUwClpYSnVZV3d3V1RBVEJnY3Foa2pPUFFJQkJnZ3Foa2pPUFFNQkJ3TkNBQVJGSzI3L2w4U2NtMXF1K2xXbEs5WFoKUUtVM0grSnFENTZuSEFYOXBUQ25YVWRQaUppemRzc01QaSs2emtCU1I2MXVJcVRsdnNIcjkwbFNyU2tQeDd1aQpvSUdZTUlHVkJna3Foa2lHOXcwQkNRNHhnWWN3Z1lRd2dZRUdBMVVkRVFSNk1IaUNNbVZqTWkwMU5DMHhPRFV0Ck1qUTJMVEV5T0M1MWN5MTNaWE4wTFRJdVkyOXRjSFYwWlM1aGJXRjZiMjVoZDNNdVkyOXRod1NzRkNCWmh3UTIKdWZhQWhqWnplWE4wWlcwNmJtOWtaVHBwY0MweE56SXRNakF0TXpJdE9Ea3VkWE10ZDJWemRDMHlMbU52YlhCMQpkR1V1YVc1MFpYSnVZV3d3Q2dZSUtvWkl6ajBFQXdJRFJ3QXdSQUlnVTUrNEFkWVcvRm9kdDExMmgvRjV4RHFQClFJS1BJemk4TUJMSTBBaVE2cGtDSUdqOHZPNDlTQldJVlo2SnhJL1lENldrRVhXdlZEbFp4cjFlZmVMM0NIeEgKLS0tLS1FTkQgQ0VSVElGSUNBVEUgUkVRVUVTVC0tLS0tCg==" | base64 --decode > /tmp/csr.out && openssl req -text -noout -in /tmp/csr.out

*/
