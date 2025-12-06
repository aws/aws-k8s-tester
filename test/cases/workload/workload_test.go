//go:build e2e

package workload

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	"github.com/aws/smithy-go/ptr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func createWorkloadJob(name, image, command string, resources map[string]string, timeout time.Duration) *batchv1.Job {
	container := corev1.Container{
		Name:            name,
		Image:           image,
		ImagePullPolicy: corev1.PullAlways,
		Resources:       buildResourceRequirements(resources),
	}

	// Override entrypoint if command is provided
	if command != "" {
		container.Command = strings.Fields(command)
	}

	podSpec := corev1.PodSpec{
		RestartPolicy:         corev1.RestartPolicyNever,
		ActiveDeadlineSeconds: ptr.Int64(int64(timeout.Seconds())),
		Containers:            []corev1.Container{container},
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: corev1.NamespaceDefault,
			Labels:    map[string]string{"app": name},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.Int32(4),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: podSpec,
			},
		},
	}

	return job
}

func buildResourceRequirements(resources map[string]string) corev1.ResourceRequirements {
	if len(resources) == 0 {
		return corev1.ResourceRequirements{}
	}
	rl := make(corev1.ResourceList)
	for name, qty := range resources {
		rl[corev1.ResourceName(name)] = resource.MustParse(qty)
	}
	return corev1.ResourceRequirements{Limits: rl, Requests: rl}
}

func parseResources(resourcesJSON string) (map[string]string, error) {
	if resourcesJSON == "" {
		return nil, nil
	}
	var resources map[string]string
	if err := json.Unmarshal([]byte(resourcesJSON), &resources); err != nil {
		return nil, err
	}
	for name, qty := range resources {
		if q, err := resource.ParseQuantity(qty); err != nil || q.IsZero() {
			delete(resources, name)
		}
	}
	return resources, nil
}

func TestWorkload(t *testing.T) {
	name := ptr.ToString(workloadTestName)
	image := ptr.ToString(workloadTestImage)
	command := ptr.ToString(workloadTestCommand)
	timeout := ptr.ToDuration(workloadTestTimeout)

	if name == "" {
		t.Fatal("workloadTestName must be set to run the test")
	}
	if image == "" {
		t.Fatal("workloadTestImage must be set to run the test")
	}

	resources, err := parseResources(ptr.ToString(workloadTestResources))
	if err != nil {
		t.Fatalf("Failed to parse workloadTestResources: %v", err)
	}

	feature := features.New(name).WithLabel("suite", "workload")
	if _, ok := resources["aws.amazon.com/neuron"]; ok {
		feature = feature.WithLabel("hardware", "neuron")
	}
	if _, ok := resources["nvidia.com/gpu"]; ok {
		feature = feature.WithLabel("hardware", "gpu")
	}

	workload := feature.Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		job := createWorkloadJob(name, image, command, resources, timeout)
		if len(resources) > 0 {
			t.Logf("Creating %s job with resources: %v", name, resources)
		} else {
			t.Logf("Creating %s job", name)
		}
		if err := cfg.Client().Resources().Create(ctx, job); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s job created successfully", name)
		return ctx
	}).
		Assess("Job succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: corev1.NamespaceDefault},
			}
			t.Logf("Waiting for %s job to complete", name)
			err := wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).JobSucceeded(job),
				wait.WithContext(ctx),
				wait.WithTimeout(timeout),
			)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			log, err := fwext.GetJobLogs(cfg.Client().RESTConfig(), &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: corev1.NamespaceDefault},
			})
			if err != nil {
				t.Error(err)
			}
			t.Logf("Test log for %s:", name)
			t.Log(log)
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: corev1.NamespaceDefault},
			}
			if err := cfg.Client().Resources().Delete(ctx, job, func(do *metav1.DeleteOptions) {
				policy := metav1.DeletePropagationBackground
				do.PropagationPolicy = &policy
			}); err != nil {
				t.Error(err)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, workload)
}
