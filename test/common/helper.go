//go:build e2e

package common

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"strings"
	"time"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// RenderCloudWatchAgentManifest renders the CloudWatch Agent manifest with dynamic dimensions
func RenderCloudWatchAgentManifest(cloudWatchAgentManifest []byte, metricDimensions map[string]string) ([]byte, error) {
	var keys []string
	for key := range metricDimensions {
		keys = append(keys, `"`+key+`"`)
	}
	dimensionsStr := strings.Join(keys, ", ")
	return fwext.RenderManifests(cloudWatchAgentManifest, map[string]interface{}{
		"MetricDimensions": metricDimensions,
		"DimensionKeys":    template.HTML(dimensionsStr),
	})
}

// DeployDaemonSet returns a function to deploy and wait for a DaemonSet to be ready
func DeployDaemonSet(name, namespace string) env.Func {
	return func(ctx context.Context, config *envconf.Config) (context.Context, error) {
		log.Printf("Waiting for %s daemonset to be ready.", name)
		daemonset := appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		}
		err := wait.For(
			fwext.NewConditionExtension(config.Client().Resources()).DaemonSetReady(&daemonset),
			wait.WithTimeout(5*time.Minute),
			wait.WithContext(ctx),
		)
		if err != nil {
			// Do not fail test for optional daemonset failures
			if name == "cwagent" || name == "dcgm-exporter" {
				log.Printf("Warning: %s daemonset is not ready: %v", name, err)
				return ctx, nil
			}
			return ctx, fmt.Errorf("%s daemonset is not ready: %w", name, err)
		}
		log.Printf("%s daemonset is ready.", name)
		return ctx, nil
	}
}
