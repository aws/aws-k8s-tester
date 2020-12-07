// Package prometheusgrafana implements Prometheus/Grafana add-on.
package prometheusgrafana

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/eks/helm"
	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/aws/elb"
	"github.com/aws/aws-k8s-tester/pkg/httputil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
	"go.uber.org/zap"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// Config defines Wordpress configuration.
type Config struct {
	Logger    *zap.Logger
	LogWriter io.Writer
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	ELB2API   elbv2iface.ELBV2API
}

var pkgName = reflect.TypeOf(tester{}).PkgPath()

func (ts *tester) Name() string { return pkgName }

func New(cfg Config) eks_tester.Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", pkgName))
	return &tester{cfg: cfg}
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
	if !ts.cfg.EKSConfig.IsEnabledAddOnPrometheusGrafana() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnPrometheusGrafana.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.cfg.EKSConfig.AddOnPrometheusGrafana.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnPrometheusGrafana.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if err := k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		chartNamespacePrometheus); err != nil {
		return err
	}
	if err := k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		chartNamespaceGrafana); err != nil {
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

	ts.cfg.EKSConfig.Sync()
	return nil
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnPrometheusGrafana() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnPrometheusGrafana.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnPrometheusGrafana.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := ts.deleteHelmGrafana(); err != nil {
		errs = append(errs, err.Error())
	}

	time.Sleep(15 * time.Second)

	if err := ts.deleteHelmPrometheus(); err != nil {
		errs = append(errs, err.Error())
	}

	if err := ts.deleteGrafanaService(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Grafana Service (%v)", err))
	}
	ts.cfg.Logger.Info("wait for 3-minute after deleting Service")
	time.Sleep(3 * time.Minute)

	/*
	   # NLB tags
	   kubernetes.io/service-name
	   leegyuho-test-prod-nlb-hello-world/hello-world-service

	   kubernetes.io/cluster/leegyuho-test-prod
	   owned
	*/
	if err := elb.DeleteELBv2(
		ts.cfg.Logger,
		ts.cfg.ELB2API,
		ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaNLBARN,
		ts.cfg.EKSConfig.Parameters.VPCID,
		map[string]string{
			"kubernetes.io/cluster/" + ts.cfg.EKSConfig.Name: "owned",
			"kubernetes.io/service-name":                     "grafana/" + grafanaServiceName,
		},
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Grafana (%v)", err))
	}

	if err := k8s_client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		chartNamespaceGrafana,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
		k8s_client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Grafana namespace (%v)", err))
	}

	if err := k8s_client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		chartNamespacePrometheus,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
		k8s_client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Prometheus namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnPrometheusGrafana.Created = false
	ts.cfg.EKSConfig.Sync()
	return nil
}

// https://eksworkshop.com/intermediate/240_monitoring/deploy-prometheus
// https://github.com/helm/charts/blob/master/stable/prometheus/values.yaml
func (ts *tester) createHelmPrometheus() error {
	ngType := "custom"
	if ts.cfg.EKSConfig.IsEnabledAddOnManagedNodeGroups() {
		ngType = "managed"
	}

	// https://github.com/helm/charts/blob/master/stable/prometheus/values.yaml
	values := map[string]interface{}{
		"alertmanager": map[string]interface{}{
			"nodeSelector": map[string]interface{}{
				// do not deploy in bottlerocket; PVC not working
				"AMIType": ec2config.AMITypeAL2X8664,
				"NGType":  ngType,
			},
			"persistentVolume": map[string]interface{}{
				// takes >=5-min for PVC, user emptyDir for testing
				"enabled": false,
			},
		},
		"server": map[string]interface{}{
			"nodeSelector": map[string]interface{}{
				// do not deploy in bottlerocket; PVC not working
				"AMIType": ec2config.AMITypeAL2X8664,
				"NGType":  ngType,
			},
			"persistentVolume": map[string]interface{}{
				"enabled": true,
				// use CSI driver with volume type "gp2", as in launch configuration
				"storageClass": "gp2",
			},
		},
	}

	descArgsSvc := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + chartNamespaceGrafana,
		"describe",
		"service/grafana",
	}
	descCmdSvc := strings.Join(descArgsSvc, " ")

	return helm.Install(helm.InstallConfig{
		Logger:         ts.cfg.Logger,
		LogWriter:      ts.cfg.LogWriter,
		Stopc:          ts.cfg.Stopc,
		Timeout:        15 * time.Minute,
		KubeConfigPath: ts.cfg.EKSConfig.KubeConfigPath,
		Namespace:      chartNamespacePrometheus,
		ChartRepoURL:   chartRepoURL,
		ChartName:      chartNamePrometheus,
		ReleaseName:    chartNamePrometheus,
		Values:         values,
		QueryFunc: func() {
			fmt.Fprintf(ts.cfg.LogWriter, "\n")

			// to catch errors
			// e.g. "Error syncing load balancer: failed to ensure load balancer: TooManyLoadBalancers: Exceeded quota of account 123123"
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			output, err := exec.New().CommandContext(ctx, descArgsSvc[0], descArgsSvc[1:]...).CombinedOutput()
			cancel()
			out := strings.TrimSpace(string(output))
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl describe service/grafana' failed", zap.Error(err))
			} else {
				fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", descCmdSvc, out)
			}
		},
		QueryInterval: 30 * time.Second,
	})
}

