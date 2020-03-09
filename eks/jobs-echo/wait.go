// Package jobsecho creates Job objects in Kubernetes.
package jobsecho

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

func waitJobs(
	lg *zap.Logger,
	stopc chan struct{},
	sigc chan os.Signal,
	clientSet *clientset.Clientset,
	timeout time.Duration,
	interval time.Duration,
	namespace string,
	jobName string,
	targets int,
	fieldSelector string,
	desiredPodPhase v1.PodPhase,
) (pods []v1.Pod, err error) {
	lg.Info("waiting Pod",
		zap.String("namespace", namespace),
		zap.String("job-name", jobName),
		zap.String("field-selector", fieldSelector),
	)
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < timeout {
		select {
		case <-stopc:
			return nil, errors.New("Pod polling aborted")
		case sig := <-sigc:
			return nil, fmt.Errorf("received os signal %v", sig)
		case <-time.After(interval):
		}

		// https://github.com/kubernetes/kubernetes/blob/d379ab2697251334774b7bd6f41b26cf39de470d/pkg/apis/batch/v1/conversion.go#L30-L41
		jobs, err := clientSet.
			CoreV1().
			Pods(namespace).
			List(metav1.ListOptions{
				FieldSelector: fieldSelector,
			})
		if err != nil {
			lg.Warn("failed to list Pod", zap.Error(err))
			continue
		}
		pods = jobs.Items
		if len(pods) == 0 {
			lg.Warn("got an empty list of Pod",
				zap.String("namespace", namespace),
				zap.String("job-name", jobName),
				zap.String("field-selector", fieldSelector),
			)
			continue
		}

		count := 0
		for _, item := range pods {
			jv, ok := item.Labels["job-name"]
			match := ok && jv == jobName
			if !match {
				match = strings.HasPrefix(item.Name, jobName)
			}
			if !match {
				continue
			}
			if item.Status.Phase != desiredPodPhase {
				continue
			}
			count++
		}
		if count == targets {
			lg.Info("found all targets", zap.Int("target", targets))
			break
		}

		lg.Info("polling",
			zap.String("namespace", namespace),
			zap.String("job-name", jobName),
			zap.Int("count", count),
			zap.Int("target", targets),
		)
	}

	return pods, nil
}
