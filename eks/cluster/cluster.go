// Package cluster implements EKS cluster tester.
package cluster

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/aws/aws-k8s-tester/eks/cluster/wait"
	wait_v2 "github.com/aws/aws-k8s-tester/eks/cluster/wait-v2"
	"github.com/aws/aws-k8s-tester/eksconfig"
	aws_s3 "github.com/aws/aws-k8s-tester/pkg/aws/s3"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	aws_v2 "github.com/aws/aws-sdk-go-v2/aws"
	aws_ec2_v2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	aws_eks_v2 "github.com/aws/aws-sdk-go-v2/service/eks"
	aws_elbv2_v2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	aws_iam_v2 "github.com/aws/aws-sdk-go-v2/service/iam"
	aws_kms_v2 "github.com/aws/aws-sdk-go-v2/service/kms"
	aws_s3_v2 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

// see https://github.com/aws/aws-k8s-tester/blob/v1.6.0/eks/cluster/cluster.go for CloudFormation based workflow

// Config defines version upgrade configuration.
type Config struct {
	Logger    *zap.Logger
	LogWriter io.Writer
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config

	S3API   s3iface.S3API
	S3APIV2 *aws_s3_v2.Client

	IAMAPIV2 *aws_iam_v2.Client

	KMSAPIV2 *aws_kms_v2.Client

	EC2APIV2 *aws_ec2_v2.Client

	EKSAPI   eksiface.EKSAPI
	EKSAPIV2 *aws_eks_v2.Client

	ELBV2APIV2 *aws_elbv2_v2.Client

	CFNAPI cloudformationiface.CloudFormationAPI
}

type Tester interface {
	// Name returns the name of the tester.
	Name() string
	// Create creates EKS cluster, and waits for completion.
	Create() error
	Client() k8s_client.EKS
	// CheckHealth checks EKS cluster health.
	CheckHealth() error
	// Delete deletes all EKS cluster resources.
	Delete() error
}

var pkgName = reflect.TypeOf(tester{}).PkgPath()

func (ts *tester) Name() string { return pkgName }

// New creates a new Job tester.
func New(cfg Config) Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", pkgName))
	return &tester{cfg: cfg, checkHealthMu: new(sync.Mutex)}
}

type tester struct {
	// v2 SDK doesn't work...
	// ref. "api error ForbiddenException: Forbidden..."
	useV2SDK bool

	cfg       Config
	k8sClient k8s_client.EKS

	checkHealthMu *sync.Mutex
}

func (ts *tester) Create() (err error) {
	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))

	if err = ts.createEncryption(); err != nil {
		return err
	}
	if err = ts.createRole(); err != nil {
		return err
	}
	if err = ts.createVPC(); err != nil {
		return err
	}
	if err = ts.createEKS(); err != nil {
		return err
	}

	ts.k8sClient, err = ts.createClient()
	if err != nil {
		return err
	}
	if err = ts.CheckHealth(); err != nil {
		return err
	}

	ts.cfg.EKSConfig.Sync()
	return nil
}

func (ts *tester) Client() k8s_client.EKS { return ts.k8sClient }

func (ts *tester) CheckHealth() (err error) {
	ts.checkHealthMu.Lock()
	defer ts.checkHealthMu.Unlock()
	return ts.checkHealth(getCaller())
}

func getCaller() string {
	fpcs := make([]uintptr, 1)
	n := runtime.Callers(3, fpcs)
	if n == 0 {
		return "none"
	}
	fun := runtime.FuncForPC(fpcs[0] - 1)
	if fun == nil {
		return "none"
	}
	return fun.Name()
}

