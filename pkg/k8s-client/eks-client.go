package k8sclient

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/utils/exec"
)

// EKSConfig defines EKS client configuration.
type EKSConfig struct {
	Logger *zap.Logger

	Region string

	ClusterName              string
	ClusterAPIServerEndpoint string
	ClusterCADecoded         string

	// ClientQPS is the QPS for kubernetes client.
	// To use while talking with kubernetes apiserver.
	//
	// Kubernetes client DefaultQPS is 5.
	// Kubernetes client DefaultBurst is 10.
	// ref. https://github.com/kubernetes/kubernetes/blob/4d0e86f0b8d1eae00a202009858c8739e4c9402e/staging/src/k8s.io/client-go/rest/config.go#L43-L46
	//
	// kube-apiserver default inflight requests limits are:
	// FLAG: --max-mutating-requests-inflight="200"
	// FLAG: --max-requests-inflight="400"
	// ref. https://github.com/kubernetes/kubernetes/blob/4d0e86f0b8d1eae00a202009858c8739e4c9402e/staging/src/k8s.io/apiserver/pkg/server/config.go#L300-L301
	//
	ClientQPS float32
	// ClientBurst is the burst for kubernetes client.
	// To use while talking with kubernetes apiserver
	//
	// Kubernetes client DefaultQPS is 5.
	// Kubernetes client DefaultBurst is 10.
	// ref. https://github.com/kubernetes/kubernetes/blob/4d0e86f0b8d1eae00a202009858c8739e4c9402e/staging/src/k8s.io/client-go/rest/config.go#L43-L46
	//
	// kube-apiserver default inflight requests limits are:
	// FLAG: --max-mutating-requests-inflight="200"
	// FLAG: --max-requests-inflight="400"
	// ref. https://github.com/kubernetes/kubernetes/blob/4d0e86f0b8d1eae00a202009858c8739e4c9402e/staging/src/k8s.io/apiserver/pkg/server/config.go#L300-L301
	//
	ClientBurst int

	KubectlPath string

	KubeConfigPath    string
	KubeConfigContext string

	ServerVersion string

	EncryptionEnabled bool
}

// EKS defines EKS client operations.
type EKS interface {
	// KubernetesClientSet returns a new kubernetes client set.
	KubernetesClientSet() *kubernetes.Clientset
	// CheckEKSHealth checks the EKS health.
	CheckHealth() error
}

type eks struct {
	cfg *EKSConfig
	cli *kubernetes.Clientset
	mu  sync.Mutex
}

// KubernetesClientSet returns a new kubernetes client set.
func (e *eks) KubernetesClientSet() *kubernetes.Clientset {
	return e.cli
}

// CheckHealth checks the EKS health.
func (e *eks) CheckHealth() error {
	// allow only one health check at a time
	e.mu.Lock()
	err := e.checkHealth()
	e.mu.Unlock()
	return err
}

