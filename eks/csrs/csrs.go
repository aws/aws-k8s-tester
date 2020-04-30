// Package csrs implements tester for CertificateSigningRequest.
package csrs

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Config defines "CertificateSigningRequest" configuration.
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	Sig       chan os.Signal
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

// Tester defines CertificateSigningRequest tester.
type Tester interface {
	// Create creates "CertificateSigningRequest" objects to test CertificateSigningRequest.
	Create() error
	// Delete deletes "CertificateSigningRequest" and Pods.
	Delete() error
}

// New creates a new CSR tester.
func New(cfg Config) (Tester, error) {
	return &tester{cfg: cfg, cancel: make(chan struct{})}, nil
}

type tester struct {
	cfg               Config
	deploymentCreated time.Time
	cancel            chan struct{}
}

func (ts *tester) Create() error {
	if ts.cfg.EKSConfig.AddOnCSRs.Created {
		ts.cfg.Logger.Info("skipping create AddOnCSRs")
		return nil
	}

	ts.cfg.EKSConfig.AddOnCSRs.Created = true
	ts.cfg.EKSConfig.Sync()

	createStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnCSRs.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.AddOnCSRs.CreateTookString = ts.cfg.EKSConfig.AddOnCSRs.CreateTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	if err := k8s_client.CreateNamespace(ts.cfg.Logger, ts.cfg.K8SClient.KubernetesClientSet(), ts.cfg.EKSConfig.AddOnCSRs.Namespace); err != nil {
		return err
	}
	if err := ts.createCSRs(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.AddOnCSRs.Created {
		ts.cfg.Logger.Info("skipping delete AddOnCSRs")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnCSRs.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.AddOnCSRs.DeleteTookString = ts.cfg.EKSConfig.AddOnCSRs.DeleteTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	if err := k8s_client.DeleteNamespaceAndWait(ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnCSRs.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout); err != nil {
		return fmt.Errorf("failed to delete CSRs namespace (%v)", err)
	}

	ts.cfg.EKSConfig.AddOnCSRs.Created = false
	return ts.cfg.EKSConfig.Sync()
}

// only letters and numbers for CSR key names
var regex = regexp.MustCompile("[^a-zA-Z0-9]+")

func (ts *tester) createCSRs() (err error) {
	ts.cfg.Logger.Info("creating CSRs",
		zap.Int("objects", ts.cfg.EKSConfig.AddOnCSRs.Objects),
	)

	// valid config key must consist of alphanumeric characters
	pfx := strings.ToLower(regex.ReplaceAllString(ts.cfg.EKSConfig.Name, ""))

	// overwrite if any
	ts.cfg.EKSConfig.AddOnCSRs.CreatedNames = make([]string, 0, ts.cfg.EKSConfig.AddOnCSRs.Objects)
	ts.cfg.EKSConfig.Sync()

	if ts.cfg.EKSConfig.ClientQPS <= 1 {
		err = ts.createCSRsSequential(pfx, ts.cfg.EKSConfig.AddOnCSRs.FailThreshold)
	} else {
		err = ts.createCSRsParallel(pfx, ts.cfg.EKSConfig.AddOnCSRs.FailThreshold)
	}
	ts.cfg.EKSConfig.Sync()
	return err
}

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

var reqData, _ = base64.StdEncoding.DecodeString("LS0tLS1CRUdJTiBDRVJUSUZJQ0FURSBSRVFVRVNULS0tLS0KTUlJQnJEQ0NBVk1DQVFBd1dERVZNQk1HQTFVRUNoTU1jM2x6ZEdWdE9tNXZaR1Z6TVQ4d1BRWURWUVFERXpaegplWE4wWlcwNmJtOWtaVHBwY0MweE56SXRNakF0TXpJdE9Ea3VkWE10ZDJWemRDMHlMbU52YlhCMWRHVXVhVzUwClpYSnVZV3d3V1RBVEJnY3Foa2pPUFFJQkJnZ3Foa2pPUFFNQkJ3TkNBQVJGSzI3L2w4U2NtMXF1K2xXbEs5WFoKUUtVM0grSnFENTZuSEFYOXBUQ25YVWRQaUppemRzc01QaSs2emtCU1I2MXVJcVRsdnNIcjkwbFNyU2tQeDd1aQpvSUdZTUlHVkJna3Foa2lHOXcwQkNRNHhnWWN3Z1lRd2dZRUdBMVVkRVFSNk1IaUNNbVZqTWkwMU5DMHhPRFV0Ck1qUTJMVEV5T0M1MWN5MTNaWE4wTFRJdVkyOXRjSFYwWlM1aGJXRjZiMjVoZDNNdVkyOXRod1NzRkNCWmh3UTIKdWZhQWhqWnplWE4wWlcwNmJtOWtaVHBwY0MweE56SXRNakF0TXpJdE9Ea3VkWE10ZDJWemRDMHlMbU52YlhCMQpkR1V1YVc1MFpYSnVZV3d3Q2dZSUtvWkl6ajBFQXdJRFJ3QXdSQUlnVTUrNEFkWVcvRm9kdDExMmgvRjV4RHFQClFJS1BJemk4TUJMSTBBaVE2cGtDSUdqOHZPNDlTQldJVlo2SnhJL1lENldrRVhXdlZEbFp4cjFlZmVMM0NIeEgKLS0tLS1FTkQgQ0VSVElGSUNBVEUgUkVRVUVTVC0tLS0tCg==")

func (ts *tester) createCSR(idx int, name string) *certificatesv1beta1.CertificateSigningRequest {
	return &certificatesv1beta1.CertificateSigningRequest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "certificates.k8s.io/v1beta1",
			Kind:       "CertificateSigningRequest",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         ts.cfg.EKSConfig.AddOnCSRs.Namespace,
			GenerateName:      name,
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
			Conditions:  pickCond(idx, "Testing via "+ts.cfg.EKSConfig.Name, ts.cfg.EKSConfig.AddOnCSRs.InitialRequestConditionType),
		},
	}
}

