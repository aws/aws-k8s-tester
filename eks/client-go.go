package eks

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"golang.org/x/oauth2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func (ts *Tester) updateK8sClientSet() (err error) {
	ts.lg.Info("creating k8s client config")
	cfg := ts.createClientConfig()
	if cfg == nil {
		return errors.New("*restclient.Config is nil")
	}
	ts.lg.Info("creating k8s client")
	ts.k8sClientSet, err = clientset.NewForConfig(cfg)
	if err == nil {
		ts.lg.Info("updated k8s client set")
	}
	return err
}

const authProviderName = "eks"

func (ts *Tester) createClientConfig() *restclient.Config {
	if ts.cfg.Name == "" {
		return nil
	}
	if ts.cfg.Region == "" {
		return nil
	}
	if ts.cfg.Status.ClusterAPIServerEndpoint == "" {
		return nil
	}
	if ts.cfg.Status.ClusterCADecoded == "" {
		return nil
	}
	return &restclient.Config{
		Host: ts.cfg.Status.ClusterAPIServerEndpoint,
		TLSClientConfig: restclient.TLSClientConfig{
			CAData: []byte(ts.cfg.Status.ClusterCADecoded),
		},
		AuthProvider: &clientcmdapi.AuthProviderConfig{
			Name: authProviderName,
			Config: map[string]string{
				"region":       ts.cfg.Region,
				"cluster-name": ts.cfg.Name,
			},
		},
	}
}

func init() {
	restclient.RegisterAuthProviderPlugin(authProviderName, newAuthProvider)
}

func newAuthProvider(_ string, config map[string]string, _ restclient.AuthProviderConfigPersister) (restclient.AuthProvider, error) {
	awsRegion, ok := config["region"]
	if !ok {
		return nil, fmt.Errorf("'clientcmdapi.AuthProviderConfig' does not include 'region' key %+v", config)
	}
	clusterName, ok := config["cluster-name"]
	if !ok {
		return nil, fmt.Errorf("'clientcmdapi.AuthProviderConfig' does not include 'cluster-name' key %+v", config)
	}

	sess := session.Must(session.NewSession(aws.NewConfig().WithRegion(awsRegion)))
	return &eksAuthProvider{ts: newTokenSource(sess, clusterName)}, nil
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

func newTokenSource(sess *session.Session, clusterName string) oauth2.TokenSource {
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

func (ts *Tester) listPods(ns string) error {
	pods, err := ts.getPods(ns)
	if err != nil {
		return err
	}
	println()
	for _, v := range pods.Items {
		fmt.Printf("%q Pod using client-go: %q\n", ns, v.Name)
	}
	println()
	return nil
}

func (ts *Tester) getPods(ns string) (*v1.PodList, error) {
	return ts.k8sClientSet.CoreV1().Pods(ns).List(metav1.ListOptions{})
}

func (ts *Tester) getAllNodes() (*v1.NodeList, error) {
	return ts.k8sClientSet.CoreV1().Nodes().List(metav1.ListOptions{})
}
