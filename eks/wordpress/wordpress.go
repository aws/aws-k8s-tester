// Package wordpress implements wordpress add-on.
package wordpress

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/eks/helm"
	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/httputil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// Config defines Wordpress configuration.
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}

	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

func New(cfg Config) eks_tester.Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	return &tester{cfg: cfg}
}

type tester struct {
	cfg Config
}

const (
	chartRepoName = "bitnami"
	chartRepoURL  = "https://charts.bitnami.com/bitnami"
	chartName     = "wordpress"
)

func (ts *tester) Create() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnWordpress() {
		ts.cfg.Logger.Info("skipping create AddOnWordpress")
		return nil
	}
	if ts.cfg.EKSConfig.AddOnWordpress.Created {
		ts.cfg.Logger.Info("skipping create AddOnWordpress")
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	ts.cfg.EKSConfig.AddOnWordpress.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnWordpress.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if err := k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnWordpress.Namespace,
	); err != nil {
		return err
	}
	if err := helm.RepoAdd(ts.cfg.Logger, chartRepoName, chartRepoURL); err != nil {
		return err
	}
	if err := ts.createHelmWordpress(); err != nil {
		return err
	}
	if err := ts.waitService(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnWordpress() {
		ts.cfg.Logger.Info("skipping delete AddOnWordpress")
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnWordpress.Created {
		ts.cfg.Logger.Info("skipping delete AddOnWordpress")
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnWordpress.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := ts.deleteHelmWordpress(); err != nil {
		errs = append(errs, err.Error())
	}

	if err := k8s_client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnWordpress.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Wordpress namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnWordpress.Created = false
	return ts.cfg.EKSConfig.Sync()
}

// https://github.com/helm/charts/blob/master/stable/wordpress/values.yaml
// https://github.com/helm/charts/blob/master/stable/mariadb/values.yaml
func (ts *tester) createHelmWordpress() error {
	ngType := "managed"
	if ts.cfg.EKSConfig.IsEnabledAddOnNodeGroups() {
		// TODO: test in MNG
		ngType = "custom"
	}

	// https://github.com/helm/charts/blob/master/stable/wordpress/values.yaml
	values := map[string]interface{}{
		"nodeSelector": map[string]interface{}{
			// do not deploy in bottlerocket; PVC not working
			"AMIType": ec2config.AMITypeAL2X8664,
			"NGType":  ngType,
		},
		"wordpressUsername": ts.cfg.EKSConfig.AddOnWordpress.UserName,
		"wordpressPassword": ts.cfg.EKSConfig.AddOnWordpress.Password,
		"persistence": map[string]interface{}{
			"enabled": true,
			// use CSI driver with volume type "gp2", as in launch configuration
			"storageClassName": "gp2",
		},
		// https://github.com/helm/charts/blob/master/stable/mariadb/values.yaml
		"mariadb": map[string]interface{}{
			"enabled": true,
			"rootUser": map[string]interface{}{
				"password":      ts.cfg.EKSConfig.AddOnWordpress.Password,
				"forcePassword": false,
			},
			"db": map[string]interface{}{
				"name":     "wordpress",
				"user":     ts.cfg.EKSConfig.AddOnWordpress.UserName,
				"password": ts.cfg.EKSConfig.AddOnWordpress.Password,
			},
			"master": map[string]interface{}{
				"nodeSelector": map[string]interface{}{
					// do not deploy in bottlerocket; PVC not working
					"AMIType": ec2config.AMITypeAL2X8664,
					"NGType":  ngType,
				},
				"persistence": map[string]interface{}{
					"enabled": true,
					// use CSI driver with volume type "gp2", as in launch configuration
					"storageClassName": "gp2",
				},
			},
			"slave": map[string]interface{}{
				"nodeSelector": map[string]interface{}{
					// do not deploy in bottlerocket; PVC not working
					"AMIType": ec2config.AMITypeAL2X8664,
					"NGType":  ngType,
				},
			},
		},
	}

	return helm.Install(helm.InstallConfig{
		Logger:         ts.cfg.Logger,
		Stopc:          ts.cfg.Stopc,
		Timeout:        15 * time.Minute,
		KubeConfigPath: ts.cfg.EKSConfig.KubeConfigPath,
		Namespace:      ts.cfg.EKSConfig.AddOnWordpress.Namespace,
		ChartRepoURL:   chartRepoURL,
		ChartName:      chartName,
		ReleaseName:    chartName,
		Values:         values,
		QueryFunc:      nil,
		QueryInterval:  30 * time.Second,
	})
}

func (ts *tester) deleteHelmWordpress() error {
	return helm.Uninstall(helm.InstallConfig{
		Logger:         ts.cfg.Logger,
		Timeout:        15 * time.Minute,
		KubeConfigPath: ts.cfg.EKSConfig.KubeConfigPath,
		Namespace:      ts.cfg.EKSConfig.AddOnWordpress.Namespace,
		ChartName:      chartName,
		ReleaseName:    chartName,
	})
}

func (ts *tester) waitService() error {
	svcName := "wordpress"
	ts.cfg.Logger.Info("waiting for WordPress service")

	waitDur := 2 * time.Minute
	ts.cfg.Logger.Info("waiting for WordPress service", zap.Duration("wait", waitDur))
	select {
	case <-ts.cfg.Stopc:
		return errors.New("WordPress service creation aborted")
	case <-time.After(waitDur):
	}

	args := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnWordpress.Namespace,
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
			return errors.New("WordPress service creation aborted")
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

		ts.cfg.Logger.Info("querying WordPress service for HTTP endpoint")
		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		so, err := ts.cfg.K8SClient.KubernetesClientSet().
			CoreV1().
			Services(ts.cfg.EKSConfig.AddOnWordpress.Namespace).
			Get(ctx, svcName, metav1.GetOptions{})
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("failed to get WordPress service; retrying", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		ts.cfg.Logger.Info(
			"WordPress service has been linked to LoadBalancer",
			zap.String("load-balancer", fmt.Sprintf("%+v", so.Status.LoadBalancer)),
		)
		for _, ing := range so.Status.LoadBalancer.Ingress {
			ts.cfg.Logger.Info(
				"WordPress service has been linked to LoadBalancer.Ingress",
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

	ts.cfg.EKSConfig.AddOnWordpress.URL = "http://" + hostName

	// TODO: is there any better way to find out the NLB name?
	ts.cfg.EKSConfig.AddOnWordpress.NLBName = strings.Split(hostName, "-")[0]
	ss := strings.Split(hostName, ".")[0]
	ss = strings.Replace(ss, "-", "/", -1)
	ts.cfg.EKSConfig.AddOnWordpress.NLBARN = fmt.Sprintf(
		"arn:aws:elasticloadbalancing:%s:%s:loadbalancer/net/%s",
		ts.cfg.EKSConfig.Region,
		ts.cfg.EKSConfig.Status.AWSAccountID,
		ss,
	)

	fmt.Printf("\nNLB WordPress ARN: %s\n", ts.cfg.EKSConfig.AddOnWordpress.NLBARN)
	fmt.Printf("NLB WordPress Name: %s\n", ts.cfg.EKSConfig.AddOnWordpress.NLBName)
	fmt.Printf("NLB WordPress URL: %s\n", ts.cfg.EKSConfig.AddOnWordpress.URL)
	fmt.Printf("WordPress UserName: %s\n", ts.cfg.EKSConfig.AddOnWordpress.UserName)
	fmt.Printf("WordPress Password: %d characters\n\n", len(ts.cfg.EKSConfig.AddOnWordpress.Password))

	ts.cfg.Logger.Info("waiting before testing WordPress Service")
	time.Sleep(20 * time.Second)

	retryStart = time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("WordPress Service creation aborted")
		case <-time.After(5 * time.Second):
		}

		out, err := httputil.ReadInsecure(ts.cfg.Logger, os.Stderr, ts.cfg.EKSConfig.AddOnWordpress.URL)
		if err != nil {
			ts.cfg.Logger.Warn("failed to read NLB WordPress Service; retrying", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}
		httpOutput := string(out)
		fmt.Printf("\nNLB WordPress Service output:\n%s\n", httpOutput)

		if strings.Contains(httpOutput, `<p>Welcome to WordPress. This is your first post`) {
			ts.cfg.Logger.Info(
				"read WordPress Service; exiting",
				zap.String("host-name", hostName),
			)
			break
		}

		ts.cfg.Logger.Warn("unexpected WordPress Service output; retrying")
	}

	fmt.Printf("\nNLB WordPress ARN: %s\n", ts.cfg.EKSConfig.AddOnWordpress.NLBARN)
	fmt.Printf("NLB WordPress Name: %s\n", ts.cfg.EKSConfig.AddOnWordpress.NLBName)
	fmt.Printf("NLB WordPress URL: %s\n", ts.cfg.EKSConfig.AddOnWordpress.URL)
	fmt.Printf("WordPress UserName: %s\n", ts.cfg.EKSConfig.AddOnWordpress.UserName)
	fmt.Printf("WordPress Password: %d characters\n\n", len(ts.cfg.EKSConfig.AddOnWordpress.Password))

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) AggregateResults() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnWordpress() {
		ts.cfg.Logger.Info("skipping aggregate AddOnWordpress")
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnWordpress.Created {
		ts.cfg.Logger.Info("skipping aggregate AddOnWordpress")
		return nil
	}

	ts.cfg.Logger.Info("starting tester.AggregateResults", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	return nil
}
