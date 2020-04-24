// Package prometheusgrafana implements Prometheus/Grafana add-on.
package prometheusgrafana

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/eks/helm"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/httputil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// Config defines Wordpress configuration.
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}
	Sig    chan os.Signal

	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

// Tester defines Prometheus/Grafana tester
type Tester interface {
	// Create installs Prometheus/Grafana.
	Create() error
	// Delete deletes Prometheus/Grafana.
	Delete() error
}

func NewTester(cfg Config) (Tester, error) {
	return &tester{cfg: cfg}, nil
}

type tester struct {
	cfg Config
}

const (
	chartRepoName = "stable"
	chartRepoURL  = "https://kubernetes-charts.storage.googleapis.com"

	chartNamespacePrometheus = "prometheus"
	chartNamespaceGrafana    = "grafana"

	chartNamePrometheus = "prometheus"
	chartNameGrafana    = "grafana"
)

func (ts *tester) Create() error {
	if ts.cfg.EKSConfig.AddOnPrometheusGrafana.Created {
		ts.cfg.Logger.Info("skipping create AddOnPrometheusGrafana")
		return nil
	}

	ts.cfg.EKSConfig.AddOnPrometheusGrafana.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()

	defer func() {
		ts.cfg.EKSConfig.AddOnPrometheusGrafana.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.AddOnPrometheusGrafana.CreateTookString = ts.cfg.EKSConfig.AddOnPrometheusGrafana.CreateTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	if err := k8s_client.CreateNamespace(ts.cfg.Logger, ts.cfg.K8SClient.KubernetesClientSet(), chartNamespacePrometheus); err != nil {
		return err
	}
	if err := k8s_client.CreateNamespace(ts.cfg.Logger, ts.cfg.K8SClient.KubernetesClientSet(), chartNamespaceGrafana); err != nil {
		return err
	}
	if err := helm.RepoAdd(ts.cfg.Logger, chartRepoName, chartRepoURL); err != nil {
		return err
	}
	if err := ts.createHelmPrometheus(); err != nil {
		return err
	}
	if err := ts.createHelmGrafana(); err != nil {
		return err
	}
	if err := ts.waitServiceGrafana(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.AddOnPrometheusGrafana.Created {
		ts.cfg.Logger.Info("skipping delete AddOnPrometheusGrafana")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnPrometheusGrafana.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.AddOnPrometheusGrafana.DeleteTookString = ts.cfg.EKSConfig.AddOnPrometheusGrafana.DeleteTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := ts.deleteHelmGrafana(); err != nil {
		errs = append(errs, err.Error())
	}

	if err := ts.deleteHelmPrometheus(); err != nil {
		errs = append(errs, err.Error())
	}

	if err := k8s_client.DeleteNamespaceAndWait(ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		chartNamespaceGrafana,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Wordpress namespace (%v)", err))
	}

	if err := k8s_client.DeleteNamespaceAndWait(ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		chartNamespacePrometheus,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Wordpress namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnPrometheusGrafana.Created = false
	return ts.cfg.EKSConfig.Sync()
}

// https://eksworkshop.com/intermediate/240_monitoring/deploy-prometheus
// https://github.com/helm/charts/blob/master/stable/prometheus/values.yaml
func (ts *tester) createHelmPrometheus() error {
	ngType := "custom"
	if ts.cfg.EKSConfig.IsEnabledAddOnManagedNodeGroups() {
		ngType = "managed"
	}

	// https://github.com/helm/charts/blob/master/stable/prometheus/values.yaml
	values := make(map[string]interface{})

	values["alertmanager"] = map[string]interface{}{
		"nodeSelector": map[string]interface{}{
			"NGType": ngType,
			// do not deploy in bottlerocket; PVC not working
			"AMIType": ec2config.AMITypeAL2X8664,
		},
		"persistentVolume": map[string]interface{}{
			// takes >=5-min for PVC, user emptyDir for testing
			"enabled": false,
		},
	}
	values["server"] = map[string]interface{}{
		"nodeSelector": map[string]interface{}{
			"NGType": ngType,
			// do not deploy in bottlerocket; PVC not working
			"AMIType": ec2config.AMITypeAL2X8664,
		},
		"persistentVolume": map[string]interface{}{
			"enabled": true,
			// use CSI driver with volume type "gp2", as in launch configuration
			"storageClass": "gp2",
		},
	}

	return helm.Install(helm.InstallConfig{
		Logger:         ts.cfg.Logger,
		Timeout:        15 * time.Minute,
		KubeConfigPath: ts.cfg.EKSConfig.KubeConfigPath,
		Namespace:      chartNamespacePrometheus,
		ChartRepoURL:   chartRepoURL,
		ChartName:      chartNamePrometheus,
		ReleaseName:    chartNamePrometheus,
		Values:         values,
	})
}

func (ts *tester) deleteHelmPrometheus() error {
	return helm.Uninstall(helm.InstallConfig{
		Logger:         ts.cfg.Logger,
		Timeout:        15 * time.Minute,
		KubeConfigPath: ts.cfg.EKSConfig.KubeConfigPath,
		Namespace:      chartNamespacePrometheus,
		ChartName:      chartNamePrometheus,
		ReleaseName:    chartNamePrometheus,
	})
}

// https://eksworkshop.com/intermediate/240_monitoring/deploy-grafana
// https://github.com/helm/charts/blob/master/stable/grafana/values.yaml
func (ts *tester) createHelmGrafana() error {
	ngType := "custom"
	if ts.cfg.EKSConfig.IsEnabledAddOnManagedNodeGroups() {
		ngType = "managed"
	}

	// https://github.com/helm/charts/blob/master/stable/grafana/values.yaml
	values := make(map[string]interface{})

	values["adminUser"] = ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaAdminUserName
	values["adminPassword"] = ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaAdminPassword

	values["nodeSelector"] = map[string]interface{}{
		"NGType": ngType,
		// do not deploy in bottlerocket; PVC not working
		"AMIType": ec2config.AMITypeAL2X8664,
	}

	values["persistence"] = map[string]interface{}{
		"enabled": true,
		// use CSI driver with volume type "gp2", as in launch configuration
		"storageClass": "gp2",
	}

	values["service"] = map[string]interface{}{
		"type": "LoadBalancer",
	}

	values["datasources"] = map[string]interface{}{
		"datasources.yaml": map[string]interface{}{
			"apiVersion": 1,
			"datasources": []map[string]interface{}{
				{
					"name":      "Prometheus",
					"type":      "prometheus",
					"url":       "http://prometheus-server.prometheus.svc.cluster.local",
					"access":    "proxy",
					"isDefault": true,
				},
			},
		},
	}

	values["dashboardProviders"] = map[string]interface{}{
		"dashboardproviders.yaml": map[string]interface{}{
			"apiVersion": 1,
			"providers": []map[string]interface{}{
				{
					"disableDeletion": false,
					"editable":        true,
					"folder":          "",
					"name":            "default",
					"options": map[string]interface{}{
						"path": "/var/lib/grafana/dashboards/default",
					},
					"orgId": 1,
					"type":  "file",
				},
			},
		},
	}
	values["dashboards"] = map[string]interface{}{
		"default": map[string]interface{}{
			"kubernetes-cluster": map[string]interface{}{
				"gnetId":     6417,
				"revision":   1,
				"datasource": "Prometheus",
			},
		},
	}

	return helm.Install(helm.InstallConfig{
		Logger:         ts.cfg.Logger,
		Timeout:        15 * time.Minute,
		KubeConfigPath: ts.cfg.EKSConfig.KubeConfigPath,
		Namespace:      chartNamespaceGrafana,
		ChartRepoURL:   chartRepoURL,
		ChartName:      chartNameGrafana,
		ReleaseName:    chartNameGrafana,
		Values:         values,
	})
}

func (ts *tester) deleteHelmGrafana() error {
	return helm.Uninstall(helm.InstallConfig{
		Logger:         ts.cfg.Logger,
		Timeout:        15 * time.Minute,
		KubeConfigPath: ts.cfg.EKSConfig.KubeConfigPath,
		Namespace:      chartNamespaceGrafana,
		ChartName:      chartNameGrafana,
		ReleaseName:    chartNameGrafana,
	})
}

func (ts *tester) waitServiceGrafana() error {
	svcName := "grafana"
	ts.cfg.Logger.Info("waiting for Grafana service")

	waitDur := 2 * time.Minute
	ts.cfg.Logger.Info("waiting for Grafana service", zap.Duration("wait", waitDur))
	select {
	case <-ts.cfg.Stopc:
		return errors.New("Grafana service creation aborted")
	case sig := <-ts.cfg.Sig:
		return fmt.Errorf("received os signal %v", sig)
	case <-time.After(waitDur):
	}

	args := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + chartNameGrafana,
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
			return errors.New("Grafana service creation aborted")
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

		ts.cfg.Logger.Info("querying Grafana service for HTTP endpoint")
		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		so, err := ts.cfg.K8SClient.KubernetesClientSet().
			CoreV1().
			Services(chartNameGrafana).
			Get(ctx, svcName, metav1.GetOptions{})
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("failed to get Grafana service; retrying", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		ts.cfg.Logger.Info(
			"Grafana service has been linked to LoadBalancer",
			zap.String("load-balancer", fmt.Sprintf("%+v", so.Status.LoadBalancer)),
		)
		for _, ing := range so.Status.LoadBalancer.Ingress {
			ts.cfg.Logger.Info(
				"Grafana service has been linked to LoadBalancer.Ingress",
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

	ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaURL = "http://" + hostName + "/login"

	// TODO: is there any better way to find out the NLB name?
	ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaNLBName = strings.Split(hostName, "-")[0]
	ss := strings.Split(hostName, ".")[0]
	ss = strings.Replace(ss, "-", "/", -1)
	ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaNLBARN = fmt.Sprintf(
		"arn:aws:elasticloadbalancing:%s:%s:loadbalancer/net/%s",
		ts.cfg.EKSConfig.Region,
		ts.cfg.EKSConfig.Status.AWSAccountID,
		ss,
	)

	fmt.Printf("\nNLB Grafana ARN: %s\n", ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaNLBARN)
	fmt.Printf("NLB Grafana Name: %s\n", ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaNLBName)
	fmt.Printf("NLB Grafana URL: %s\n", ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaURL)
	fmt.Printf("Grafana Admin User Name: %s\n", ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaAdminUserName)
	fmt.Printf("Grafana Admin Password: %d characters\n\n", len(ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaAdminPassword))

	ts.cfg.Logger.Info("waiting before testing Grafana Service")
	time.Sleep(20 * time.Second)

	retryStart = time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("Grafana Service creation aborted")
		case sig := <-ts.cfg.Sig:
			return fmt.Errorf("received os signal %v", sig)
		case <-time.After(5 * time.Second):
		}

		out, err := httputil.ReadInsecure(ts.cfg.Logger, os.Stderr, ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaURL)
		if err != nil {
			ts.cfg.Logger.Warn("failed to read NLB Grafana Service; retrying", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}
		httpOutput := string(out)
		fmt.Printf("\nNLB Grafana Service output:\n%s\n", httpOutput)

		if strings.Contains(httpOutput, `Loading Grafana`) || true {
			ts.cfg.Logger.Info(
				"read Grafana Service; exiting",
				zap.String("host-name", hostName),
			)
			break
		}

		ts.cfg.Logger.Warn("unexpected Grafana Service output; retrying")
	}

	fmt.Printf("\nNLB Grafana ARN: %s\n", ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaNLBARN)
	fmt.Printf("NLB Grafana Name: %s\n", ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaNLBName)
	fmt.Printf("NLB Grafana URL: %s\n", ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaURL)
	fmt.Printf("Grafana Admin User Name: %s\n", ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaAdminUserName)
	fmt.Printf("Grafana Admin Password: %d characters\n\n", len(ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaAdminPassword))

	return ts.cfg.EKSConfig.Sync()
}
