//go:build e2e

package common

import (
	"context"
	"fmt"
	"log"
	"time"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	"github.com/aws/aws-k8s-tester/test/manifests"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// DeployDranet renders the dranet manifest template with the given image,
// applies it to the cluster, and waits for the dranet DaemonSet to be ready.
// Returns the rendered manifest bytes for later cleanup.
func DeployDranet(ctx context.Context, config *envconf.Config, rdmaDeviceDraDriverImage string) (renderedManifest []byte, err error) {
	renderedManifest, err = fwext.RenderManifests(manifests.DranetManifest, struct {
		RdmaDeviceDraDriverImage string
	}{
		RdmaDeviceDraDriverImage: rdmaDeviceDraDriverImage,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to render dranet manifest: %w", err)
	}
	if err := fwext.ApplyManifests(config.Client().RESTConfig(), renderedManifest); err != nil {
		return nil, fmt.Errorf("failed to apply dranet manifest: %w", err)
	}
	ds := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "dranet", Namespace: "kube-system"},
	}
	err = wait.For(
		fwext.NewConditionExtension(config.Client().Resources()).DaemonSetReady(&ds),
		wait.WithTimeout(5*time.Minute),
		wait.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("dranet daemonset is not ready: %w", err)
	}
	log.Println("dranet daemonset is ready.")
	return renderedManifest, nil
}

// DeployMPIOperator applies the MPI operator manifest and waits for the
// mpi-operator Deployment to become available.
func DeployMPIOperator(ctx context.Context, config *envconf.Config) error {
	if err := fwext.ApplyManifests(config.Client().RESTConfig(), manifests.MpiOperatorManifest); err != nil {
		return fmt.Errorf("failed to apply mpi-operator manifest: %w", err)
	}
	dep := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "mpi-operator", Namespace: "mpi-operator"},
	}
	err := wait.For(conditions.New(config.Client().Resources()).DeploymentConditionMatch(&dep, appsv1.DeploymentAvailable, v1.ConditionTrue),
		wait.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to deploy mpi-operator: %w", err)
	}
	log.Println("mpi-operator deployment is available.")
	return nil
}
