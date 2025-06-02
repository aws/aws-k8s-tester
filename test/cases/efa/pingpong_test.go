//go:build e2e

package efa

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/aws/aws-k8s-tester/internal/e2e"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PING_PONG_SERVICE_NAME = "pingpong-service"
	SERVER_POD_NAME        = "pingpong-server"
	CLIENT_POD_NAME        = "pingpong-client"
	PINGPONG_COMMAND       = "fi_pingpong"
)

func getPingPongPodName(server bool) string {
	if server {
		return SERVER_POD_NAME
	} else {
		return CLIENT_POD_NAME
	}
}

func getPingPongArgs(server bool) (args []string) {
	args = []string{"-S", aws.ToString(pingPongSize), "-I", fmt.Sprint(aws.ToInt(pingPongIters)), "-p", "efa"}
	if aws.ToBool(verbose) {
		args = append(args, "-v")
	}
	if !server {
		args = append(args, fmt.Sprintf("%s.%s", SERVER_POD_NAME, PING_PONG_SERVICE_NAME))
	}
	return
}

func getPingPongResourceLabels(server bool) map[string]string {
	return map[string]string{
		"test-suite":      "pingpong",
		"pingpong-server": fmt.Sprint(server),
	}
}

func generatePingPongServiceManifest() corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PING_PONG_SERVICE_NAME,
			Namespace: TEST_NAMESPACE_NAME,
		},
		Spec: v1.ServiceSpec{
			Selector:  getPingPongResourceLabels(true),
			ClusterIP: "None",
		},
	}
}

func generatePingPongPodManifest(server bool, node corev1.Node) corev1.Pod {
	efaResourceQuantity := resource.MustParse(fmt.Sprint(getEfaCapacity(node)))
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getPingPongPodName(server),
			Namespace: TEST_NAMESPACE_NAME,
			Labels:    getPingPongResourceLabels(server),
		},
		Spec: corev1.PodSpec{
			Hostname:      getPingPongPodName(server),
			Subdomain:     PING_PONG_SERVICE_NAME,
			RestartPolicy: v1.RestartPolicyOnFailure,
			// TODO: centralize re-usable logic for pod spec formatting
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
					Name:    "pingpong",
					Image:   aws.ToString(testImage),
					Command: []string{"timeout", fmt.Sprintf("%ds", aws.ToInt(pingPongDeadlineSeconds)), PINGPONG_COMMAND},
					Args:    getPingPongArgs(server),
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

func getPingPongPods(ctx context.Context, config *envconf.Config) (corev1.Pod, corev1.Pod, error) {
	efaNodes, err := getEfaNodes(ctx, config)
	if err != nil {
		return corev1.Pod{}, corev1.Pod{}, err
	}

	if len(efaNodes) < 2 {
		return corev1.Pod{}, corev1.Pod{}, fmt.Errorf("need at least 2 nodes with EFA capacity, got %d", len(efaNodes))
	}

	serverNode := efaNodes[0]
	log.Printf("[INFO] Using node %s (type: %s), as server", serverNode.Name, serverNode.Labels["node.kubernetes.io/instance-type"])

	clientNode := efaNodes[1]
	log.Printf("[INFO] Using node %s (type: %s), as client", clientNode.Name, clientNode.Labels["node.kubernetes.io/instance-type"])

	return generatePingPongPodManifest(true, serverNode), generatePingPongPodManifest(false, clientNode), nil
}

func TestPingPong(t *testing.T) {
	var err error
	var pingPongService corev1.Service
	var client, server corev1.Pod
	pingpong := features.New("pingpong").
		WithLabel("suite", "efa").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			pingPongService = generatePingPongServiceManifest()
			client, server, err = getPingPongPods(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}

			assert.NoError(t, cfg.Client().Resources().Create(ctx, &pingPongService))
			assert.NoError(t, cfg.Client().Resources().Create(ctx, &server))
			assert.NoError(t, cfg.Client().Resources().Create(ctx, &client))
			return ctx
		}).
		Assess("Pingpong between nodes succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			assert.NoError(t, wait.For(conditions.New(cfg.Client().Resources()).PodPhaseMatch(&server, v1.PodSucceeded),
				wait.WithTimeout(15*time.Minute),
				wait.WithContext(ctx),
			))

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			serverPodLogs, err := e2e.ReadPodLogs(ctx, cfg.Client().RESTConfig(), server.Namespace, server.Name, server.Spec.Containers[0].Name)
			if err != nil {
				t.Logf("Could not get pods for server")
			}
			t.Logf("Logs for server\n%s", serverPodLogs)

			assert.NoError(t, cfg.Client().Resources().Delete(ctx, &pingPongService))
			assert.NoError(t, cfg.Client().Resources().Delete(ctx, &server))
			assert.NoError(t, cfg.Client().Resources().Delete(ctx, &client))
			return ctx
		}).
		Feature()
	testenv.Test(t, pingpong)
}