func (ts *tester) checkHealth(caller string) (err error) {
	fmt.Print(ts.cfg.EKSConfig.Colorize("\n\n[yellow]*********************************\n"))
	fmt.Printf(ts.cfg.EKSConfig.Colorize("[light_green]checkHealth [default](%q, caller %q)\n"), ts.cfg.EKSConfig.ConfigPath, caller)

	defer func() {
		if err == nil {
			ts.cfg.EKSConfig.RecordStatus(eks.ClusterStatusActive)
		}
	}()

	// TODO: investigate why "ts.k8sClient == nil" after cluster creation
	if ts.k8sClient == nil {
		ts.cfg.Logger.Info("empty client; creating client")
		ts.k8sClient, err = ts.createClient()
		if err != nil {
			return err
		}
	}

	// might take several minutes for DNS to propagate
	waitDur := 5 * time.Minute
	retryStart := time.Now()
	for time.Since(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("health check aborted")
		case <-time.After(5 * time.Second):
		}
		if ts.cfg.EKSConfig.Status == nil {
			ts.cfg.Logger.Warn("empty EKSConfig.Status")
		} else {
			ts.cfg.EKSConfig.Status.ServerVersionInfo, err = ts.k8sClient.FetchServerVersion()
			if err != nil {
				ts.cfg.Logger.Warn("get version failed", zap.Error(err))
			}
		}
		err = ts.k8sClient.CheckHealth()
		if err == nil {
			break
		}
		ts.cfg.Logger.Warn("health check failed", zap.Error(err))
		ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("health check failed (%v)", err))
	}

	ts.cfg.Logger.Info("health check success")
	return err
}

func (ts *tester) Delete() error {
	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))

	var errs []string

	if err := ts.deleteEKS(); err != nil {
		errs = append(errs, err.Error())
		ts.cfg.Logger.Warn("EKS cluster delete failed -- please try delete again to clean up other resources!!!", zap.Error(err))
	} else {
		// only proceed when the cluster delete succeeded
		// otherwise, it's not safe to delete non-EKS resources
		// (e.g. delete CMK can fail other dependent components
		// under the same account)
		if err := ts.deleteEncryption(); err != nil {
			errs = append(errs, err.Error())
		}
		if err := ts.deleteRole(); err != nil {
			errs = append(errs, err.Error())
		}
		if err := ts.deleteVPC(); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.Sync()
	return nil
}