func (ts *tester) deleteHelmPrometheus() error {
	return helm.Uninstall(helm.InstallConfig{
		Logger:         ts.cfg.Logger,
		LogWriter:      ts.cfg.LogWriter,
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
	values := map[string]interface{}{
		"adminUser":     ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaAdminUserName,
		"adminPassword": ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaAdminPassword,
		"nodeSelector": map[string]interface{}{
			// do not deploy in bottlerocket; PVC not working
			"AMIType": ec2config.AMITypeAL2X8664,
			"NGType":  ngType,
		},
		// standard_init_linux.go:211: exec user process caused "exec format error"
		// if set to "1.31.1"
		// ref. https://github.com/helm/charts/pull/23195
		// ref. https://github.com/aws/aws-k8s-tester/issues/131
		// make sure these are default empty in case chart version is <=5.4.0
		// ref. https://github.com/helm/charts/pull/23240
		"initChownData": map[string]interface{}{
			"image": map[string]interface{}{
				"tag": "latest",
				"sha": "",
			},
		},
		"persistence": map[string]interface{}{
			"enabled": true,
			// use CSI driver with volume type "gp2", as in launch configuration
			"storageClass": "gp2",
		},
		"service": map[string]interface{}{
			"type": "LoadBalancer",
		},
		"datasources": map[string]interface{}{
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
		},
		"dashboardProviders": map[string]interface{}{
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
		},
		"dashboards": map[string]interface{}{
			"default": map[string]interface{}{
				"kubernetes-cluster": map[string]interface{}{
					"gnetId":     6417,
					"revision":   1,
					"datasource": "Prometheus",
				},
			},
		},
	}
	return helm.Install(helm.InstallConfig{
		Logger:         ts.cfg.Logger,
		LogWriter:      ts.cfg.LogWriter,
		Stopc:          ts.cfg.Stopc,
		Timeout:        15 * time.Minute,
		KubeConfigPath: ts.cfg.EKSConfig.KubeConfigPath,
		Namespace:      chartNamespaceGrafana,
		ChartRepoURL:   chartRepoURL,
		ChartName:      chartNameGrafana,
		ReleaseName:    chartNameGrafana,
		Values:         values,
		QueryFunc: func() {
			getAllArgs := []string{
				ts.cfg.EKSConfig.KubectlPath,
				"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
				"--namespace=" + chartNamespaceGrafana,
				"get",
				"all",
			}
			getAllCmd := strings.Join(getAllArgs, " ")

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			output, err := exec.New().CommandContext(ctx, getAllArgs[0], getAllArgs[1:]...).CombinedOutput()
			cancel()
			out := strings.TrimSpace(string(output))
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl get all' failed", zap.Error(err))
			}
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", getAllCmd, out)
		},
		QueryInterval: 30 * time.Second,
	})
}

func (ts *tester) deleteHelmGrafana() error {
	return helm.Uninstall(helm.InstallConfig{
		Logger:         ts.cfg.Logger,
		LogWriter:      ts.cfg.LogWriter,
		Timeout:        15 * time.Minute,
		KubeConfigPath: ts.cfg.EKSConfig.KubeConfigPath,
		Namespace:      chartNamespaceGrafana,
		ChartName:      chartNameGrafana,
		ReleaseName:    chartNameGrafana,
	})
}

func (ts *tester) waitServiceGrafana() error {
	ts.cfg.Logger.Info("waiting for Grafana service")

	waitDur := 2 * time.Minute
	ts.cfg.Logger.Info("waiting for Grafana service", zap.Duration("wait", waitDur))
	select {
	case <-ts.cfg.Stopc:
		return errors.New("Grafana service creation aborted")
	case <-time.After(waitDur):
	}

	args := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + chartNameGrafana,
		"describe",
		"svc",
		grafanaServiceName,
	}
	argsCmd := strings.Join(args, " ")
	hostName := ""
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("Grafana service creation aborted")
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		cmdOut, err := exec.New().CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl describe svc' failed", zap.String("command", argsCmd), zap.Error(err))
		} else {
			out := string(cmdOut)
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"%s\" output:\n%s\n\n", argsCmd, out)
		}

		ts.cfg.Logger.Info("querying Grafana service for HTTP endpoint")
		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		so, err := ts.cfg.K8SClient.KubernetesClientSet().
			CoreV1().
			Services(chartNameGrafana).
			Get(ctx, grafanaServiceName, metav1.GetOptions{})
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

	fmt.Fprintf(ts.cfg.LogWriter, "\nNLB Grafana ARN: %s\n", ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaNLBARN)
	fmt.Fprintf(ts.cfg.LogWriter, "NLB Grafana Name: %s\n", ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaNLBName)
	fmt.Fprintf(ts.cfg.LogWriter, "NLB Grafana URL: %s\n", ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaURL)
	fmt.Fprintf(ts.cfg.LogWriter, "Grafana Admin User Name: %s\n", ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaAdminUserName)
	fmt.Fprintf(ts.cfg.LogWriter, "Grafana Admin Password: %d characters\n\n", len(ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaAdminPassword))

	ts.cfg.Logger.Info("waiting before testing Grafana Service")
	time.Sleep(20 * time.Second)

	retryStart = time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("Grafana Service creation aborted")
		case <-time.After(5 * time.Second):
		}

		out, err := httputil.ReadInsecure(ts.cfg.Logger, ioutil.Discard, ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaURL)
		if err != nil {
			ts.cfg.Logger.Warn("failed to read NLB Grafana Service; retrying", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}
		httpOutput := string(out)
		fmt.Fprintf(ts.cfg.LogWriter, "\nNLB Grafana Service output:\n%s\n", httpOutput)

		if strings.Contains(httpOutput, `Loading Grafana`) {
			ts.cfg.Logger.Info(
				"read Grafana Service; exiting",
				zap.String("host-name", hostName),
			)
			break
		}

		ts.cfg.Logger.Warn("unexpected Grafana Service output; retrying")
	}

	fmt.Fprintf(ts.cfg.LogWriter, "\nNLB Grafana ARN: %s\n", ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaNLBARN)
	fmt.Fprintf(ts.cfg.LogWriter, "NLB Grafana Name: %s\n", ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaNLBName)
	fmt.Fprintf(ts.cfg.LogWriter, "NLB Grafana URL: %s\n", ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaURL)
	fmt.Fprintf(ts.cfg.LogWriter, "Grafana Admin User Name: %s\n", ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaAdminUserName)
	fmt.Fprintf(ts.cfg.LogWriter, "Grafana Admin Password: %d characters\n\n", len(ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaAdminPassword))

	ts.cfg.EKSConfig.Sync()
	return nil
}

const grafanaServiceName = "grafana"

func (ts *tester) deleteGrafanaService() error {
	ts.cfg.Logger.Info("deleting grafana Service")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Services(chartNamespaceGrafana).
		Delete(
			ctx,
			grafanaServiceName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete grafana Service (%v)", err)
	}

	ts.cfg.Logger.Info("deleted grafana Service", zap.Error(err))
	ts.cfg.EKSConfig.Sync()
	return nil
}
