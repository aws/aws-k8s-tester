package ulimit

import (
	"bytes"
	"context"
	_ "embed"
	fwext "github.com/aws/aws-k8s-tester/e2e2/internal/framework_extensions"
	"io"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"log"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"strings"
	"testing"
	"time"
)

var (
	//go:embed manifests/ulimit.yaml
	ulimitManifest []byte

	expectedResourceLimit = map[string]string{
		"-R": "unlimited",
		"-c": "unlimited",
		"-d": "unlimited",
		"-e": "0",
		"-f": "unlimited",
		"-i": "30446",
		"-l": "unlimited",
		"-m": "unlimited",
		"-n": "1048576",
		"-p": "8",
		"-q": "819200",
		"-r": "0",
		"-s": "10240",
		"-t": "unlimited",
		"-u": "unlimited",
		"-v": "unlimited",
		"-x": "unlimited",
	}
)

func TestKubernetes(t *testing.T) {
	f1 := features.New("ulimit pod").
		WithLabel("type", "ulimit").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			err := fwext.ApplyManifests(cfg.Client().RESTConfig(), ulimitManifest)
			if err != nil {
				t.Fatalf("failed to apply manifests: %v", err)
			}
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "ulimit", Namespace: "default"},
			}
			err = wait.For(conditions.New(cfg.Client().Resources()).ResourceMatch(pod, containerTerminated),
				wait.WithTimeout(time.Minute*5))
			if err != nil {
				t.Fatalf("encounter error when waiting for container finished running commands: %v", err)
			}
			return ctx
		}).
		Assess("Use default resources limit", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := kubernetes.NewForConfig(cfg.Client().RESTConfig())
			if err != nil {
				t.Fatal(err)
			}
			tailLine := int64(10000)
			podLogOptions := corev1.PodLogOptions{
				Container: "al2023",
				TailLines: &tailLine,
			}
			req := client.CoreV1().Pods("default").GetLogs("ulimit", &podLogOptions)
			logs, err := req.Stream(ctx)
			if err != nil {
				log.Fatalf("error in opening stream: %v", err)
			}
			defer logs.Close()
			compareResourceLimitsWithExpectedValues(logs)
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			err := fwext.DeleteManifests(cfg.Client().RESTConfig(), ulimitManifest)
			if err != nil {
				log.Fatalf("failed to delete manifests: %v", err)
			}
			return ctx
		}).Feature()

	// test feature
	testenv.Test(t, f1)
}

func compareResourceLimitsWithExpectedValues(logs io.ReadCloser) {
	buf := new(bytes.Buffer)
	_, err := io.Copy(buf, logs)
	if err != nil {
		log.Fatalf("error in copy information from podLogs to buf: %v", err)
	}
	str := buf.String()

	lines := strings.Split(str, "\n")
	for _, line := range lines[:len(lines)-1] {
		info := strings.Split(line, " ")
		marker := getMarker(info[len(info)-2])
		value := info[len(info)-1]
		if expectedResourceLimit[marker] != value {
			log.Fatalf("resource limit doesn't match with the default value, limit we get %v, but default value is %v", line, expectedResourceLimit[marker])
		}
		log.Printf("resrouce limit fetched from ulimit: %v. Equal to the default value %v", line, expectedResourceLimit[marker])
	}
}

func containerTerminated(obj k8s.Object) bool {
	j := obj.(*corev1.Pod)
	containerTerminatedState := j.Status.ContainerStatuses[0].State.Terminated
	return containerTerminatedState.Reason == "Completed"
}

func getMarker(str string) string {
	startIndex := 0
	if str[:1] == "(" {
		startIndex = 1
	}
	return str[startIndex : len(str)-1]
}