func (ts *tester) updateClusterStatusV1(v1 wait.ClusterStatus, desired string) {
	if v1.Cluster == nil {
		if v1.Error != nil {
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed with error %v", v1.Error))
			ts.cfg.EKSConfig.Status.Up = false
		} else {
			ts.cfg.EKSConfig.RecordStatus(eksconfig.ClusterStatusDELETEDORNOTEXIST)
		}
		return
	}

	ts.cfg.EKSConfig.Status.ClusterARN = aws_v2.ToString(v1.Cluster.Arn)
	ts.cfg.EKSConfig.RecordStatus(aws_v2.ToString(v1.Cluster.Status))

	if desired != eksconfig.ClusterStatusDELETEDORNOTEXIST &&
		ts.cfg.EKSConfig.Status.ClusterStatusCurrent != eksconfig.ClusterStatusDELETEDORNOTEXIST {

		if v1.Cluster.Endpoint != nil {
			ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint = aws_v2.ToString(v1.Cluster.Endpoint)
		}

		if v1.Cluster.Identity != nil &&
			v1.Cluster.Identity.Oidc != nil &&
			v1.Cluster.Identity.Oidc.Issuer != nil {
			ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL = aws_v2.ToString(v1.Cluster.Identity.Oidc.Issuer)
			u, err := url.Parse(ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL)
			if err != nil {
				ts.cfg.Logger.Warn(
					"failed to parse ClusterOIDCIssuerURL",
					zap.String("url", ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL),
					zap.Error(err),
				)
			}
			if u.Scheme != "https" {
				ts.cfg.Logger.Warn("invalid scheme", zap.String("scheme", u.Scheme))
			}
			if u.Port() == "" {
				ts.cfg.Logger.Info("updating host with port :443", zap.String("host", u.Host))
				u.Host += ":443"
			}
			ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL = u.String()
			ts.cfg.EKSConfig.Status.ClusterOIDCIssuerHostPath = u.Hostname() + u.Path
			ts.cfg.EKSConfig.Status.ClusterOIDCIssuerARN = fmt.Sprintf(
				"arn:aws:iam::%s:oidc-provider/%s",
				ts.cfg.EKSConfig.Status.AWSAccountID,
				ts.cfg.EKSConfig.Status.ClusterOIDCIssuerHostPath,
			)

			ts.cfg.Logger.Info("fetching OIDC CA thumbprint", zap.String("url", ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL))
			httpClient := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{},
					Proxy:           http.ProxyFromEnvironment,
				},
			}
			var resp *http.Response
			for i := 0; i < 5; i++ {
				resp, err = httpClient.Get(ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL)
				if err == nil {
					break
				}
				code := 0
				if resp != nil {
					code = resp.StatusCode
				}
				// TODO: parse response status code to decide retries?
				ts.cfg.Logger.Warn("failed to fetch OIDC CA thumbprint",
					zap.String("url", ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL),
					zap.Int("status-code", code),
					zap.Error(err),
				)
				time.Sleep(5 * time.Second)
			}
			if resp != nil && resp.Body != nil {
				defer resp.Body.Close()
			}
			if resp != nil && resp.TLS != nil {
				certs := len(resp.TLS.PeerCertificates)
				if certs >= 1 {
					root := resp.TLS.PeerCertificates[certs-1]
					ts.cfg.EKSConfig.Status.ClusterOIDCIssuerCAThumbprint = fmt.Sprintf("%x", sha1.Sum(root.Raw))
					ts.cfg.Logger.Info("fetched OIDC CA thumbprint")
				} else {
					ts.cfg.Logger.Warn("received empty TLS peer certs")
				}
			} else {
				ts.cfg.Logger.Warn("received empty HTTP empty TLS response")
			}
		}

		if v1.Cluster.CertificateAuthority != nil && v1.Cluster.CertificateAuthority.Data != nil {
			ts.cfg.EKSConfig.Status.ClusterCA = aws_v2.ToString(v1.Cluster.CertificateAuthority.Data)
		}
		d, err := base64.StdEncoding.DecodeString(ts.cfg.EKSConfig.Status.ClusterCA)
		if err != nil {
			ts.cfg.Logger.Warn("failed to decode cluster CA", zap.Error(err))
		}
		ts.cfg.EKSConfig.Status.ClusterCADecoded = string(d)

	} else {

		ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint = ""
		ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL = ""
		ts.cfg.EKSConfig.Status.ClusterOIDCIssuerHostPath = ""
		ts.cfg.EKSConfig.Status.ClusterOIDCIssuerARN = ""
		ts.cfg.EKSConfig.Status.ClusterOIDCIssuerCAThumbprint = ""
		ts.cfg.EKSConfig.Status.ClusterCA = ""
		ts.cfg.EKSConfig.Status.ClusterCADecoded = ""

	}

	ts.cfg.EKSConfig.Sync()
}

