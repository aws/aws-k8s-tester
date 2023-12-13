package netpol

import (
	"bytes"
	"context"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/e2e2/internal/framework_extensions"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"
)

var testenv env.Environment

func TestMain(m *testing.M) {
	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("failed to initialize test environment: %v", err)
	}
	testenv = env.NewWithConfig(cfg)
	namespaces := []string{"a", "b", "c"}

	testenv.Setup(

		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			client, err := config.NewClient()
			if err != nil {
				return ctx, err
			}

			servers := map[string]string{
				"a": "a-server",
				"b": "b-server",
				"c": "c-server",
			}

			// 1. Install Latest CNI version
			log.Print("Install the latest VPC-CNI on the cluster")
			err = installLatestCNI(config, ctx)
			if err != nil {
				return ctx, errors.Wrap(err, "Failed to install latest aws-vpc-cni on cluster")
			}

			// 2. Create three namespaces
			log.Print("Creating the test namespaces")
			for _, ns := range namespaces {
				err = createNamespace(ns, client, ctx)
				if err != nil {
					return ctx, errors.Wrapf(err, "Failed to create namespace %s", ns)
				}
			}

			// 3. Create deployment and service
			log.Print("Creating the test deployment and service")
			for ns, server := range servers {
				err = createServerAndService(ns, server, 1, client, ctx)
				if err != nil {
					return ctx, errors.Wrapf(err, "Failed to create deployment and service for %s", server)
				}
			}

			return ctx, nil
		},
	)

	testenv.Finish(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			client, err := config.NewClient()

			if err != nil {
				return ctx, err
			}
			log.Print("Deleting the test namespaces")
			for _, ns := range namespaces {
				client.Resources().Delete(ctx, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns, Namespace: ns}})
			}

			return ctx, nil
		},
	)

	os.Exit(testenv.Run(m))
}

func installLatestCNI(config *envconf.Config, ctx context.Context) error {

	repoName := "eks"
	repoUrl := "https://aws.github.io/eks-charts"
	chartName := "eks/aws-vpc-cni"
	releaseNamespace := "kube-system"

	client, err := config.NewClient()

	if err != nil {
		return errors.Wrap(err, "Failed to get client")
	}

	// Delete Cluster role
	client.Resources().Delete(ctx, &rbac.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "aws-node", Namespace: releaseNamespace}})

	// Delete Cluster-Role Binding
	client.Resources().Delete(ctx, &rbac.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "aws-node", Namespace: releaseNamespace}})

	// Delete ServiceAccount
	client.Resources().Delete(ctx, &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "aws-node", Namespace: releaseNamespace}})

	// Delete Daemonset
	client.Resources().Delete(ctx, &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "aws-node", Namespace: releaseNamespace}})

	// Delete Configmap
	client.Resources().Delete(ctx, &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "amazon-vpc-cni", Namespace: releaseNamespace}})

	manager := helm.New(config.KubeconfigFile())
	if err := manager.RunRepo(helm.WithArgs("add", repoName, repoUrl)); err != nil {
		return errors.Wrap(err, "failed to add eks-charts repository")
	}

	if err := manager.RunRepo(helm.WithArgs("update")); err != nil {
		return errors.Wrap(err, "failed to upgrade eks-charts repo")
	}

	err = manager.RunInstall(helm.WithChart(chartName), helm.WithNamespace(releaseNamespace),
		helm.WithArgs("--generate-name", "--set", "enableNetworkPolicy=true", "--set", "originalMatchLabels=true"),
		helm.WithWait(), helm.WithTimeout("5m"))

	if err != nil {
		return err
	}

	cniDS := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "aws-node", Namespace: releaseNamespace},
	}

	err = wait.For(fwext.NewConditionExtension(client.Resources()).DaemonSetReady(cniDS), wait.WithTimeout(time.Minute*5))
	if err != nil {
		return err
	}

	log.Print("Installed the latest VPC-CNI using helm chart")
	return nil
}

func createNamespace(name string, client klient.Client, ctx context.Context) error {

	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: name,
			Labels:    map[string]string{"ns": name},
		},
	}

	if err := client.Resources().Create(ctx, ns); err != nil {
		return err
	}
	return nil
}

func createServerAndService(namespace string, name string, replicas int32, client klient.Client, ctx context.Context) error {

	labels := map[string]string{"app": name}

	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1.ServiceSpec{
			Ports:    []v1.ServicePort{{Name: name, Protocol: "TCP", Port: 80}},
			Selector: labels,
		},
	}

	if err := client.Resources().Create(ctx, service); err != nil {
		return err
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: name, Image: "nginx"}}},
			},
		},
	}

	if err := client.Resources().Create(ctx, deploy); err != nil {
		return err
	}

	err := wait.For(conditions.New(client.Resources()).DeploymentConditionMatch(deploy, appsv1.DeploymentAvailable, v1.ConditionTrue),
		wait.WithTimeout(time.Minute*5))
	if err != nil {
		return err
	}

	return nil
}

