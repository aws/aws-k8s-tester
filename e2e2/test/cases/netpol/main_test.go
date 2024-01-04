package netpol

import (
	"context"
	"flag"
	"log"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"

	fwext "github.com/aws/aws-k8s-tester/e2e2/internal/framework_extensions"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var (
	testenv           env.Environment
	clusterName       string
	endPointUrl       string
	kubernetesVersion string
	addonName         string = "vpc-cni"
)

func TestMain(m *testing.M) {

	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("failed to initialize test environment: %v", err)
	}

	config, err := config.LoadDefaultConfig(context.TODO())
	eksclient := eks.NewFromConfig(config)
	testenv = env.NewWithConfig(cfg)

	flag.StringVar(&clusterName, "cluster-name", "", "Name of the cluster")
	flag.StringVar(&endPointUrl, "endpoint-url", "", "Endpoint url to use")
	flag.Parse()

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
			kubernetesVersion, err = getClusterVersion(ctx, eksclient)
			if err != nil {
				return ctx, err
			}

			err = installLatestCNIVersion(ctx, config, eksclient)
			if err != nil {
				return ctx, err
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

			log.Print("Installing the Default version of VPC-CNI on the cluster")
			err = installDefaultCNIVersion(ctx, config, eksclient)
			if err != nil {
				return ctx, err
			}
			return ctx, nil
		},
	)

	os.Exit(testenv.Run(m))
}

func installDefaultCNIVersion(ctx context.Context, config *envconf.Config, eksclient *eks.Client) error {

	// Uninstall the currently install addon
	uninstallCNIAddon(ctx, config, eksclient)

	// Passing addonVersion empty installs the default version of addon
	err := installCNIAddon(ctx, config, eksclient, "", "")
	if err != nil {
		return errors.Wrap(err, "Could not install the default addon version")
	}

	return nil
}

func installLatestCNIVersion(ctx context.Context, config *envconf.Config, eksclient *eks.Client) error {

	version, err := getLatestCNIAddon(ctx, eksclient)
	if err != nil {
		return err
	}

	configurationValues := "{\"enableNetworkPolicy\": \"true\"}"
	err = installCNIAddon(ctx, config, eksclient, version, configurationValues)
	if err != nil {
		return err
	}

	return nil
}

func uninstallCNIAddon(ctx context.Context, config *envconf.Config, eksclient *eks.Client) error {

	cniDS := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "aws-node", Namespace: "kube-system"}}

	_, err := eksclient.DeleteAddon(ctx, &eks.DeleteAddonInput{
		AddonName:   aws.String(addonName),
		ClusterName: aws.String(clusterName),
	})

	err = wait.For(conditions.New(config.Client().Resources()).ResourceDeleted(cniDS), wait.WithTimeout(time.Minute*5))
	if err != nil {
		return errors.Wrap(err, "Daemonset could not be deleted")
	}

	return nil
}

func getLatestCNIAddon(ctx context.Context, eksclient *eks.Client) (string, error) {

	addonVersions, err := eksclient.DescribeAddonVersions(ctx, &eks.DescribeAddonVersionsInput{
		AddonName:         aws.String(addonName),
		KubernetesVersion: aws.String(kubernetesVersion),
	})

	if err != nil {
		return "", err
	}

	if len(*&addonVersions.Addons) > 0 {
		return *addonVersions.Addons[0].AddonVersions[0].AddonVersion, nil
	} else {
		return "", errors.Errorf("Addon versions not available")
	}
}

func installCNIAddon(ctx context.Context, config *envconf.Config, eksclient *eks.Client, addonVersion string, configurationValues string) error {

	// Delete old Daemonset if exists
	cniDS := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "aws-node", Namespace: "kube-system"}}
	config.Client().Resources().Delete(ctx, cniDS)

	_, err := eksclient.CreateAddon(ctx, &eks.CreateAddonInput{
		AddonName:           aws.String(addonName),
		ClusterName:         aws.String(clusterName),
		AddonVersion:        aws.String(addonVersion),
		ConfigurationValues: aws.String(configurationValues),
		ResolveConflicts:    types.ResolveConflictsOverwrite,
	})

	if err != nil {
		return errors.Wrap(err, "Failed to create addon")
	}

	err = wait.For(fwext.NewConditionExtension(config.Client().Resources()).DaemonSetReady(cniDS), wait.WithTimeout(time.Minute*5))

	if err != nil {
		return errors.Wrap(err, "Daemonset failed to reach running state")
	}

	return nil
}

func getClusterVersion(ctx context.Context, eksclient *eks.Client) (string, error) {

	cluster, err := eksclient.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	})

	if err != nil {
		return "", err
	}

	return *cluster.Cluster.Version, nil
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
