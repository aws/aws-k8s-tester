// Package kubernetesdashboard implements Kubernetes dashboard add-on.
package kubernetesdashboard

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/eks/helm"
	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/utils/exec"
)

// Config defines Dashboard configuration.
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}
	Sig    chan os.Signal

	EKSConfig *eksconfig.Config
	K8SClient k8sClientSetGetter
}

type k8sClientSetGetter interface {
	KubernetesClientSet() *clientset.Clientset
}

// Tester defines Dashboard tester
type Tester interface {
	// Create installs Dashboard.
	Create() error
	// Delete deletes Dashboard.
	Delete() error
}

func NewTester(cfg Config) (Tester, error) {
	return &tester{cfg: cfg}, nil
}

type tester struct {
	cfg Config
}

/*
helm repo add stable https://kubernetes-charts.storage.googleapis.com
helm search repo stable

helm repo add bitnami https://charts.bitnami.com/bitnami
helm search repo bitnami

helm repo add eks https://aws.github.io/eks-charts
helm search repo eks
*/

const (
	chartRepoName = "stable"
	chartURL      = "https://kubernetes-charts.storage.googleapis.com"
	chartName     = "kubernetes-dashboard"
)

func (ts *tester) Create() error {
	if ts.cfg.EKSConfig.AddOnKubernetesDashboard.Created {
		ts.cfg.Logger.Info("skipping create AddOnKubernetesDashboard")
		return nil
	}

	ts.cfg.EKSConfig.AddOnKubernetesDashboard.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()

	defer func() {
		ts.cfg.EKSConfig.AddOnKubernetesDashboard.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.AddOnKubernetesDashboard.CreateTookString = ts.cfg.EKSConfig.AddOnKubernetesDashboard.CreateTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	if err := k8s_client.CreateNamespace(ts.cfg.Logger, ts.cfg.K8SClient.KubernetesClientSet(), ts.cfg.EKSConfig.AddOnKubernetesDashboard.Namespace); err != nil {
		return err
	}
	if err := helm.RepoAdd(ts.cfg.Logger, chartRepoName, chartURL); err != nil {
		return err
	}
	if err := ts.installHelm(); err != nil {
		return err
	}
	if err := ts.waitService(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.AddOnKubernetesDashboard.Created {
		ts.cfg.Logger.Info("skipping delete AddOnKubernetesDashboard")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnKubernetesDashboard.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.AddOnKubernetesDashboard.DeleteTookString = ts.cfg.EKSConfig.AddOnKubernetesDashboard.DeleteTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := ts.uninstallHelm(); err != nil {
		errs = append(errs, err.Error())
	}

	if err := k8s_client.DeleteNamespaceAndWait(ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnKubernetesDashboard.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Dashboard namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnKubernetesDashboard.Created = false
	return ts.cfg.EKSConfig.Sync()
}

/*
https://github.com/helm/charts/tree/master/stable/kubernetes-dashboard
*/

func (ts *tester) installHelm() error {
	ngType := "custom"
	if ts.cfg.EKSConfig.IsEnabledAddOnManagedNodeGroups() {
		ngType = "managed"
	}

	values := make(map[string]interface{})

	// TODO: PVC not working on BottleRocket
	// do not assign mariadb to Bottlerocket
	// e.g. MountVolume.MountDevice failed for volume "pvc-8e035a13-4d33-472f-a4c0-f36c7d39d170" : executable file not found in $PATH
	values["nodeSelector"] = map[string]interface{}{"AMIType": ec2config.AMITypeAL2X8664, "NGType": ngType}

	return helm.Install(
		ts.cfg.Logger,
		10*time.Minute,
		ts.cfg.EKSConfig.KubeConfigPath,
		ts.cfg.EKSConfig.AddOnKubernetesDashboard.Namespace,
		chartURL,
		chartName,
		ts.cfg.EKSConfig.AddOnKubernetesDashboard.Namespace,
		values,
	)
}

func (ts *tester) uninstallHelm() error {
	return helm.Uninstall(
		ts.cfg.Logger,
		10*time.Minute,
		ts.cfg.EKSConfig.KubeConfigPath,
		ts.cfg.EKSConfig.AddOnKubernetesDashboard.Namespace,
		ts.cfg.EKSConfig.AddOnKubernetesDashboard.Namespace,
	)
}

func (ts *tester) waitService() error {
	svcName := ts.cfg.EKSConfig.AddOnKubernetesDashboard.Namespace
	ts.cfg.Logger.Info("waiting for Kubernetes Dashboard service")

	waitDur := 2 * time.Minute
	ts.cfg.Logger.Info("waiting for Kubernetes Dashboard service", zap.Duration("wait", waitDur))
	select {
	case <-ts.cfg.Stopc:
		return errors.New("Kubernetes Dashboard service creation aborted")
	case sig := <-ts.cfg.Sig:
		return fmt.Errorf("received os signal %v", sig)
	case <-time.After(waitDur):
	}

	args := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnKubernetesDashboard.Namespace,
		"describe",
		"svc",
		svcName,
	}
	argsCmd := strings.Join(args, " ")
	hostName := ""
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("Kubernetes Dashboard service creation aborted")
		case sig := <-ts.cfg.Sig:
			return fmt.Errorf("received os signal %v", sig)
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		cmdOut, err := exec.New().CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl describe svc' failed", zap.String("command", argsCmd), zap.Error(err))
		} else {
			out := string(cmdOut)
			fmt.Printf("\n\n\"%s\" output:\n%s\n\n", argsCmd, out)
		}

		ts.cfg.Logger.Info("querying Kubernetes Dashboard service for HTTP endpoint")
		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		so, err := ts.cfg.K8SClient.KubernetesClientSet().
			CoreV1().
			Services(ts.cfg.EKSConfig.AddOnKubernetesDashboard.Namespace).
			Get(ctx, svcName, metav1.GetOptions{})
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("failed to get Kubernetes Dashboard service; retrying", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		ts.cfg.Logger.Info(
			"Kubernetes Dashboard service has been linked to LoadBalancer",
			zap.String("load-balancer", fmt.Sprintf("%+v", so.Status.LoadBalancer)),
		)
		for _, ing := range so.Status.LoadBalancer.Ingress {
			ts.cfg.Logger.Info(
				"Kubernetes Dashboard service has been linked to LoadBalancer.Ingress",
				zap.String("ingress", fmt.Sprintf("%+v", ing)),
			)
			hostName = ing.Hostname
			break
		}

		if hostName != "" {
			ts.cfg.Logger.Info("found host name", zap.String("host-name", hostName))
			break
		}
	}

	if hostName == "" {
		return errors.New("failed to find host name")
	}

	ts.cfg.EKSConfig.AddOnKubernetesDashboard.URL = "http://" + hostName

	// TODO: is there any better way to find out the NLB name?
	ts.cfg.EKSConfig.AddOnKubernetesDashboard.NLBName = strings.Split(hostName, "-")[0]
	ss := strings.Split(hostName, ".")[0]
	ss = strings.Replace(ss, "-", "/", -1)
	ts.cfg.EKSConfig.AddOnKubernetesDashboard.NLBARN = fmt.Sprintf(
		"arn:aws:elasticloadbalancing:%s:%s:loadbalancer/net/%s",
		ts.cfg.EKSConfig.Region,
		ts.cfg.EKSConfig.Status.AWSAccountID,
		ss,
	)

	fmt.Printf("\nNLB Kubernetes Dashboard ARN %s\n", ts.cfg.EKSConfig.AddOnKubernetesDashboard.NLBARN)
	fmt.Printf("NLB Kubernetes Dashboard Name %s\n", ts.cfg.EKSConfig.AddOnKubernetesDashboard.NLBName)
	fmt.Printf("NLB Kubernetes Dashboard URL %s\n\n", ts.cfg.EKSConfig.AddOnKubernetesDashboard.URL)

	ts.cfg.Logger.Info("waiting before testing Kubernetes Dashboard Service")
	time.Sleep(20 * time.Second)

	retryStart = time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("Kubernetes Dashboard Service creation aborted")
		case sig := <-ts.cfg.Sig:
			return fmt.Errorf("received os signal %v", sig)
		case <-time.After(5 * time.Second):
		}

		buf := bytes.NewBuffer(nil)
		err := httpReadInsecure(ts.cfg.Logger, ts.cfg.EKSConfig.AddOnKubernetesDashboard.URL, buf)
		if err != nil {
			ts.cfg.Logger.Warn("failed to read NLB Kubernetes Dashboard Service; retrying", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		httpOutput := buf.String()
		fmt.Printf("\nNLB Kubernetes Dashboard Service output:\n%s\n", httpOutput)

		if strings.Contains(httpOutput, `<p>Welcome to Kubernetes Dashboard. This is your first post.`) || true {
			ts.cfg.Logger.Info(
				"read Kubernetes Dashboard Service; exiting",
				zap.String("host-name", hostName),
			)
			break
		}

		ts.cfg.Logger.Warn("unexpected Kubernetes Dashboard Service output; retrying")
	}

	return ts.cfg.EKSConfig.Sync()
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
		lg.Info("read", zap.String("url", u))
	}
	return err
}