func (ts *tester) createCSRsSequential(pfx string, failThreshold int) error {
	qps := float64(ts.cfg.EKSConfig.ClientQPS)
	burst := int(ts.cfg.EKSConfig.ClientBurst)
	ts.cfg.Logger.Info("creating CSRs sequential",
		zap.Float64("qps", qps),
		zap.Int("burst", burst),
	)

	fails := 0
	for i := 0; i < ts.cfg.EKSConfig.AddOnCSRs.Objects; i++ {
		key := fmt.Sprintf("%s%06d", pfx, i)
		req := ts.createCSR(i, key)
		t1 := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		csr, err := ts.cfg.K8SClient.KubernetesClientSet().
			CertificatesV1beta1().
			CertificateSigningRequests().
			Create(ctx, req, metav1.CreateOptions{})
		cancel()
		t2 := time.Now()
		if err != nil {
			select {
			case <-ts.cancel:
				return errors.New("CSR creation aborted")
			case <-ts.cfg.Stopc:
				return errors.New("CSR creation aborted")
			default:
				fails++
				ts.cfg.Logger.Warn("create CSR failed",
					zap.Int("fails", fails),
					zap.Int("threshold", failThreshold),
					zap.Error(err),
				)
				if fails >= failThreshold {
					close(ts.cancel)
					return fmt.Errorf("exceeded CSR writes fail threshold %d (%v)", failThreshold, err)
				}
			}
			continue
		}
		fails = 0

		csrName := csr.GetObjectMeta().GetName()
		ts.cfg.EKSConfig.AddOnCSRs.CreatedNames = append(ts.cfg.EKSConfig.AddOnCSRs.CreatedNames, csrName)
		ts.cfg.EKSConfig.Sync()

		if ts.cfg.EKSConfig.LogLevel == "debug" || i%50 == 0 {
			ts.cfg.Logger.Info(
				"created CSR",
				zap.String("key", csrName),
				zap.Int("index", i),
				zap.Int("created", len(ts.cfg.EKSConfig.AddOnCSRs.CreatedNames)),
				zap.Int("target-total", ts.cfg.EKSConfig.AddOnCSRs.Objects),
				zap.Duration("took", t2.Sub(t1)),
			)
		}
	}

	ts.cfg.Logger.Info("created CSRs sequential",
		zap.Int("objects", ts.cfg.EKSConfig.AddOnCSRs.Objects),
		zap.Int("success", len(ts.cfg.EKSConfig.AddOnCSRs.CreatedNames)),
	)
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createCSRsParallel(pfx string, failThreshold int) error {
	qps := float64(ts.cfg.EKSConfig.ClientQPS)
	burst := int(ts.cfg.EKSConfig.ClientBurst)
	rateLimiter := rate.NewLimiter(rate.Limit(qps), burst)
	ts.cfg.Logger.Info("creating CSRs parallel",
		zap.Float64("qps", qps),
		zap.Int("burst", burst),
	)

	rch := make(chan result, int(qps))
	for i := 0; i < ts.cfg.EKSConfig.AddOnCSRs.Objects; i++ {
		go func(i int) {
			if !rateLimiter.Allow() {
				ts.cfg.Logger.Debug("waiting for rate limiter creating CSR", zap.Int("index", i))
				werr := rateLimiter.Wait(context.Background())
				ts.cfg.Logger.Debug("waited for rate limiter", zap.Int("index", i), zap.Error(werr))
			}
			select {
			case <-ts.cancel:
				return
			case <-ts.cfg.Stopc:
				return
			default:
			}

			key := fmt.Sprintf("%s%06d", pfx, i)
			req := ts.createCSR(i, key)
			t1 := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			csr, err := ts.cfg.K8SClient.KubernetesClientSet().
				CertificatesV1beta1().
				CertificateSigningRequests().
				Create(ctx, req, metav1.CreateOptions{})
			cancel()
			t2 := time.Now()
			if err != nil {
				select {
				case <-ts.cancel:
					ts.cfg.Logger.Warn("exiting")
					return
				case <-ts.cfg.Stopc:
					ts.cfg.Logger.Warn("exiting")
					return
				case rch <- result{csr: csr, err: err, took: t2.Sub(t1), start: t1, end: t2}:
				}
				return
			}

			select {
			case <-ts.cancel:
				ts.cfg.Logger.Warn("exiting")
				return
			case <-ts.cfg.Stopc:
				ts.cfg.Logger.Warn("exiting")
				return
			case rch <- result{csr: csr, err: nil, took: t2.Sub(t1), start: t1, end: t2}:
			}

			if ts.cfg.EKSConfig.LogLevel == "debug" || i%50 == 0 {
				ts.cfg.Logger.Info("created CSR",
					zap.String("key", csr.GetObjectMeta().GetName()),
					zap.Int("index", i),
					zap.Int("created", len(ts.cfg.EKSConfig.AddOnCSRs.CreatedNames)),
					zap.Int("target-total", ts.cfg.EKSConfig.AddOnCSRs.Objects),
					zap.Duration("took", t2.Sub(t1)),
				)
			}
		}(i)
	}

	fails := 0
	for i := 0; i < ts.cfg.EKSConfig.AddOnCSRs.Objects; i++ {
		var rv result
		select {
		case rv = <-rch:
		case <-ts.cancel:
			ts.cfg.Logger.Warn("exiting")
			return errors.New("aborted")
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("exiting")
			return errors.New("aborted")
		}
		if rv.err != nil {
			fails++
			ts.cfg.Logger.Warn("create CSR failed",
				zap.Int("fails", fails),
				zap.Int("threshold", failThreshold),
				zap.Error(rv.err),
			)
			if fails >= failThreshold {
				close(ts.cancel)
				return fmt.Errorf("exceeded CSR writes fail threshold %d (%v)", failThreshold, rv.err)
			}
			continue
		}
		fails = 0

		csrName := rv.csr.GetObjectMeta().GetName()
		ts.cfg.EKSConfig.AddOnCSRs.CreatedNames = append(ts.cfg.EKSConfig.AddOnCSRs.CreatedNames, csrName)
		ts.cfg.EKSConfig.Sync()
	}

	ts.cfg.Logger.Info("created CSRs parallel",
		zap.Int("objects", ts.cfg.EKSConfig.AddOnCSRs.Objects),
		zap.Int("success", len(ts.cfg.EKSConfig.AddOnCSRs.CreatedNames)),
	)
	return ts.cfg.EKSConfig.Sync()
}

type result struct {
	csr   *certificatesv1beta1.CertificateSigningRequest
	err   error
	took  time.Duration
	start time.Time
	end   time.Time
}

var conds = []certificatesv1beta1.RequestConditionType{
	certificatesv1beta1.CertificateApproved,
	certificatesv1beta1.CertificateDenied,
	certificatesv1beta1.RequestConditionType(""),
}

func pickCond(idx int, msg string, tp string) (cs []certificatesv1beta1.CertificateSigningRequestCondition) {
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