func (ts *tester) updateClusterStatusV2(v2 wait_v2.ClusterStatus, desired string) {
	if v2.Cluster == nil {
		if v2.Error != nil {
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed with error %v", v2.Error))
			ts.cfg.EKSConfig.Status.Up = false
		} else {
			ts.cfg.EKSConfig.RecordStatus(eksconfig.ClusterStatusDELETEDORNOTEXIST)
		}
		return
	}

	ts.cfg.EKSConfig.Status.ClusterARN = aws_v2.ToString(v2.Cluster.Arn)
	ts.cfg.EKSConfig.RecordStatus(fmt.Sprint(v2.Cluster.Status))

	if desired != eksconfig.ClusterStatusDELETEDORNOTEXIST &&
		ts.cfg.EKSConfig.Status.ClusterStatusCurrent != eksconfig.ClusterStatusDELETEDORNOTEXIST {

		if v2.Cluster.Endpoint != nil {
			ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint = aws_v2.ToString(v2.Cluster.Endpoint)
		}

		if v2.Cluster.Identity != nil &&
			v2.Cluster.Identity.Oidc != nil &&
			v2.Cluster.Identity.Oidc.Issuer != nil {
			ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL = aws_v2.ToString(v2.Cluster.Identity.Oidc.Issuer)
			u, err := url.Parse(ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL)
			if err != nil {
				ts.cfg.Logger.Warn(
					"failed to parse ClusterOIDCIssuerURL",
					zap.String("url", ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL),
					zap.Error(err),
				)
			}
			if u.Scheme != "https" {
				ts.cfg.Logger.Warn("invalid scheme", zap.String("scheme", u.Scheme))
			}
			if u.Port() == "" {
				ts.cfg.Logger.Info("updating host with port :443", zap.String("host", u.Host))
				u.Host += ":443"
			}
			ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL = u.String()
			ts.cfg.EKSConfig.Status.ClusterOIDCIssuerHostPath = u.Hostname() + u.Path
			ts.cfg.EKSConfig.Status.ClusterOIDCIssuerARN = fmt.Sprintf(
				"arn:aws:iam::%s:oidc-provider/%s",
				ts.cfg.EKSConfig.Status.AWSAccountID,
				ts.cfg.EKSConfig.Status.ClusterOIDCIssuerHostPath,
			)

			ts.cfg.Logger.Info("fetching OIDC CA thumbprint", zap.String("url", ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL))
			httpClient := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{},
					Proxy:           http.ProxyFromEnvironment,
				},
			}
			var resp *http.Response
			for i := 0; i < 5; i++ {
				resp, err = httpClient.Get(ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL)
				if err == nil {
					break
				}
				code := 0
				if resp != nil {
					code = resp.StatusCode
				}
				// TODO: parse response status code to decide retries?
				ts.cfg.Logger.Warn("failed to fetch OIDC CA thumbprint",
					zap.String("url", ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL),
					zap.Int("status-code", code),
					zap.Error(err),
				)
				time.Sleep(5 * time.Second)
			}
			if resp != nil && resp.Body != nil {
				defer resp.Body.Close()
			}
			if resp != nil && resp.TLS != nil {
				certs := len(resp.TLS.PeerCertificates)
				if certs >= 1 {
					root := resp.TLS.PeerCertificates[certs-1]
					ts.cfg.EKSConfig.Status.ClusterOIDCIssuerCAThumbprint = fmt.Sprintf("%x", sha1.Sum(root.Raw))
					ts.cfg.Logger.Info("fetched OIDC CA thumbprint")
				} else {
					ts.cfg.Logger.Warn("received empty TLS peer certs")
				}
			} else {
				ts.cfg.Logger.Warn("received empty HTTP empty TLS response")
			}
		}

		if v2.Cluster.CertificateAuthority != nil && v2.Cluster.CertificateAuthority.Data != nil {
			ts.cfg.EKSConfig.Status.ClusterCA = aws_v2.ToString(v2.Cluster.CertificateAuthority.Data)
		}
		d, err := base64.StdEncoding.DecodeString(ts.cfg.EKSConfig.Status.ClusterCA)
		if err != nil {
			ts.cfg.Logger.Warn("failed to decode cluster CA", zap.Error(err))
		}
		ts.cfg.EKSConfig.Status.ClusterCADecoded = string(d)

	} else {

		ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint = ""
		ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL = ""
		ts.cfg.EKSConfig.Status.ClusterOIDCIssuerHostPath = ""
		ts.cfg.EKSConfig.Status.ClusterOIDCIssuerARN = ""
		ts.cfg.EKSConfig.Status.ClusterOIDCIssuerCAThumbprint = ""
		ts.cfg.EKSConfig.Status.ClusterCA = ""
		ts.cfg.EKSConfig.Status.ClusterCADecoded = ""

	}

	ts.cfg.EKSConfig.Sync()
}

