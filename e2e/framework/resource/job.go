package resource

import (
	"context"
	"time"

	"github.com/aws/aws-k8s-tester/e2e/framework/utils"
	batch_v1 "k8s.io/api/batch/v1"
	core_v1 "k8s.io/api/core/v1"
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
func isJobFinished(j *batch_v1.Job) bool {
	for _, c := range j.Status.Conditions {
		if (c.Type == batch_v1.JobComplete || c.Type == batch_v1.JobFailed) && c.Status == core_v1.ConditionTrue {
			return true
		}
	}
	return false
}

func (m *JobManager) WaitJobComplete(ctx context.Context, job *batch_v1.Job) (*batch_v1.Job, error) {
	var (
		observedJob *batch_v1.Job
		err         error
	)
	return observedJob, wait.PollImmediateUntil(utils.PollIntervalShort, func() (bool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		observedJob, err = m.cs.BatchV1().Jobs(job.Namespace).Get(ctx, job.Name, metav1.GetOptions{})
		cancel()
		if err != nil {
			return false, err
		}
		return isJobFinished(observedJob), nil
	}, ctx.Done())
}
