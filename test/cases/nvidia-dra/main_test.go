//go:build e2e

package nvidia_dra

import (
	"context"
	"embed"
	"flag"
	"fmt"
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
	clientset                 kubernetes.Interface
	nodeType                  *string
	rdmaDeviceDraDriverImage  *string
	acceleratorDraDriverImage *string
	containerTestImage        *string
	nodeCount                 int
)

// supportedRdmaTypes lists the recognized RDMA device types.
var supportedRdmaTypes = []string{"efa"}

func validateConfig() error {
	if err := common.ValidateRequiredFlags(map[string]string{
		"rdmaDeviceDraDriverImage": *rdmaDeviceDraDriverImage,
		"containerTestImage":       *containerTestImage,
		"nodeType":                 *nodeType,
	}); err != nil {
		return err
	}
	// Validate that nodeType maps to a known topology (and thus a known RDMA type).
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
	// Verify kubectl is available on the PATH.
	if _, err := exec.LookPath("kubectl"); err != nil {
		return fmt.Errorf("kubectl is required but not found on PATH: %w", err)
	}
	return nil
}

const (
	nvidiaDRAHelmReleaseName = "nvidia-dra-driver-gpu"
	nvidiaDRAHelmRepoName    = "nvidia-dra"
	nvidiaDRAHelmRepoURL     = "https://helm.ngc.nvidia.com/nvidia"
	nvidiaDRANamespace       = "nvidia-dra-driver-gpu"
	nvidiaDRAHelmChartVer    = "25.8.1"
)

// labelNodesGPUPresent labels all nodes with nvidia.com/gpu.present=true.
func labelNodesGPUPresent(ctx context.Context) error {
	args := []string{
		"label", "nodes", "--all",
		"nvidia.com/gpu.present=true",
		"--overwrite",
	}
	log.Printf("[INFO] Labeling nodes: kubectl %s", strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl label nodes failed: %w", err)
	}
	log.Println("All nodes labeled with nvidia.com/gpu.present=true.")
	return nil
}

// installNvidiaDRADriverHelm adds the NVIDIA Helm repo and installs the NVIDIA DRA driver.
// If acceleratorDraDriverImage is non-empty, it splits on the last ":" to extract
// repository and tag and passes them as --set overrides.
func installNvidiaDRADriverHelm(ctx context.Context, config *envconf.Config) (context.Context, error) {
	// Add the Helm repo.
	repoArgs := []string{"repo", "add", nvidiaDRAHelmRepoName, nvidiaDRAHelmRepoURL}
	log.Printf("[INFO] Adding NVIDIA Helm repo: helm %s", strings.Join(repoArgs, " "))
	repoCmd := exec.CommandContext(ctx, "helm", repoArgs...)
	repoCmd.Stdout = os.Stdout
	repoCmd.Stderr = os.Stderr
	if err := repoCmd.Run(); err != nil {
		return ctx, fmt.Errorf("helm repo add nvidia-dra failed: %w", err)
	}

	// Install (or upgrade) the chart.
	args := []string{
		"upgrade", "--install", nvidiaDRAHelmReleaseName,
		fmt.Sprintf("%s/%s", nvidiaDRAHelmRepoName, nvidiaDRAHelmReleaseName),
		"--version", nvidiaDRAHelmChartVer,
		"--create-namespace",
		"--namespace", nvidiaDRANamespace,
		"--set", "resources.gpus.enabled=true",
		"--set", "gpuResourcesEnabledOverride=true",
		"--timeout", "5m",
	}
	if *acceleratorDraDriverImage != "" {
		repo, tag := common.SplitImageRepoTag(*acceleratorDraDriverImage)
		args = append(args,
			"--set", fmt.Sprintf("image.repository=%s", repo),
			"--set", fmt.Sprintf("image.tag=%s", tag),
		)
	}
	log.Printf("[INFO] Installing NVIDIA DRA driver via Helm: helm %s", strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, "helm", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return ctx, fmt.Errorf("helm install nvidia-dra-driver-gpu failed: %w", err)
	}
	log.Println("NVIDIA DRA driver Helm release installed successfully.")
	return ctx, nil
}

// uninstallNvidiaDRADriverHelm uninstalls the NVIDIA DRA driver Helm release.
func uninstallNvidiaDRADriverHelm(ctx context.Context, config *envconf.Config) (context.Context, error) {
	args := []string{
		"uninstall", nvidiaDRAHelmReleaseName,
		"--namespace", nvidiaDRANamespace,
	}
	log.Printf("[INFO] Uninstalling NVIDIA DRA driver Helm release: helm %s", strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, "helm", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("[WARN] helm uninstall nvidia-dra-driver-gpu failed (may already be removed): %v", err)
	}
	return ctx, nil
}

func waitForNvidiaDRADriverReady(ctx context.Context, config *envconf.Config) (context.Context, error) {
	ds := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "nvidia-dra-driver-gpu-kubelet-plugin", Namespace: nvidiaDRANamespace},
	}
	err := wait.For(
		fwext.NewConditionExtension(config.Client().Resources()).DaemonSetReady(&ds),
		wait.WithTimeout(5*time.Minute),
		wait.WithContext(ctx),
	)
	if err != nil {
		return ctx, fmt.Errorf("nvidia-dra-driver daemonset is not ready: %w", err)
	}
	log.Println("nvidia-dra-driver daemonset is ready.")
	return ctx, nil
}

func TestMain(m *testing.M) {
	nodeType = flag.String("nodeType", "", "instance type for the cluster (e.g. p5.48xlarge)")
	rdmaDeviceDraDriverImage = flag.String("rdmaDeviceDraDriverImage", "", "container image for the dranet DRA driver")
	acceleratorDraDriverImage = flag.String("acceleratorDraDriverImage", "", "container image for the NVIDIA DRA driver")
	containerTestImage = flag.String("containerTestImage", "", "container image for the NCCL test workload")

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
				rctManifests, err := common.LoadRCTManifests(rctsFS, filepath.Join("rcts", topo.RCTSubDir))
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

			// Label all nodes with nvidia.com/gpu.present=true.
			g.Go(func() error {
				return labelNodesGPUPresent(gctx)
			})

			// Add NVIDIA Helm repo and install NVIDIA DRA driver.
			g.Go(func() error {
				_, err := installNvidiaDRADriverHelm(gctx, config)
				return err
			})

			if err := g.Wait(); err != nil {
				return ctx, err
			}
			return ctx, nil
		},
		waitForNvidiaDRADriverReady,
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			var err error
			clientset, err = kubernetes.NewForConfig(config.Client().RESTConfig())
			if err != nil {
				return ctx, err
			}
			nodeCount, err = common.CountNodesByType(ctx, clientset, *nodeType)
			return ctx, err
		},
	}
	testenv.Setup(setUpFunctions...)

	testenv.Finish(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			// Uninstall NVIDIA DRA driver Helm release first.
			ctx, _ = uninstallNvidiaDRADriverHelm(ctx, config)
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