type kubeconfig struct {
	ClusterAPIServerEndpoint string
	ClusterCA                string
	AWSIAMAuthenticatorPath  string
	ClusterName              string
	AuthenticationAPIVersion string
}

const tmplKUBECONFIG = `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: {{ .ClusterAPIServerEndpoint }}
    certificate-authority-data: {{ .ClusterCA }}
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: aws
  name: aws
current-context: aws
preferences: {}
users:
- name: aws
  user:
    exec:
      apiVersion: {{ .AuthenticationAPIVersion }}
      command: {{ .AWSIAMAuthenticatorPath }}
      args:
      - token
      - -i
      - {{ .ClusterName }}
`

// https://docs.aws.amazon.com/cli/latest/reference/eks/update-kubeconfig.html
// https://docs.aws.amazon.com/eks/latest/userguide/create-kubeconfig.html
// "aws eks update-kubeconfig --name --role-arn --kubeconfig"
func (ts *tester) createClient() (cli k8s_client.EKS, err error) {
	fmt.Print(ts.cfg.EKSConfig.Colorize("\n\n[yellow]*********************************\n"))
	fmt.Printf(ts.cfg.EKSConfig.Colorize("[light_green]createClient [default](%q)\n"), ts.cfg.EKSConfig.ConfigPath)
	ts.cfg.EKSConfig.AuthenticationAPIVersion ="client.authentication.k8s.io/v1alpha1"

	if ts.cfg.EKSConfig.AWSIAMAuthenticatorPath != "" && ts.cfg.EKSConfig.AWSIAMAuthenticatorDownloadURL != "" {
		tpl := template.Must(template.New("tmplKUBECONFIG").Parse(tmplKUBECONFIG))
		buf := bytes.NewBuffer(nil)
		if err = tpl.Execute(buf, kubeconfig{
			ClusterAPIServerEndpoint: ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint,
			ClusterCA:                ts.cfg.EKSConfig.Status.ClusterCA,
			AWSIAMAuthenticatorPath:  ts.cfg.EKSConfig.AWSIAMAuthenticatorPath,
			ClusterName:              ts.cfg.EKSConfig.Name,
			AuthenticationAPIVersion: ts.cfg.EKSConfig.AuthenticationAPIVersion,
		}); err != nil {
			return nil, err
		}
		ts.cfg.Logger.Info("writing KUBECONFIG with aws-iam-authenticator", zap.String("kubeconfig-path", ts.cfg.EKSConfig.KubeConfigPath))
		if err = ioutil.WriteFile(ts.cfg.EKSConfig.KubeConfigPath, buf.Bytes(), 0777); err != nil {
			return nil, err
		}
		if err = aws_s3.Upload(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3.BucketName,
			path.Join(ts.cfg.EKSConfig.Name, "kubeconfig.yaml"),
			ts.cfg.EKSConfig.KubeConfigPath,
		); err != nil {
			return nil, err
		}
		ts.cfg.Logger.Info("wrote KUBECONFIG with aws-iam-authenticator", zap.String("kubeconfig-path", ts.cfg.EKSConfig.KubeConfigPath))
	} else {
		args := []string{
			ts.cfg.EKSConfig.AWSCLIPath,
			"eks",
			fmt.Sprintf("--region=%s", ts.cfg.EKSConfig.Region),
			"update-kubeconfig",
			fmt.Sprintf("--name=%s", ts.cfg.EKSConfig.Name),
			fmt.Sprintf("--kubeconfig=%s", ts.cfg.EKSConfig.KubeConfigPath),
			"--verbose",
		}
		if ts.cfg.EKSConfig.ResolverURL != "" {
			args = append(args, fmt.Sprintf("--endpoint=%s", ts.cfg.EKSConfig.ResolverURL))
		}
		cmd := strings.Join(args, " ")
		ts.cfg.Logger.Info("writing KUBECONFIG with 'aws eks update-kubeconfig'",
			zap.String("kubeconfig-path", ts.cfg.EKSConfig.KubeConfigPath),
			zap.String("cmd", cmd),
		)
		retryStart, waitDur := time.Now(), 3*time.Minute
		var output []byte
		for time.Since(retryStart) < waitDur {
			select {
			case <-ts.cfg.Stopc:
				return nil, errors.New("update-kubeconfig aborted")
			case <-time.After(5 * time.Second):
			}
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
			output, err = exec.New().CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
			cancel()
			out := string(output)
			fmt.Fprintf(ts.cfg.LogWriter, "\n'%s' output:\n\n%s\n\n", cmd, out)
			if err != nil {
				ts.cfg.Logger.Warn("'aws eks update-kubeconfig' failed", zap.Error(err))
				if !strings.Contains(out, "Cluster status not active") || !strings.Contains(err.Error(), "exit") {
					return nil, fmt.Errorf("'aws eks update-kubeconfig' failed (output %q, error %v)", out, err)
				}
				continue
			}
			ts.cfg.Logger.Info("'aws eks update-kubeconfig' success", zap.String("kubeconfig-path", ts.cfg.EKSConfig.KubeConfigPath))
			if err = aws_s3.Upload(
				ts.cfg.Logger,
				ts.cfg.S3API,
				ts.cfg.EKSConfig.S3.BucketName,
				path.Join(ts.cfg.EKSConfig.Name, "kubeconfig.yaml"),
				ts.cfg.EKSConfig.KubeConfigPath,
			); err != nil {
				return nil, err
			}
			break
		}
		if err != nil {
			ts.cfg.Logger.Warn("failed 'aws eks update-kubeconfig'", zap.Error(err))
			return nil, err
		}

		ts.cfg.Logger.Info("ran 'aws eks update-kubeconfig'")
		fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", cmd, strings.TrimSpace(string(output)))
	}

	ts.cfg.Logger.Info("creating k8s client")
	kcfg := &k8s_client.EKSConfig{
		Logger:                             ts.cfg.Logger,
		Region:                             ts.cfg.EKSConfig.Region,
		ClusterName:                        ts.cfg.EKSConfig.Name,
		KubeConfigPath:                     ts.cfg.EKSConfig.KubeConfigPath,
		KubectlPath:                        ts.cfg.EKSConfig.KubectlPath,
		ServerVersion:                      ts.cfg.EKSConfig.Version,
		EncryptionEnabled:                  ts.cfg.EKSConfig.Encryption.CMKARN != "",
		S3API:                              ts.cfg.S3API,
		S3BucketName:                       ts.cfg.EKSConfig.S3.BucketName,
		S3MetricsRawOutputDirKubeAPIServer: path.Join(ts.cfg.EKSConfig.Name, "metrics-kube-apiserver"),
		MetricsRawOutputDirKubeAPIServer:   filepath.Join(filepath.Dir(ts.cfg.EKSConfig.ConfigPath), ts.cfg.EKSConfig.Name+"-metrics-kube-apiserver"),
		Clients:                            ts.cfg.EKSConfig.Clients,
		ClientQPS:                          ts.cfg.EKSConfig.ClientQPS,
		ClientBurst:                        ts.cfg.EKSConfig.ClientBurst,
		ClientTimeout:                      ts.cfg.EKSConfig.ClientTimeout,
	}
	if ts.cfg.EKSConfig.IsEnabledAddOnClusterVersionUpgrade() {
		kcfg.UpgradeServerVersion = ts.cfg.EKSConfig.AddOnClusterVersionUpgrade.Version
	}
	if ts.cfg.EKSConfig.Status != nil {
		kcfg.ClusterAPIServerEndpoint = ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint
		kcfg.ClusterCADecoded = ts.cfg.EKSConfig.Status.ClusterCADecoded
	}
	cli, err = k8s_client.NewEKS(kcfg)
	if err != nil {
		ts.cfg.Logger.Warn("failed to create k8s client", zap.Error(err))
	} else {
		ts.cfg.Logger.Info("created k8s client")
	}
	return cli, err
}
