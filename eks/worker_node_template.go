package eks

import (
	"os"

	"github.com/aws/aws-k8s-tester/pkg/httputil"
	"go.uber.org/zap"
)

func createWorkerNodeTemplateFromURL(lg *zap.Logger) (string, error) {
	d, err := httputil.Download(lg, os.Stdout, workerNodeStackTemplateURL)
	if err != nil {
		return "", nil
	}
	return string(d), nil
}

// https://docs.aws.amazon.com/eks/latest/userguide/launch-workers.html
// https://github.com/awslabs/amazon-eks-ami/blob/master/amazon-eks-nodegroup.yaml
const workerNodeStackTemplateURL = "https://raw.githubusercontent.com/awslabs/amazon-eks-ami/master/amazon-eks-nodegroup.yamll"
