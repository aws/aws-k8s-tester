// Package csrs implements tester for CertificateSigningRequest.
package csrs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sort"
	"sync"
	"time"

	aws_s3 "github.com/aws/aws-k8s-tester/pkg/aws/s3"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/metrics"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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

// Config configures CSR loader.
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}

	S3API        s3iface.S3API
	S3BucketName string

	CWAPI cloudwatchiface.CloudWatchAPI

	Client        k8s_client.EKS
	ClientTimeout time.Duration

	Objects int

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
	InitialRequestConditionType string

	WritesRawJSONPath       string
	WritesRawJSONS3Key      string
	WritesSummaryJSONPath   string
	WritesSummaryJSONS3Key  string
	WritesSummaryTablePath  string
	WritesSummaryTableS3Key string
}

// Loader defines CSR loader operations.
type Loader interface {
	Start()
	Stop()
	CollectMetrics() (writesSummary metrics.RequestsSummary, err error)
}

type loader struct {
	cfg            Config
	donec          chan struct{}
	donecCloseOnce *sync.Once

	writeLatencies metrics.Durations
}

func New(cfg Config) Loader {
	return &loader{
		cfg:            cfg,
		donec:          make(chan struct{}),
		donecCloseOnce: new(sync.Once),
	}
}

func (ld *loader) Start() {
	ld.cfg.Logger.Info("starting write function")
	ld.writeLatencies = startWrites(ld.cfg.Logger, ld.cfg.Client.KubernetesClientSet(), ld.cfg.ClientTimeout, ld.cfg.Objects, ld.cfg.InitialRequestConditionType, ld.cfg.Stopc, ld.donec)
	ld.cfg.Logger.Info("completed write function")
}

func (ld *loader) Stop() {
	ld.cfg.Logger.Info("stopping and waiting for write function")
	ld.donecCloseOnce.Do(func() {
		close(ld.donec)
	})
	ld.cfg.Logger.Info("stopped and waited for write function")
}

// GetMetrics locally fetches output from registered metrics.
// ref. https://pkg.go.dev/github.com/prometheus/client_golang@v1.6.0/prometheus/promhttp?tab=doc#Handler
func (ts *loader) CollectMetrics() (writesSummary metrics.RequestsSummary, err error) {
	writesSummary = metrics.RequestsSummary{TestID: time.Now().UTC().Format(time.RFC3339Nano)}

	// https://pkg.go.dev/github.com/prometheus/client_golang/prometheus?tab=doc#Gatherer
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		ts.cfg.Logger.Warn("failed to gather prometheus metrics", zap.Error(err))
		return metrics.RequestsSummary{}, err
	}
	for _, mf := range mfs {
		if mf == nil {
			continue
		}
		switch *mf.Name {
		case "csrs_client_write_requests_success_total":
			gg := mf.Metric[0].GetGauge()
			writesSummary.SuccessTotal = gg.GetValue()
		case "csrs_client_write_requests_failure_total":
			gg := mf.Metric[0].GetGauge()
			writesSummary.FailureTotal = gg.GetValue()
		case "csrs_client_write_request_latency_milliseconds":
			writesSummary.LatencyHistogram, err = metrics.ParseHistogram("milliseconds", mf.Metric[0].GetHistogram())
			if err != nil {
				return metrics.RequestsSummary{}, err
			}
		}
	}

	ts.cfg.Logger.Info("sorting write latency results", zap.Int("total-data-points", ts.writeLatencies.Len()))
	now := time.Now()
	sort.Sort(ts.writeLatencies)
	ts.cfg.Logger.Info("sorted write latency results", zap.Int("total-data-points", ts.writeLatencies.Len()), zap.String("took", time.Since(now).String()))
	writesSummary.LantencyP50 = ts.writeLatencies.PickLantencyP50()
	writesSummary.LantencyP90 = ts.writeLatencies.PickLantencyP90()
	writesSummary.LantencyP99 = ts.writeLatencies.PickLantencyP99()
	writesSummary.LantencyP999 = ts.writeLatencies.PickLantencyP999()
	writesSummary.LantencyP9999 = ts.writeLatencies.PickLantencyP9999()

	ts.cfg.Logger.Info("writing latency results in JSON to disk", zap.String("path", ts.cfg.WritesRawJSONPath))
	wb, err := json.Marshal(ts.writeLatencies)
	if err != nil {
		ts.cfg.Logger.Warn("failed to encode latency results in JSON", zap.Error(err))
		return metrics.RequestsSummary{}, err
	}
	if err = ioutil.WriteFile(ts.cfg.WritesRawJSONPath, wb, 0600); err != nil {
		ts.cfg.Logger.Warn("failed to write latency results in JSON to disk", zap.String("path", ts.cfg.WritesRawJSONPath), zap.Error(err))
		return metrics.RequestsSummary{}, err
	}
	if err = aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.S3BucketName,
		ts.cfg.WritesRawJSONS3Key,
		ts.cfg.WritesRawJSONPath,
	); err != nil {
		return metrics.RequestsSummary{}, err
	}

	if err = ioutil.WriteFile(ts.cfg.WritesSummaryJSONPath, []byte(writesSummary.JSON()), 0600); err != nil {
		ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
		return metrics.RequestsSummary{}, err
	}
	if err = aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.S3BucketName,
		ts.cfg.WritesSummaryJSONS3Key,
		ts.cfg.WritesSummaryJSONPath,
	); err != nil {
		return metrics.RequestsSummary{}, err
	}
	if err = ioutil.WriteFile(ts.cfg.WritesSummaryTablePath, []byte(writesSummary.Table()), 0600); err != nil {
		ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
		return metrics.RequestsSummary{}, err
	}
	if err = aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.S3BucketName,
		ts.cfg.WritesSummaryTableS3Key,
		ts.cfg.WritesSummaryTablePath,
	); err != nil {
		return metrics.RequestsSummary{}, err
	}
	fmt.Printf("\n\nWritesSummaryTable:\n%s\n", writesSummary.Table())

	return writesSummary, nil
}

