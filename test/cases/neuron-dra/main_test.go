//go:build e2e

package neuron_dra

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	"github.com/aws/aws-k8s-tester/test/common"
	"github.com/aws/aws-k8s-tester/test/manifests"
	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

//go:embed rcts
var rctsFS embed.FS

var (
	testenv                   env.Environment
	nodeType                  *string
	rdmaDeviceDraDriverImage  *string
	acceleratorDraDriverImage *string
	containerTestImage        *string
	nodeCount                 int
)

// supportedRdmaTypes lists the recognized RDMA device types.
var supportedRdmaTypes = []string{"efa"}

func validateConfig() error {
	type requiredFlag struct {
		name  string
		value string
	}
	for _, f := range []requiredFlag{
		{"rdmaDeviceDraDriverImage", *rdmaDeviceDraDriverImage},
		{"containerTestImage", *containerTestImage},
		{"nodeType", *nodeType},
	} {
		if f.value == "" {
			return fmt.Errorf("-%s is required and must be non-empty", f.name)
		}
	}
	// Validate that nodeType maps to a known topology (and thus a known RDMA type)
	topo, err := GetTopologyForNodeType(*nodeType)
	if err != nil {
		return fmt.Errorf("invalid -nodeType: %w", err)
	}
	if !slices.Contains(supportedRdmaTypes, topo.RdmaType) {
		return fmt.Errorf("instance family %q has unsupported RDMA type %q; supported: %v", topo.Family, topo.RdmaType, supportedRdmaTypes)
	}
	// Verify helm is available on the PATH.
	if _, err := exec.LookPath("helm"); err != nil {
		return fmt.Errorf("helm is required but not found on PATH: %w", err)
	}
	return nil
}

const (
	neuronHelmReleaseName = "neuron-helm-chart"
	neuronHelmChartOCI    = "oci://public.ecr.aws/neuron/neuron-helm-chart"
	neuronDRANamespace    = "neuron-dra-driver"
)

// installNeuronDRADriverHelm installs the Neuron DRA driver via the public Helm chart.
// If acceleratorDraDriverImage is non-empty, it splits on the last ":" to extract
// repository and tag and passes them as --set overrides.
func installNeuronDRADriverHelm(ctx context.Context, config *envconf.Config) (context.Context, error) {
	args := []string{
		"upgrade", "--install", neuronHelmReleaseName, neuronHelmChartOCI,
		"--namespace", neuronDRANamespace,
		"--create-namespace",
		"--set", "devicePlugin.enabled=false",
		"--set", "npd.enabled=false",
		"--set", "draDriver.enabled=true",
		"--wait",
		"--timeout", "5m",
	}
	if *acceleratorDraDriverImage != "" {
		repo, tag := splitImageRepoTag(*acceleratorDraDriverImage)
		args = append(args,
			"--set", fmt.Sprintf("draDriver.image.repository=%s", repo),
			"--set", fmt.Sprintf("draDriver.image.tag=%s", tag),
		)
	}
	log.Printf("[INFO] Installing Neuron DRA driver via Helm: helm %s", strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, "helm", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return ctx, fmt.Errorf("helm install neuron-dra-driver failed: %w", err)
	}
	log.Println("Neuron DRA driver Helm release installed successfully.")
	return ctx, nil
}

// uninstallNeuronDRADriverHelm uninstalls the Neuron DRA driver Helm release.
func uninstallNeuronDRADriverHelm(ctx context.Context, config *envconf.Config) (context.Context, error) {
	args := []string{
		"uninstall", neuronHelmReleaseName,
		"--namespace", neuronDRANamespace,
	}
	log.Printf("[INFO] Uninstalling Neuron DRA driver Helm release: helm %s", strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, "helm", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("[WARN] helm uninstall neuron-dra-driver failed (may already be removed): %v", err)
	}
	return ctx, nil
}

// splitImageRepoTag splits a container image reference on the last ":" into
// repository and tag. If there is no ":", the entire string is treated as the
// repository and the tag defaults to "latest".
func splitImageRepoTag(image string) (repo, tag string) {
	idx := strings.LastIndex(image, ":")
	if idx < 0 {
		return image, "latest"
	}
	return image[:idx], image[idx+1:]
}

func deployNeuronDRADriver(ctx context.Context, config *envconf.Config) (context.Context, error) {
	ds := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "neuron-dra-driver-kubelet-plugin", Namespace: neuronDRANamespace},
	}
	err := wait.For(
		fwext.NewConditionExtension(config.Client().Resources()).DaemonSetReady(&ds),
		wait.WithTimeout(5*time.Minute),
		wait.WithContext(ctx),
	)
	if err != nil {
		return ctx, fmt.Errorf("neuron-dra-driver daemonset is not ready: %w", err)
	}
	log.Println("neuron-dra-driver daemonset is ready.")
	return ctx, nil
}