// NewEKS returns a new EKS client.
func NewEKS(cfg *EKSConfig) (EKS, error) {
	if cfg == nil {
		return nil, errors.New("nil EKSConfig")
	}
	if cfg.Logger == nil {
		cfg.Logger = zap.NewExample()
	}
	var kcfg *restclient.Config
	var err error
	if cfg.KubeConfigPath != "" {
		switch {
		case cfg.KubeConfigContext != "":
			cfg.Logger.Info("creating k8s client using KUBECONFIG and context",
				zap.String("kubeconfig", cfg.KubeConfigPath),
				zap.String("context", cfg.KubeConfigContext),
			)
			kcfg, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
				&clientcmd.ClientConfigLoadingRules{
					ExplicitPath: cfg.KubeConfigPath,
				},
				&clientcmd.ConfigOverrides{
					CurrentContext: cfg.KubeConfigContext,
					ClusterInfo:    clientcmdapi.Cluster{Server: ""},
				},
			).ClientConfig()
		case cfg.KubeConfigContext == "":
			cfg.Logger.Info("creating k8s client using KUBECONFIG",
				zap.String("kubeconfig", cfg.KubeConfigPath),
			)
			kcfg, err = clientcmd.BuildConfigFromFlags("", cfg.KubeConfigPath)
		}
		if err != nil {
			cfg.Logger.Warn("failed to read kubeconfig", zap.Error(err))
		}
	}
	if kcfg == nil {
		kcfg = createClientConfigEKS(cfg)
	}
	if kcfg == nil {
		cfg.Logger.Warn("failed to create k8s client config")
		return nil, errors.New("failed to create k8s client config")
	}

	cfg.Logger.Info("loaded k8s client config",
		zap.String("host", kcfg.Host),
		zap.String("user-name", kcfg.Username),
		zap.String("server-name", kcfg.ServerName),
	)

	if cfg.ClusterAPIServerEndpoint == "" {
		cfg.ClusterAPIServerEndpoint = kcfg.Host
		cfg.Logger.Info("updated apiserver endpoint from kubeconfig", zap.String("apiserver-endpoint", kcfg.Host))
	} else if cfg.ClusterAPIServerEndpoint != kcfg.Host {
		cfg.Logger.Warn("unexpected apiserver endpoint",
			zap.String("apiserver-endpoint", cfg.ClusterAPIServerEndpoint),
			zap.String("kubeconfig-host", kcfg.Host),
		)
	}
	if cfg.ClusterAPIServerEndpoint == "" {
		return nil, errors.New("empty ClusterAPIServerEndpoint")
	}

	if cfg.ClusterCADecoded == "" {
		cfg.ClusterCADecoded = string(kcfg.TLSClientConfig.CAData)
		cfg.Logger.Info("updated cluster ca from kubeconfig", zap.String("cluster-ca", string(kcfg.TLSClientConfig.CAData)))
	} else if cfg.ClusterCADecoded != string(kcfg.TLSClientConfig.CAData) {
		cfg.Logger.Warn("unexpected cluster ca",
			zap.String("cluster-ca", cfg.ClusterCADecoded),
			zap.String("kubeconfig-cluster-ca", string(kcfg.TLSClientConfig.CAData)),
		)
	}
	if cfg.ClusterCADecoded == "" {
		return nil, errors.New("empty ClusterCADecoded")
	}

	if kcfg.AuthProvider != nil {
		if cfg.Region == "" {
			cfg.Region = kcfg.AuthProvider.Config["region"]
			cfg.Logger.Info("updated region from kubeconfig", zap.String("apiserver-endpoint", kcfg.AuthProvider.Config["region"]))
		} else if cfg.Region != kcfg.AuthProvider.Config["region"] {
			cfg.Logger.Warn("unexpected region",
				zap.String("apiserver-endpoint", cfg.Region),
				zap.String("kubeconfig-host", kcfg.AuthProvider.Config["region"]),
			)
		}
		if cfg.ClusterName == "" {
			cfg.ClusterName = kcfg.AuthProvider.Config["cluster-name"]
			cfg.Logger.Info("updated cluster-name from kubeconfig", zap.String("apiserver-endpoint", kcfg.AuthProvider.Config["cluster-name"]))
		} else if cfg.ClusterName != kcfg.AuthProvider.Config["cluster-name"] {
			cfg.Logger.Warn("unexpected cluster-name",
				zap.String("apiserver-endpoint", cfg.ClusterName),
				zap.String("kubeconfig-host", kcfg.AuthProvider.Config["cluster-name"]),
			)
		}
	}
	if cfg.Region == "" {
		cfg.Logger.Warn("no region found in k8s client")
	}
	if cfg.ClusterName == "" {
		cfg.Logger.Warn("no cluster name found in k8s client")
	}

	if cfg.ClientQPS > 0 {
		kcfg.QPS = cfg.ClientQPS
	}
	if cfg.ClientBurst > 0 {
		kcfg.Burst = cfg.ClientBurst
	}

	ek := &eks{cfg: cfg}
	ek.cli, err = clientset.NewForConfig(kcfg)
	if err != nil {
		cfg.Logger.Warn("failed to create k8s client", zap.Error(err))
		return nil, err
	}
	cfg.Logger.Info("created k8s client", zap.Float32("qps", kcfg.QPS), zap.Int("burst", kcfg.Burst))

	return ek, nil
}

const authProviderName = "eks"

func createClientConfigEKS(cfg *EKSConfig) *restclient.Config {
	if cfg.Region == "" {
		return nil
	}
	if cfg.ClusterName == "" {
		return nil
	}
	if cfg.ClusterAPIServerEndpoint == "" {
		return nil
	}
	if cfg.ClusterCADecoded == "" {
		return nil
	}
	cfg.Logger.Info("creating k8s client using status")
	return &restclient.Config{
		Host: cfg.ClusterAPIServerEndpoint,
		TLSClientConfig: restclient.TLSClientConfig{
			CAData: []byte(cfg.ClusterCADecoded),
		},
		AuthProvider: &clientcmdapi.AuthProviderConfig{
			Name: authProviderName,
			Config: map[string]string{
				"region":       cfg.Region,
				"cluster-name": cfg.ClusterName,
			},
		},
	}
}

func init() {
	restclient.RegisterAuthProviderPlugin(authProviderName, newAuthProviderEKS)
}

func newAuthProviderEKS(_ string, config map[string]string, _ restclient.AuthProviderConfigPersister) (restclient.AuthProvider, error) {
	awsRegion, ok := config["region"]
	if !ok {
		return nil, fmt.Errorf("'clientcmdapi.AuthProviderConfig' does not include 'region' key %+v", config)
	}
	clusterName, ok := config["cluster-name"]
	if !ok {
		return nil, fmt.Errorf("'clientcmdapi.AuthProviderConfig' does not include 'cluster-name' key %+v", config)
	}

	sess := session.Must(session.NewSession(aws.NewConfig().WithRegion(awsRegion)))
	return &eksAuthProvider{ts: newTokenSourceEKS(sess, clusterName)}, nil
}

