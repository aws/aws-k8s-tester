//go:build e2e

package netpol

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

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
