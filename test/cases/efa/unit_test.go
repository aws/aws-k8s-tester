//go:build e2e

package efa

import (
	"context"
	_ "embed"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-k8s-tester/internal/e2e"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func generateUnitTestManifest(node corev1.Node, testIndex int) corev1.Pod {
	efaAllocatable := fmt.Sprint(getEfaCapacity(node))
	efaResourceQuantity := resource.MustParse(efaAllocatable)
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("efa-unit-%d", testIndex),
			Namespace: TEST_NAMESPACE_NAME,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: v1.RestartPolicyOnFailure,
			// TODO: centralize re-usable logic for pod spec fkormatting
			Affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "kubernetes.io/hostname",
										Operator: "In",
										Values: []string{
											node.Name,
										},
									},
								},
							},
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:    "unit-test",
					Image:   aws.ToString(testImage),
					Command: []string{"./scripts/unit-test.sh"},
					Env: []v1.EnvVar{
						{
							Name:  "EXPECTED_EFA_DEVICE_COUNT",
							Value: efaAllocatable,
						},
						{
							Name:  "EC2_INSTANCE_TYPE",
							Value: node.Labels["node.kubernetes.io/instance-type"],
						},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							EFA_RESOURCE_NAME: efaResourceQuantity,
						},
						Limits: corev1.ResourceList{
							EFA_RESOURCE_NAME: efaResourceQuantity,
						},
					},
				},
			},
		},
	}
}

func getUnitTestPodManifests(ctx context.Context, config *envconf.Config) ([]corev1.Pod, error) {
	var podManifests []corev1.Pod
	efaNodes, err := getEfaNodes(ctx, config)
	if err != nil {
		return []corev1.Pod{}, err
	}

	for nodeIndex, node := range efaNodes {
		podManifests = append(podManifests, generateUnitTestManifest(node, nodeIndex))
	}

	return podManifests, err
}

func TestUnit(t *testing.T) {
	var err error
	var pods []corev1.Pod
	unit := features.New("unit").
		WithLabel("suite", "efa").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			pods, err = getUnitTestPodManifests(ctx, cfg)
			if err != nil {
				t.Fatalf("Failed to generate unit test manifests: %v", err)
			}

			for _, pod := range pods {
				assert.NoError(t, cfg.Client().Resources().Create(ctx, &pod))
			}

			return ctx
		}).
		Assess("Unit test succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			suiteCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
			defer cancel()
			for _, pod := range pods {
				assert.NoError(t, wait.For(conditions.New(cfg.Client().Resources()).PodPhaseMatch(&pod, v1.PodSucceeded),
					wait.WithContext(suiteCtx),
				))
			}

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			for _, pod := range pods {
				podLogs, err := e2e.ReadPodLogs(ctx, cfg.Client().RESTConfig(), pod.Namespace, pod.Name, pod.Spec.Containers[0].Name)
				if err != nil {
					t.Logf("Could not get logs for pod %q", pod.Name)
				} else {
					t.Logf("Logs for pod %q\n%s", pod.Name, podLogs)
				}
			}

			for _, pod := range pods {
				assert.NoError(t, cfg.Client().Resources().Delete(ctx, &pod))
			}
			return ctx
		}).
		Feature()
	testenv.Test(t, unit)
}
