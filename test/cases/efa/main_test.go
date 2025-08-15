//go:build e2e

package efa

import (
	"context"
	_ "embed"
	"flag"
	"log"
	"os"
	"os/signal"
	"testing"
	"time"

	"github.com/aws/aws-k8s-tester/internal/e2e"
	"github.com/aws/aws-k8s-tester/test/manifests"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

func getTestNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: TEST_NAMESPACE_NAME,
		},
	}
}

func deployEFAPlugin(ctx context.Context, config *envconf.Config) (context.Context, error) {
	err := e2e.ApplyManifests(config.Client().RESTConfig(), manifests.EfaDevicePluginManifest)
	if err != nil {
		return ctx, err
	}
	efaDS := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "aws-efa-k8s-device-plugin-daemonset", Namespace: "kube-system"},
	}
	err = wait.For(e2e.NewConditionExtension(config.Client().Resources()).DaemonSetReady(&efaDS),
		wait.WithContext(ctx),
		wait.WithTimeout(5*time.Minute),
	)
	if err != nil {
		return ctx, err
	}

	return ctx, nil
}

func TestMain(m *testing.M) {
	testImage = flag.String("testImage", "", "container image to use for tests")
	pingPongSize = flag.String("pingPongSize", "all", "sizes to use for ping pong")
	pingPongIters = flag.Int("pingPongIters", 10000, "number of iterations to use for ping pong")
	pingPongDeadlineSeconds = flag.Int("pingPongDeadlineSeconds", 120, "maximum run time for a ping pong attempt")
	nodeType = flag.String("nodeType", "", "instance type to target for tests")
	expectedEFADeviceCount = flag.Int("expectedEFADeviceCount", -1, "expected number of efa devices for the target nodes")
	verbose = flag.Bool("verbose", true, "use verbose mode for tests")

	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("failed to initialize test environment: %v", err)
	}

	if *testImage == "" {
		log.Fatal("--testImage must be set, use https://github.com/aws/aws-k8s-tester/blob/main/test/efa/Dockerfile to build the image")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	timedCtx, cancel := context.WithTimeout(ctx, 55*time.Minute)
	defer cancel()

	testenv = env.NewWithConfig(cfg)
	testenv = testenv.WithContext(timedCtx)

	ec2Client = e2e.NewEC2Client()

	testenv.Setup(
		deployEFAPlugin,
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			select {
			case <-ctx.Done():
			// Cooldown to let device plugin update node object with resources
			case <-time.After(15 * time.Second):
			}

			return ctx, cfg.Client().Resources().Create(ctx, getTestNamespace())
		},
	)

	testenv.Finish(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			cfg.Client().Resources().Delete(context.TODO(), getTestNamespace())
			err := e2e.DeleteManifests(cfg.Client().RESTConfig(), manifests.EfaDevicePluginManifest)
			if err != nil {
				return ctx, err
			}
			return ctx, nil
		},
	)

	os.Exit(testenv.Run(m))
}