// loadRCTManifests reads all RCT YAML files for the given nodeType from
// the embedded filesystem and returns them as raw byte slices suitable for
// fwext.ApplyManifests.
func loadRCTManifests(nodeType string) ([][]byte, error) {
	topo, err := GetTopologyForNodeType(nodeType)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join("rcts", topo.RCTSubDir)
	entries, err := fs.ReadDir(rctsFS, dir)
	if err != nil {
		return nil, fmt.Errorf("reading RCT directory %s: %w", dir, err)
	}
	var rctManifests [][]byte
	for _, entry := range entries {
		if entry.IsDir() || !isYAMLFile(entry.Name()) {
			continue
		}
		data, err := fs.ReadFile(rctsFS, filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", entry.Name(), err)
		}
		rctManifests = append(rctManifests, data)
	}
	return rctManifests, nil
}

func countNodes(ctx context.Context, config *envconf.Config) (context.Context, error) {
	clientset, err := kubernetes.NewForConfig(config.Client().RESTConfig())
	if err != nil {
		return ctx, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return ctx, fmt.Errorf("failed to list nodes: %w", err)
	}
	for _, node := range nodes.Items {
		if node.Labels["node.kubernetes.io/instance-type"] == *nodeType {
			nodeCount++
		}
	}
	if nodeCount == 0 {
		return ctx, fmt.Errorf("no nodes of type %q found", *nodeType)
	}
	log.Printf("[INFO] Found %d node(s) of type %s", nodeCount, *nodeType)
	return ctx, nil
}

func TestMain(m *testing.M) {
	nodeType = flag.String("nodeType", "", "instance type for the cluster (e.g. trn1.32xlarge)")
	rdmaDeviceDraDriverImage = flag.String("rdmaDeviceDraDriverImage", "", "container image for the dranet DRA driver")
	acceleratorDraDriverImage = flag.String("acceleratorDraDriverImage", "", "container image for the Neuron DRA driver")
	containerTestImage = flag.String("containerTestImage", "", "container image for the nccom test workload")

	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("failed to initialize test environment: %v", err)
	}

	if err := validateConfig(); err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	testenv = env.NewWithConfig(cfg).WithContext(ctx)

	// Build the manifest list and setup functions dynamically.
	// Resolve topology to determine RDMA type from nodeType.
	topo, err := GetTopologyForNodeType(*nodeType)
	if err != nil {
		log.Fatalf("failed to resolve topology: %v", err)
	}

	manifestsList := [][]byte{
		manifests.MpiOperatorManifest,
	}
	setUpFunctions := []env.Func{
		// Run independent setup steps concurrently.
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			var mu sync.Mutex
			g, gctx := errgroup.WithContext(ctx)

			// Deploy MPI operator.
			g.Go(func() error {
				return common.DeployMPIOperator(gctx, config)
			})

			// Deploy dranet and RCTs based on topology's RDMA type.
			if topo.RdmaType == "efa" {
				rctManifests, err := loadRCTManifests(*nodeType)
				if err != nil {
					return ctx, fmt.Errorf("failed to load RCT manifests: %w", err)
				}
				mu.Lock()
				manifestsList = append(manifestsList, rctManifests...)
				mu.Unlock()

				g.Go(func() error {
					renderedDranet, err := common.DeployDranet(gctx, config, *rdmaDeviceDraDriverImage)
					if err != nil {
						return err
					}
					mu.Lock()
					manifestsList = append(manifestsList, renderedDranet)
					mu.Unlock()
					return nil
				})

				g.Go(func() error {
					return fwext.ApplyManifests(config.Client().RESTConfig(), rctManifests...)
				})
			}

			// Install Neuron DRA driver via Helm chart.
			g.Go(func() error {
				_, err := installNeuronDRADriverHelm(gctx, config)
				return err
			})

			if err := g.Wait(); err != nil {
				return ctx, err
			}
			return ctx, nil
		},
		deployNeuronDRADriver,
		countNodes,
	}
	testenv.Setup(setUpFunctions...)

	testenv.Finish(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			// Uninstall Neuron DRA driver Helm release first.
			ctx, _ = uninstallNeuronDRADriverHelm(ctx, config)
			// Delete remaining manifests in reverse order.
			slices.Reverse(manifestsList)
			if err := fwext.DeleteManifests(config.Client().RESTConfig(), manifestsList...); err != nil {
				return ctx, fmt.Errorf("failed to delete manifests: %w", err)
			}
			return ctx, nil
		},
	)

	os.Exit(testenv.Run(m))
}
