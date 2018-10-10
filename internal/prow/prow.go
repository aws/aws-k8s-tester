package prow

import (
	"fmt"
	"strings"
)

// TMP_DIR/test-infra-master/config/jobs/kubernetes-sigs/cluster-api-provider-openstack/cluster-api-provider-openstack-presubmits.yaml
// "https://raw.githubusercontent.com/kubernetes/test-infra/master/config/jobs/kubernetes-sigs/cluster-api-provider-aws/cluster-api-provider-aws-presubmits.yaml"
// "https://raw.githubusercontent.com/kubernetes/test-infra/master/prow/config.yaml"
func getProwURL(prowPath string) string {
	return fmt.Sprintf(
		"https://raw.githubusercontent.com/kubernetes/test-infra/master/%s",
		strings.Split(prowPath, "/test-infra-master/")[1],
	)
}

func getProwStatusURL(name string) string {
	// e.g. https://k8s-gubernator.appspot.com/builds/kubernetes-jenkins/pr-logs/pull/batch/pull-kubernetes-e2e-gke/
	return fmt.Sprintf("https://k8s-gubernator.appspot.com/builds/kubernetes-jenkins/pr-logs/pull/batch/%s/", name)
}
