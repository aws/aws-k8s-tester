// Package jupyterhub implements Jupyter Hub add-on.
// ref. https://zero-to-jupyterhub.readthedocs.io/en/latest/index.html
package jupyterhub

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
	"github.com/aws/aws-sdk-go/aws"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// Config defines Jupyter Hub configuration.
// ref. https://zero-to-jupyterhub.readthedocs.io/en/latest/index.html
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}
	Sig    chan os.Signal

	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

// Tester defines Jupyter Hub tester.
// ref. https://zero-to-jupyterhub.readthedocs.io/en/latest/index.html
type Tester interface {
	// Create installs Jupyter Hub.
	Create() error
	// Delete deletes Jupyter Hub.
	Delete() error
}

func NewTester(cfg Config) (Tester, error) {
	return &tester{cfg: cfg}, nil
}

type tester struct {
	cfg Config
}

const (
	chartRepoName = "jupyterhub"
	chartRepoURL  = "https://jupyterhub.github.io/helm-chart"
	chartName     = "jupyterhub"
)

func (ts *tester) Create() error {
	if ts.cfg.EKSConfig.AddOnJupyterHub.Created {
		ts.cfg.Logger.Info("skipping create AddOnJupyterHub")
		return nil
	}

	ts.cfg.EKSConfig.AddOnJupyterHub.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()

	defer func() {
		ts.cfg.EKSConfig.AddOnJupyterHub.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.AddOnJupyterHub.CreateTookString = ts.cfg.EKSConfig.AddOnJupyterHub.CreateTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	// https://zero-to-jupyterhub.readthedocs.io/en/latest/setup-jupyterhub/setup-helm.html
	if err := ts.createTillerServiceAccount(); err != nil {
		return err
	}
	if err := ts.createTillerRBACClusterRoleBinding(); err != nil {
		return err
	}
	if err := k8s_client.CreateNamespace(ts.cfg.Logger, ts.cfg.K8SClient.KubernetesClientSet(), ts.cfg.EKSConfig.AddOnJupyterHub.Namespace); err != nil {
		return err
	}
	if err := helm.RepoAdd(ts.cfg.Logger, chartRepoName, chartRepoURL); err != nil {
		return err
	}
	if err := ts.createHelmJupyterHub(); err != nil {
		return err
	}
	if err := ts.waitService(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.AddOnJupyterHub.Created {
		ts.cfg.Logger.Info("skipping delete AddOnJupyterHub")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnJupyterHub.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.AddOnJupyterHub.DeleteTookString = ts.cfg.EKSConfig.AddOnJupyterHub.DeleteTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := ts.deleteHelmJupyterHub(); err != nil {
		errs = append(errs, err.Error())
	}

	if err := ts.deleteTillerRBACClusterRoleBinding(); err != nil {
		errs = append(errs, err.Error())
	}

	if err := ts.deleteTillerServiceAccount(); err != nil {
		errs = append(errs, err.Error())
	}

	if err := k8s_client.DeleteNamespaceAndWait(ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnJupyterHub.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete JupyterHub namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnJupyterHub.Created = false
	return ts.cfg.EKSConfig.Sync()
}

// https://zero-to-jupyterhub.readthedocs.io/en/latest/setup-jupyterhub/setup-helm.html
func (ts *tester) createTillerServiceAccount() error {
	ts.cfg.Logger.Info("creating Tiller ServiceAccount")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts("kube-system").
		Create(
			ctx,
			&v1.ServiceAccount{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tiller",
					Namespace: "kube-system",
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create Tiller ServiceAccount (%v)", err)
	}

	ts.cfg.Logger.Info("created Tiller ServiceAccount")
	return ts.cfg.EKSConfig.Sync()
}

// https://zero-to-jupyterhub.readthedocs.io/en/latest/setup-jupyterhub/setup-helm.html
func (ts *tester) deleteTillerServiceAccount() error {
	ts.cfg.Logger.Info("deleting Tiller ServiceAccount")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts("kube-system").
		Delete(
			ctx,
			"tiller",
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !strings.Contains(err.Error(), " not found") {
		return fmt.Errorf("failed to delete Tiller ServiceAccount (%v)", err)
	}
	ts.cfg.Logger.Info("deleted Tiller ServiceAccount", zap.Error(err))

	return ts.cfg.EKSConfig.Sync()
}

// https://github.com/jupyterhub/zero-to-jupyterhub-k8s/blob/master/jupyterhub/values.yaml
func (ts *tester) createTillerRBACClusterRoleBinding() error {
	ts.cfg.Logger.Info("creating Tiller RBAC ClusterRoleBinding")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoleBindings().
		Create(
			ctx,
			&rbacv1.ClusterRoleBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "ClusterRoleBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tiller",
					Namespace: "default",
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:      "ServiceAccount",
						Name:      "tiller",
						Namespace: "kube-system",
					},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     "cluster-admin",
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create Tiller RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("created Tiller RBAC ClusterRoleBinding")
	return ts.cfg.EKSConfig.Sync()
}

// https://github.com/jupyterhub/zero-to-jupyterhub-k8s/blob/master/jupyterhub/values.yaml
func (ts *tester) deleteTillerRBACClusterRoleBinding() error {
	ts.cfg.Logger.Info("deleting Tiller RBAC ClusterRoleBinding")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoleBindings().
		Delete(
			ctx,
			"tiller",
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !strings.Contains(err.Error(), " not found") {
		return fmt.Errorf("failed to delete Tiller RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("deleted Tiller RBAC ClusterRoleBinding", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

// https://github.com/jupyterhub/zero-to-jupyterhub-k8s/blob/master/jupyterhub/values.yaml
func (ts *tester) createHelmJupyterHub() error {
	ngType := "managed"
	if ts.cfg.EKSConfig.IsEnabledAddOnNodeGroups() {
		// TODO: test in MNG
		ngType = "custom"
	}
	// https://github.com/jupyterhub/zero-to-jupyterhub-k8s/blob/master/jupyterhub/values.yaml
	values := map[string]interface{}{
		// https://github.com/jupyterhub/zero-to-jupyterhub-k8s/blob/master/jupyterhub/values.yaml
		"hub": map[string]interface{}{
			"nodeSelector": map[string]interface{}{
				"NGType": ngType,
				// do not deploy in bottlerocket; PVC not working
				"AMIType": ec2config.AMITypeAL2X8664GPU,
			},
		},
		// https://github.com/jupyterhub/zero-to-jupyterhub-k8s/blob/master/jupyterhub/values.yaml
		"proxy": map[string]interface{}{
			"secretToken": ts.cfg.EKSConfig.AddOnJupyterHub.ProxySecretToken,
			"service": map[string]interface{}{
				"type":                  "LoadBalancer",
				"port":                  80,
				"httpsPort":             443,
				"httpsTargetPort":       "https",
				"externalTrafficPolicy": "Cluster",
			},
			"nodeSelector": map[string]interface{}{
				"NGType": ngType,
				// do not deploy in bottlerocket; PVC not working
				"AMIType": ec2config.AMITypeAL2X8664GPU,
			},
			"https": map[string]interface{}{
				"enabled": false,
			},
		},
		// https://zero-to-jupyterhub.readthedocs.io/en/latest/administrator/optimization.html
		// https://github.com/jupyterhub/zero-to-jupyterhub-k8s/blob/master/jupyterhub/values.yaml
		"scheduling": map[string]interface{}{
			"userScheduler": map[string]interface{}{
				"enabled": true,
				"nodeSelector": map[string]interface{}{
					"NGType": ngType,
					// do not deploy in bottlerocket; PVC not working
					"AMIType": ec2config.AMITypeAL2X8664GPU,
				},
			},
		},
		// https://zero-to-jupyterhub.readthedocs.io/en/latest/administrator/optimization.html#additional-sources
		// https://github.com/jupyterhub/zero-to-jupyterhub-k8s/blob/master/jupyterhub/values.yaml
		// "singleuser": map[string]interface{}{
		// 	"serviceAccountName": "tiller",
		// 	"image": map[string]interface{}{
		// 		"name": "jupyter/minimal-notebook",
		// 		"tag":  "2343e33dec46",
		// 	},
		// 	"profileList": []map[string]interface{}{
		// 		{
		// 			"display_name": "Minimal environment",
		// 			"description":  "To avoid too much bells and whistles: Python",
		// 			"default":      true,
		// 		},
		// 		{
		// 			"display_name": "Datascience environment",
		// 			"description":  "If you want the additional bells and whistles: Python, R, and Julia",
		// 			"kubespawner_override": map[string]interface{}{
		// 				"image": "jupyter/datascience-notebook:2343e33dec46",
		// 			},
		// 		},
		// 	},
		// },
	}
	return helm.Install(helm.InstallConfig{
		Logger:         ts.cfg.Logger,
		Stopc:          ts.cfg.Stopc,
		Sig:            ts.cfg.Sig,
		Timeout:        20 * time.Minute,
		KubeConfigPath: ts.cfg.EKSConfig.KubeConfigPath,
		Namespace:      ts.cfg.EKSConfig.AddOnJupyterHub.Namespace,
		ChartRepoURL:   chartRepoURL,
		ChartName:      chartName,
		ReleaseName:    chartName,
		Values:         values,
		QueryFunc:      nil,
		QueryInterval:  30 * time.Second,
	})
}

func (ts *tester) deleteHelmJupyterHub() error {
	return helm.Uninstall(helm.InstallConfig{
		Logger:         ts.cfg.Logger,
		Timeout:        20 * time.Minute,
		KubeConfigPath: ts.cfg.EKSConfig.KubeConfigPath,
		Namespace:      ts.cfg.EKSConfig.AddOnJupyterHub.Namespace,
		ChartName:      chartName,
		ReleaseName:    chartName,
	})
}

func (ts *tester) waitService() error {
	svcName := "proxy-public"
	ts.cfg.Logger.Info("waiting for JupyterHub service")

	waitDur := 2 * time.Minute
	ts.cfg.Logger.Info("waiting for JupyterHub service", zap.Duration("wait", waitDur))
	select {
	case <-ts.cfg.Stopc:
		return errors.New("JupyterHub service creation aborted")
	case sig := <-ts.cfg.Sig:
		return fmt.Errorf("received os signal %v", sig)
	case <-time.After(waitDur):
	}

	args := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnJupyterHub.Namespace,
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
			return errors.New("JupyterHub service creation aborted")
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

		ts.cfg.Logger.Info("querying JupyterHub service for HTTP endpoint")
		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		so, err := ts.cfg.K8SClient.KubernetesClientSet().
			CoreV1().
			Services(ts.cfg.EKSConfig.AddOnJupyterHub.Namespace).
			Get(ctx, svcName, metav1.GetOptions{})
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("failed to get JupyterHub service; retrying", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		ts.cfg.Logger.Info(
			"JupyterHub service has been linked to LoadBalancer",
			zap.String("load-balancer", fmt.Sprintf("%+v", so.Status.LoadBalancer)),
		)
		for _, ing := range so.Status.LoadBalancer.Ingress {
			ts.cfg.Logger.Info(
				"JupyterHub service has been linked to LoadBalancer.Ingress",
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

	ts.cfg.EKSConfig.AddOnJupyterHub.URL = "http://" + hostName

	// TODO: is there any better way to find out the NLB name?
	ts.cfg.EKSConfig.AddOnJupyterHub.NLBName = strings.Split(hostName, "-")[0]
	ss := strings.Split(hostName, ".")[0]
	ss = strings.Replace(ss, "-", "/", -1)
	ts.cfg.EKSConfig.AddOnJupyterHub.NLBARN = fmt.Sprintf(
		"arn:aws:elasticloadbalancing:%s:%s:loadbalancer/net/%s",
		ts.cfg.EKSConfig.Region,
		ts.cfg.EKSConfig.Status.AWSAccountID,
		ss,
	)

	fmt.Printf("\nNLB JupyterHub ARN: %s\n", ts.cfg.EKSConfig.AddOnJupyterHub.NLBARN)
	fmt.Printf("NLB JupyterHub Name: %s\n", ts.cfg.EKSConfig.AddOnJupyterHub.NLBName)
	fmt.Printf("NLB JupyterHub URL: %s\n\n", ts.cfg.EKSConfig.AddOnJupyterHub.URL)

	ts.cfg.Logger.Info("waiting before testing JupyterHub Service")
	time.Sleep(20 * time.Second)

	retryStart = time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("JupyterHub Service creation aborted")
		case sig := <-ts.cfg.Sig:
			return fmt.Errorf("received os signal %v", sig)
		case <-time.After(5 * time.Second):
		}

		out, err := httputil.ReadInsecure(ts.cfg.Logger, os.Stderr, ts.cfg.EKSConfig.AddOnJupyterHub.URL)
		if err != nil {
			ts.cfg.Logger.Warn("failed to read NLB JupyterHub Service; retrying", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}
		httpOutput := string(out)
		fmt.Printf("\nNLB JupyterHub Service output:\n%s\n", httpOutput)

		if strings.Contains(httpOutput, `jupyterhub-logo`) || true {
			ts.cfg.Logger.Info(
				"read JupyterHub Service; exiting",
				zap.String("host-name", hostName),
			)
			break
		}

		ts.cfg.Logger.Warn("unexpected JupyterHub Service output; retrying")
	}

	fmt.Printf("\nNLB JupyterHub ARN: %s\n", ts.cfg.EKSConfig.AddOnJupyterHub.NLBARN)
	fmt.Printf("NLB JupyterHub Name: %s\n", ts.cfg.EKSConfig.AddOnJupyterHub.NLBName)
	fmt.Printf("NLB JupyterHub URL: %s\n\n", ts.cfg.EKSConfig.AddOnJupyterHub.URL)

	return ts.cfg.EKSConfig.Sync()
}