type eksAuthProvider struct {
	ts oauth2.TokenSource
}

func (p *eksAuthProvider) WrapTransport(rt http.RoundTripper) http.RoundTripper {
	return &oauth2.Transport{
		Source: p.ts,
		Base:   rt,
	}
}

func (p *eksAuthProvider) Login() error {
	return nil
}

func newTokenSourceEKS(sess *session.Session, clusterName string) oauth2.TokenSource {
	return &eksTokenSource{sess: sess, clusterName: clusterName}
}

type eksTokenSource struct {
	sess        *session.Session
	clusterName string
}

// Reference
// https://github.com/kubernetes-sigs/aws-iam-authenticator/blob/master/README.md#api-authorization-from-outside-a-cluster
// https://github.com/kubernetes-sigs/aws-iam-authenticator/blob/master/pkg/token/token.go
const (
	v1Prefix        = "k8s-aws-v1."
	clusterIDHeader = "x-k8s-aws-id"
)

func (s *eksTokenSource) Token() (*oauth2.Token, error) {
	stsAPI := sts.New(s.sess)
	request, _ := stsAPI.GetCallerIdentityRequest(&sts.GetCallerIdentityInput{})
	request.HTTPRequest.Header.Add(clusterIDHeader, s.clusterName)

	payload, err := request.Presign(60)
	if err != nil {
		return nil, err
	}
	token := v1Prefix + base64.RawURLEncoding.EncodeToString([]byte(payload))
	tokenExpiration := time.Now().Local().Add(14 * time.Minute)
	return &oauth2.Token{
		AccessToken: token,
		TokenType:   "Bearer",
		Expiry:      tokenExpiration,
	}, nil
}