func startWrites(lg *zap.Logger, cli *kubernetes.Clientset, timeout time.Duration, objects int, condType string, stopc chan struct{}, donec chan struct{}) (ds metrics.Durations) {
	lg.Info("starting startWrites", zap.Int("objects", objects))
	ds = make(metrics.Durations, 0, 20000)

	for i := 0; i < objects; i++ {
		select {
		case <-stopc:
			lg.Warn("writes stopped")
			return
		case <-donec:
			lg.Info("writes done")
			return
		default:
		}

		// only letters and numbers for CSR key names
		key := fmt.Sprintf("csr%d%s", i, randutil.String(7))
		cd := createCond(i, "test via "+key, condType)

		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		_, err := cli.
			CertificatesV1beta1().
			CertificateSigningRequests().
			Create(ctx, &certificatesv1beta1.CertificateSigningRequest{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "certificates.k8s.io/v1beta1",
					Kind:       "CertificateSigningRequest",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:              key,
					GenerateName:      key,
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-20 * time.Minute)},
				},
				Spec: certificatesv1beta1.CertificateSigningRequestSpec{
					Groups:  []string{"system:bootstrappers", "system:nodes", "system:authenticated"},
					Request: reqData,
					UID:     "heptio-authenticator-aws:280347406217:AROAUCRQB56EUYTYXXJKV",
					Usages: []certificatesv1beta1.KeyUsage{
						certificatesv1beta1.UsageDigitalSignature,
						certificatesv1beta1.UsageKeyEncipherment,
						certificatesv1beta1.UsageServerAuth,
					},
					Username: "system:node:ip-172-20-32-89.us-west-2.compute.internal",
				},
				Status: certificatesv1beta1.CertificateSigningRequestStatus{
					Certificate: nil,
					Conditions:  cd,
				},
			}, metav1.CreateOptions{})
		cancel()
		took := time.Since(start)
		tookMS := float64(took / time.Millisecond)
		writeRequestLatencyMs.Observe(tookMS)
		ds = append(ds, took)
		if err != nil {
			writeRequestsFailureTotal.Inc()
			lg.Warn("write csr failed", zap.Error(err))
		} else {
			writeRequestsSuccessTotal.Inc()
			if i%20 == 0 {
				lg.Info("wrote csr", zap.Int("iteration", i))
			}
		}
	}
	return ds
}

var conds = []certificatesv1beta1.RequestConditionType{
	certificatesv1beta1.CertificateApproved,
	certificatesv1beta1.CertificateDenied,
	certificatesv1beta1.RequestConditionType(""),
}

func createCond(idx int, msg string, tp string) (cs []certificatesv1beta1.CertificateSigningRequestCondition) {
	cs = []certificatesv1beta1.CertificateSigningRequestCondition{
		{
			Reason:         "Test",
			Message:        msg,
			LastUpdateTime: metav1.NewTime(time.Now().Add(-time.Hour)),
		},
	}
	switch tp {
	case string(certificatesv1beta1.CertificateApproved):
		cs[0].Type = certificatesv1beta1.CertificateApproved
	case string(certificatesv1beta1.CertificateDenied):
		cs[0].Type = certificatesv1beta1.CertificateDenied
	case "Pending", "":
		cs = make([]certificatesv1beta1.CertificateSigningRequestCondition, 0)
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
