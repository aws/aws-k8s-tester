package utils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUpdateDaemonSetEnvVars(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "daemonset",
			Namespace: "ns",
		},
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "container1",
							Env: []corev1.EnvVar{
								{
									Name:  "A",
									Value: "ant",
								},
							},
						},
						{
							Name: "container2",
							Env: []corev1.EnvVar{
								{
									Name:  "A",
									Value: "ant",
								},
								{
									Name:  "B",
									Value: "bug",
								},
								{
									Name:  "D",
									Value: "dog",
								},
							},
						},
					},
				},
			},
		},
	}

	envs := []corev1.EnvVar{
		{Name: "A", Value: "ape"},
		{Name: "B", Value: "bat"},
		{Name: "C", Value: "cat"},
	}

	assert.Equal(t, len(ds.Spec.Template.Spec.Containers), 2)
	assert.Equal(t, len(ds.Spec.Template.Spec.Containers[0].Env), 1)
	assert.Equal(t, len(ds.Spec.Template.Spec.Containers[1].Env), 3)

	assert.Equal(t, ds.Spec.Template.Spec.Containers[0].Env[0].Name, "A")
	assert.Equal(t, ds.Spec.Template.Spec.Containers[0].Env[0].Value, "ant")

	assert.Equal(t, ds.Spec.Template.Spec.Containers[1].Env[0].Name, "A")
	assert.Equal(t, ds.Spec.Template.Spec.Containers[1].Env[0].Value, "ant")
	assert.Equal(t, ds.Spec.Template.Spec.Containers[1].Env[1].Name, "B")
	assert.Equal(t, ds.Spec.Template.Spec.Containers[1].Env[1].Value, "bug")
	assert.Equal(t, ds.Spec.Template.Spec.Containers[1].Env[2].Name, "D")
	assert.Equal(t, ds.Spec.Template.Spec.Containers[1].Env[2].Value, "dog")

	fmt.Printf("container1 before %v\n", ds.Spec.Template.Spec.Containers[0].Env)
	fmt.Printf("container2 before %v\n", ds.Spec.Template.Spec.Containers[1].Env)

	updateDaemonSetEnvVars(ds, envs)

	fmt.Printf("container1 after: %v\n", ds.Spec.Template.Spec.Containers[0].Env)
	fmt.Printf("container2 after: %v\n", ds.Spec.Template.Spec.Containers[1].Env)

	assert.Equal(t, len(ds.Spec.Template.Spec.Containers[0].Env), 3)
	assert.Equal(t, len(ds.Spec.Template.Spec.Containers[1].Env), 4)

	assert.Equal(t, ds.Spec.Template.Spec.Containers[0].Env[0].Name, "A")
	assert.Equal(t, ds.Spec.Template.Spec.Containers[0].Env[0].Value, "ape")
	assert.Equal(t, ds.Spec.Template.Spec.Containers[0].Env[1].Name, "B")
	assert.Equal(t, ds.Spec.Template.Spec.Containers[0].Env[1].Value, "bat")
	assert.Equal(t, ds.Spec.Template.Spec.Containers[0].Env[2].Name, "C")
	assert.Equal(t, ds.Spec.Template.Spec.Containers[0].Env[2].Value, "cat")

	assert.Equal(t, ds.Spec.Template.Spec.Containers[1].Env[0].Name, "A")
	assert.Equal(t, ds.Spec.Template.Spec.Containers[1].Env[0].Value, "ape")
	assert.Equal(t, ds.Spec.Template.Spec.Containers[1].Env[1].Name, "B")
	assert.Equal(t, ds.Spec.Template.Spec.Containers[1].Env[1].Value, "bat")
	assert.Equal(t, ds.Spec.Template.Spec.Containers[1].Env[2].Name, "D")
	assert.Equal(t, ds.Spec.Template.Spec.Containers[1].Env[2].Value, "dog")
	assert.Equal(t, ds.Spec.Template.Spec.Containers[1].Env[3].Name, "C")
	assert.Equal(t, ds.Spec.Template.Spec.Containers[1].Env[3].Value, "cat")
}

func TestGetNodeInternalIP(t *testing.T) {
	n := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ip-192-168-118-81.us-west-2.compute.internal",
		},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{
					Address: "192.168.118.81",
					Type:    corev1.NodeInternalIP,
				},
				{
					Address: "54.189.164.61",
					Type:    corev1.NodeExternalIP,
				},
				{
					Address: "ip-192-168-118-81.us-west-2.compute.internal",
					Type:    corev1.NodeInternalDNS,
				},
				{
					Address: "ec2-54-189-164-61.us-west-2.compute.amazonaws.com",
					Type:    corev1.NodeExternalDNS,
				},
				{
					Address: "ip-192-168-118-81.us-west-2.compute.internal",
					Type:    corev1.NodeHostName,
				},
			},
		},
	}

	ip, err := GetNodeInternalIP(n)
	assert.NoError(t, err)
	assert.Equal(t, ip, "192.168.118.81")
}