func (e *eks) checkHealth() error {
	if e.cfg == nil {
		return errors.New("nil EKSConfig")
	}
	if e.cfg.KubectlPath == "" {
		return errors.New("empty EKSConfig.KubectlPath")
	}
	if e.cfg.KubeConfigPath == "" {
		return errors.New("empty EKSConfig.KubeConfigPath")
	}
	if e.cfg.ClusterAPIServerEndpoint == "" {
		return errors.New("empty EKSConfig.ClusterAPIServerEndpoint")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err := exec.New().CommandContext(
		ctx,
		e.cfg.KubectlPath,
		"--kubeconfig="+e.cfg.KubeConfigPath,
		"version",
	).CombinedOutput()
	cancel()
	out := string(output)
	if err != nil {
		return fmt.Errorf("'kubectl version' failed %v (output %q)", err, out)
	}
	fmt.Printf("\n\"kubectl version\" output:\n%s\n", out)

	ep := e.cfg.ClusterAPIServerEndpoint + "/version"
	buf := bytes.NewBuffer(nil)
	if err = httpReadInsecure(e.cfg.Logger, ep, buf); err != nil {
		return err
	}
	out = buf.String()
	if e.cfg.ServerVersion != "" && !strings.Contains(out, fmt.Sprintf(`"gitVersion": "v%s`, e.cfg.ServerVersion)) {
		return fmt.Errorf("%q does not contain version %q", out, e.cfg.ServerVersion)
	}
	fmt.Printf("\n\n\"%s\" output:\n%s\n", ep, out)

	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	output, err = exec.New().CommandContext(
		ctx,
		e.cfg.KubectlPath,
		"--kubeconfig="+e.cfg.KubeConfigPath,
		"cluster-info",
	).CombinedOutput()
	cancel()
	out = string(output)
	if err != nil {
		return fmt.Errorf("'kubectl cluster-info' failed %v (output %q)", err, out)
	}
	if !strings.Contains(out, "is running at") {
		return fmt.Errorf("'kubectl cluster-info' not ready (output %q)", out)
	}
	fmt.Printf("\n\"kubectl cluster-info\" output:\n%s\n", out)

	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	output, err = exec.New().CommandContext(
		ctx,
		e.cfg.KubectlPath,
		"--kubeconfig="+e.cfg.KubeConfigPath,
		"get",
		"cs",
	).CombinedOutput()
	cancel()
	out = string(output)
	if err != nil {
		return fmt.Errorf("'kubectl get cs' failed %v (output %q)", err, out)
	}
	fmt.Printf("\n\"kubectl get cs\" output:\n%s\n", out)

	ep = e.cfg.ClusterAPIServerEndpoint + "/healthz?verbose"
	buf.Reset()
	if err := httpReadInsecure(e.cfg.Logger, ep, buf); err != nil {
		return err
	}
	out = buf.String()
	if !strings.Contains(out, "healthz check passed") {
		return fmt.Errorf("%q does not contain 'healthz check passed'", out)
	}
	fmt.Printf("\n\n\"%s\" output (\"kubectl get --raw /healthz?verbose\"):\n%s\n", ep, out)

	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	output, err = exec.New().CommandContext(
		ctx,
		e.cfg.KubectlPath,
		"--kubeconfig="+e.cfg.KubeConfigPath,
		"--namespace=kube-system",
		"get",
		"all",
	).CombinedOutput()
	cancel()
	out = string(output)
	if err != nil {
		return fmt.Errorf("'kubectl get all -n=kube-system' failed %v (output %q)", err, out)
	}
	fmt.Printf("\n\"kubectl all -n=kube-system\" output:\n%s", out)

	fmt.Printf("\n\"kubectl get pods -n=kube-system\" output:\n")
	pods, err := e.cli.CoreV1().Pods("kube-system").List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pods %v", err)
	}
	for _, v := range pods.Items {
		fmt.Printf("kube-system Pod: %q\n", v.Name)
	}
	println()

	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	output, err = exec.New().CommandContext(
		ctx,
		e.cfg.KubectlPath,
		"--kubeconfig="+e.cfg.KubeConfigPath,
		"get",
		"configmaps",
		"--namespace=kube-system",
	).CombinedOutput()
	cancel()
	out = string(output)
	if err != nil {
		return fmt.Errorf("'kubectl get configmaps --namespace=kube-system' failed %v (output %q)", err, out)
	}
	fmt.Printf("\n\"kubectl get configmaps --namespace=kube-system\" output:\n%s\n", out)

	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	output, err = exec.New().CommandContext(
		ctx,
		e.cfg.KubectlPath,
		"--kubeconfig="+e.cfg.KubeConfigPath,
		"get",
		"namespaces",
	).CombinedOutput()
	cancel()
	out = string(output)
	if err != nil {
		return fmt.Errorf("'kubectl get namespaces' failed %v (output %q)", err, out)
	}
	fmt.Printf("\n\"kubectl get namespaces\" output:\n%s\n", out)

	fmt.Printf("\n\"curl -sL http://localhost:8080/metrics | grep storage_\" output:\n")
	output, err = e.cli.
		CoreV1().
		RESTClient().
		Get().
		RequestURI("/metrics").
		Do().
		Raw()
	if err != nil {
		return fmt.Errorf("failed to fetch /metrics (%v)", err)
	}
	const (
		metricDEKGen            = "apiserver_storage_data_key_generation_latencies_microseconds_count"
		metricEnvelopeCacheMiss = "apiserver_storage_envelope_transformation_cache_misses_total"
	)
	dekGenCnt, cacheMissCnt := int64(0), int64(0)
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "# "):
			continue

		case strings.HasPrefix(line, metricDEKGen+" "):
			vs := strings.TrimSpace(strings.Replace(line, metricDEKGen, "", -1))
			dekGenCnt, err = strconv.ParseInt(vs, 10, 64)
			if err != nil {
				e.cfg.Logger.Warn("failed to parse",
					zap.String("line", line),
					zap.Error(err),
				)
			}

		case strings.HasPrefix(line, metricEnvelopeCacheMiss+" "):
			vs := strings.TrimSpace(strings.Replace(line, metricEnvelopeCacheMiss, "", -1))
			cacheMissCnt, err = strconv.ParseInt(vs, 10, 64)
			if err != nil {
				e.cfg.Logger.Warn("failed to parse",
					zap.String("line", line),
					zap.Error(err),
				)
			}
		}

		if dekGenCnt > 0 || cacheMissCnt > 0 {
			break
		}
	}
	e.cfg.Logger.Info("encryption metrics",
		zap.Int64("dek-gen-count", dekGenCnt),
		zap.Int64("cache-miss-count", cacheMissCnt),
	)
	if e.cfg.EncryptionEnabled {
		if dekGenCnt <= 0 && cacheMissCnt <= 0 {
			return errors.New("encrypted enabled, unexpected /metrics")
		}
		e.cfg.Logger.Info("successfully checked encryption")
	}

	e.cfg.Logger.Info("checked /metrics")
	return nil
}

// curl -k [URL]
func httpReadInsecure(lg *zap.Logger, u string, wr io.Writer) error {
	lg.Info("reading", zap.String("url", u))
	cli := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}}
	r, err := cli.Get(u)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode >= 400 {
		return fmt.Errorf("%q returned %d", u, r.StatusCode)
	}

	_, err = io.Copy(wr, r.Body)
	if err != nil {
		lg.Warn("failed to read", zap.String("url", u), zap.Error(err))
	} else {
		lg.Info("read",
			zap.String("url", u),
		)
	}
	return err
}
