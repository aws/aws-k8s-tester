// Package prometheusgrafana implements Prometheus/Grafana add-on.
package prometheusgrafana

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
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
	chartURL      = "https://kubernetes-charts.storage.googleapis.com"

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
	if err := helm.RepoAdd(ts.cfg.Logger, chartRepoName, chartURL); err != nil {
		return err
	}
	if err := ts.installHelmPrometheus(); err != nil {
		return err
	}
	if err := ts.installHelmGrafana(); err != nil {
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

	if err := ts.uninstallHelmGrafana(); err != nil {
		errs = append(errs, err.Error())
	}

	if err := ts.uninstallHelmPrometheus(); err != nil {
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

/*
# https://eksworkshop.com/intermediate/240_monitoring/deploy-prometheus/
# https://github.com/helm/charts/tree/master/stable/prometheus

kubectl create namespace prometheus
helm install prometheus stable/prometheus \
  --namespace prometheus \
  --set alertmanager.persistentVolume.storageClass="gp2" \
  --set server.persistentVolume.storageClass="gp2"
*/
func (ts *tester) installHelmPrometheus() error {
	ngType := "custom"
	if ts.cfg.EKSConfig.IsEnabledAddOnManagedNodeGroups() {
		ngType = "managed"
	}

	values := make(map[string]interface{})

	// TODO: PVC not working on BottleRocket
	// do not assign mariadb to Bottlerocket
	// e.g. MountVolume.MountDevice failed for volume "pvc-8e035a13-4d33-472f-a4c0-f36c7d39d170" : executable file not found in $PATH
	values["nodeSelector"] = map[string]interface{}{"AMIType": ec2config.AMITypeAL2X8664, "NGType": ngType}

	values["alertmanager.persistentVolume.storageClass"] = "gp2"
	values["server.persistentVolume.storageClass"] = "gp2"

	return helm.Install(
		ts.cfg.Logger,
		10*time.Minute,
		ts.cfg.EKSConfig.KubeConfigPath,
		chartNamespacePrometheus,
		chartURL,
		chartNamePrometheus,
		chartNamePrometheus,
		values,
	)
}

func (ts *tester) uninstallHelmPrometheus() error {
	return helm.Uninstall(
		ts.cfg.Logger,
		10*time.Minute,
		ts.cfg.EKSConfig.KubeConfigPath,
		chartNamespacePrometheus,
		chartNamePrometheus,
	)
}

/*
# https://eksworkshop.com/intermediate/240_monitoring/deploy-grafana/
# https://github.com/helm/charts/tree/master/stable/grafana

kubectl create namespace grafana
helm install grafana stable/grafana \
  --namespace grafana \
  --set persistence.storageClassName="gp2" \
  --set adminPassword='EKS!sAWSome' \
  --set datasources."datasources\.yaml".apiVersion=1 \
  --set datasources."datasources\.yaml".datasources[0].name=Prometheus \
  --set datasources."datasources\.yaml".datasources[0].type=prometheus \
  --set datasources."datasources\.yaml".datasources[0].url=http://prometheus-server.prometheus.svc.cluster.local \
  --set datasources."datasources\.yaml".datasources[0].access=proxy \
  --set datasources."datasources\.yaml".datasources[0].isDefault=true \
  --set service.type=LoadBalancer
*/
func (ts *tester) installHelmGrafana() error {
	ngType := "custom"
	if ts.cfg.EKSConfig.IsEnabledAddOnManagedNodeGroups() {
		ngType = "managed"
	}

	values := make(map[string]interface{})

	// TODO: PVC not working on BottleRocket
	// do not assign mariadb to Bottlerocket
	// e.g. MountVolume.MountDevice failed for volume "pvc-8e035a13-4d33-472f-a4c0-f36c7d39d170" : executable file not found in $PATH
	values["nodeSelector"] = map[string]interface{}{"AMIType": ec2config.AMITypeAL2X8664, "NGType": ngType}

	values["persistence.storageClassName"] = "gp2"
	values["adminPassword"] = ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaAdminPassword
	values[`datasources."datasources.yaml".apiVersion`] = 1
	values[`datasources."datasources.yaml".datasources[0].name`] = "Prometheus"
	values[`datasources."datasources.yaml".datasources[0].type`] = "prometheus"
	values[`datasources."datasources.yaml".datasources[0].url`] = "http://prometheus-server.prometheus.svc.cluster.local"
	values[`datasources."datasources.yaml".datasources[0].access`] = "proxy"
	values[`datasources."datasources.yaml".datasources[0].isDefault`] = true
	values["service.type"] = "LoadBalancer"

	return helm.Install(
		ts.cfg.Logger,
		10*time.Minute,
		ts.cfg.EKSConfig.KubeConfigPath,
		chartNamespaceGrafana,
		chartURL,
		chartNameGrafana,
		chartNameGrafana,
		values,
	)
}

func (ts *tester) uninstallHelmGrafana() error {
	return helm.Uninstall(
		ts.cfg.Logger,
		10*time.Minute,
		ts.cfg.EKSConfig.KubeConfigPath,
		chartNamespaceGrafana,
		chartNameGrafana,
	)
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

	ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaURL = "http://" + hostName

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

	fmt.Printf("\nNLB Grafana ARN %s\n", ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaNLBARN)
	fmt.Printf("NLB Grafana Name %s\n", ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaNLBName)
	fmt.Printf("NLB Grafana URL %s\n\n", ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaURL)

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

		buf := bytes.NewBuffer(nil)
		err := httpReadInsecure(ts.cfg.Logger, ts.cfg.EKSConfig.AddOnPrometheusGrafana.GrafanaURL, buf)
		if err != nil {
			ts.cfg.Logger.Warn("failed to read NLB Grafana Service; retrying", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		httpOutput := buf.String()
		fmt.Printf("\nNLB Grafana Service output:\n%s\n", httpOutput)

		if strings.Contains(httpOutput, `<p>Welcome to Grafana. This is your first post.`) || true {
			ts.cfg.Logger.Info(
				"read Grafana Service; exiting",
				zap.String("host-name", hostName),
			)
			break
		}

		ts.cfg.Logger.Warn("unexpected Grafana Service output; retrying")
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
