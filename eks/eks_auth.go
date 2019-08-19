package eks

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"golang.org/x/oauth2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
)

// Reference
// https://github.com/kubernetes-sigs/aws-iam-authenticator/blob/master/README.md#api-authorization-from-outside-a-cluster
// https://github.com/kubernetes-sigs/aws-iam-authenticator/blob/master/pkg/token/token.go

const authProviderName = "aws-eks-token"

func (md *embedded) updateK8sClientSet() (err error) {
	md.k8sClientSet, err = kubernetes.NewForConfig(&rest.Config{
		Host: md.cfg.ClusterState.Endpoint,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: []byte(md.cfg.ClusterState.CADecoded),
		},
		AuthProvider: &api.AuthProviderConfig{
			Name: authProviderName,
			Config: map[string]string{
				// TODO: use this to support temporary credentials
				// "aws-credentials-path": md.awsCredsPath,

				"aws-region":   md.cfg.AWSRegion,
				"cluster-name": md.cfg.ClusterName,
			},
		},
	})
	return err
}

func init() {
	rest.RegisterAuthProviderPlugin(authProviderName, newAuthProvider)
}

func newAuthProvider(_ string, config map[string]string, _ rest.AuthProviderConfigPersister) (rest.AuthProvider, error) {
	// TODO: use this to support temporary credentials
	// awsCredentialsPath := config["aws-credentials-path"]

	awsRegion, ok := config["aws-region"]
	if !ok {
		return nil, fmt.Errorf("'api.AuthProviderConfig' does not include 'aws-region' key %+v", config)
	}
	clusterName, ok := config["cluster-name"]
	if !ok {
		return nil, fmt.Errorf("'api.AuthProviderConfig' does not include 'cluster-name' key %+v", config)
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
