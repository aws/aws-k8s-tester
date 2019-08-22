package resource

import (
	"context"

	"github.com/aws/aws-k8s-tester/e2e/framework/utils"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

type JobManager struct {
	cs kubernetes.Interface
}

func NewJobManager(cs kubernetes.Interface) *JobManager {
	return &JobManager{
		cs: cs,
	}
}

// TODO return c.Type so we know if the job failed or finished happily
func isJobFinished(j *batchv1.Job) bool {
	for _, c := range j.Status.Conditions {
		if (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func (m *JobManager) WaitJobComplete(ctx context.Context, job *batchv1.Job) (*batchv1.Job, error) {
	var (
		observedJob *batchv1.Job
		err         error
	)
	return observedJob, wait.PollImmediateUntil(utils.PollIntervalShort, func() (bool, error) {
		observedJob, err = m.cs.BatchV1().Jobs(job.Namespace).Get(job.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return isJobFinished(observedJob), nil
	}, ctx.Done())
}