func TestNetworkPolicyCases(t *testing.T) {

	protocolTCP := corev1.ProtocolTCP
	protocolUDP := corev1.ProtocolUDP
	networkPolicy := networking.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "block-c-to-a", Namespace: "a"},
		Spec: networking.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "a-server"}},
			PolicyTypes: []networking.PolicyType{networking.PolicyTypeIngress, networking.PolicyTypeEgress},
			Ingress: []networking.NetworkPolicyIngressRule{
				{
					From: []networking.NetworkPolicyPeer{
						{
							PodSelector:       &metav1.LabelSelector{MatchLabels: map[string]string{"app": "b-server"}},
							NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"ns": "b"}},
						},
					},
					Ports: []networking.NetworkPolicyPort{
						{
							Protocol: &protocolTCP,
							Port:     &intstr.IntOrString{IntVal: 80},
						},
					},
				},
			},
			Egress: []networking.NetworkPolicyEgressRule{
				{
					To: []networking.NetworkPolicyPeer{
						{
							PodSelector:       &metav1.LabelSelector{MatchLabels: map[string]string{"app": "b-server"}},
							NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"ns": "b"}},
						},
					},
					Ports: []networking.NetworkPolicyPort{
						{
							Protocol: &protocolTCP,
							Port:     &intstr.IntOrString{IntVal: 80},
						},
					},
				},
				{
					Ports: []networking.NetworkPolicyPort{
						{
							Protocol: &protocolUDP,
							Port:     &intstr.IntOrString{IntVal: 53},
						},
					},
				},
			},
		},
	}

	allowAll := features.New("allowAll").
		WithLabel("suite", "netpol").
		WithLabel("policy", "none").
		Assess("curl from A to B succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				return ctx
			}
			pods := &corev1.PodList{}
			namespace := "a"
			containerName := "a-server"
			err = client.Resources("a").List(context.TODO(), pods)
			if err != nil || pods.Items == nil {
				t.Error("error while getting pods", err)
			}
			podName := pods.Items[0].Name

			var stdout, stderr bytes.Buffer
			command := []string{"curl", "-m", "2", "-I", "http://b-server.b:80"}
			client.Resources().ExecInPod(context.TODO(), namespace, podName, containerName, command, &stdout, &stderr)

			httpStatus := strings.Split(stdout.String(), "\n")[0]
			if !strings.Contains(httpStatus, "200") {
				t.Fatal("Couldn't connect to server B")
			}
			return ctx

		}).
		Assess("curl from C to A succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				return ctx
			}
			namespace := "c"
			containerName := "c-server"
			pods := &corev1.PodList{}
			err = client.Resources("c").List(context.TODO(), pods)
			if err != nil || pods.Items == nil {
				t.Error("error while getting pods", err)
			}
			podName := pods.Items[0].Name

			var stdout, stderr bytes.Buffer
			command := []string{"curl", "-m", "2", "-I", "http://a-server.a:80"}
			client.Resources().ExecInPod(context.TODO(), namespace, podName, containerName, command, &stdout, &stderr)

			httpStatus := strings.Split(stdout.String(), "\n")[0]
			if !strings.Contains(httpStatus, "200") {
				t.Fatal("Couldn't connect to server A")
			}
			return ctx
		}).
		Feature()

	blockCToA := features.New("blockCToA").
		WithLabel("suite", "netpol").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				return ctx
			}

			log.Print("Applying Network Policy")
			if err := client.Resources().Create(ctx, &networkPolicy); err != nil {
				t.Error("error while applying Network Policy", err)
				return ctx
			}

			// This time-wait is to account for Network Policy Controller to start up, run leader election in the control plane
			// and to apply the network policy
			time.Sleep(1 * time.Minute)

			return ctx

		}).
		Assess("curl from A to B succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				return ctx
			}
			pods := &corev1.PodList{}
			namespace := "a"
			containerName := "a-server"
			err = client.Resources("a").List(context.TODO(), pods)
			if err != nil || pods.Items == nil {
				t.Error("error while getting pods", err)
			}
			podName := pods.Items[0].Name

			var stdout, stderr bytes.Buffer
			command := []string{"curl", "-m", "2", "-I", "http://b-server.b:80"}
			client.Resources().ExecInPod(context.TODO(), namespace, podName, containerName, command, &stdout, &stderr)

			httpStatus := strings.Split(stdout.String(), "\n")[0]
			if !strings.Contains(httpStatus, "200") {
				t.Fatal("Couldn't connect to server B")
			}
			return ctx
		}).
		Assess("curl from C to A fails", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				return ctx
			}
			namespace := "c"
			containerName := "c-server"
			pods := &corev1.PodList{}
			err = client.Resources("c").List(context.TODO(), pods)
			if err != nil || pods.Items == nil {
				t.Error("error while getting pods", err)
			}
			podName := pods.Items[0].Name

			var stdout, stderr bytes.Buffer
			command := []string{"curl", "-m", "2", "-I", "http://a-server.a:80"}
			client.Resources().ExecInPod(context.TODO(), namespace, podName, containerName, command, &stdout, &stderr)

			httpStatus := strings.Split(stdout.String(), "\n")[0]
			if strings.Contains(httpStatus, "200") {
				t.Fatal("Network Policy didn't block connection to server A")
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				return ctx
			}

			if err := client.Resources().Delete(ctx, &networkPolicy); err != nil {
				t.Error("error while deleting Network Policy", err)
				return ctx
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, allowAll, blockCToA)
}
