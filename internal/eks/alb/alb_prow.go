package alb

import (
	"bytes"
	"html/template"
)

// CreateProwJobYAML returns Prow job configurations.
func CreateProwJobYAML(cfg ConfigProwJobYAML) (string, error) {
	tpl := template.Must(template.New("albProwTempl").Parse(albProwTempl))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, cfg); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ConfigProwJobYAML defines ALB Prow job configuration.
type ConfigProwJobYAML struct {
	GINKGO_TIMEOUT string
	GINKGO_VERBOSE string

	AWSTESTER_EKS_EMBEDDED         string
	AWSTESTER_EKS_WAIT_BEFORE_DOWN string
	AWSTESTER_EKS_DOWN             string

	AWSTESTER_EKS_AWSTESTER_IMAGE string

	AWSTESTER_EKS_WORKER_NODE_INSTANCE_TYPE string
	AWSTESTER_EKS_WORKER_NODE_ASG_MIN       string
	AWSTESTER_EKS_WORKER_NODE_ASG_MAX       string

	AWSTESTER_EKS_ALB_ENABLE                  string
	AWSTESTER_EKS_ALB_ENABLE_SCALABILITY_TEST string

	AWSTESTER_EKS_ALB_ALB_INGRESS_CONTROLLER_IMAGE string

	AWSTESTER_EKS_ALB_TARGET_TYPE string
	AWSTESTER_EKS_ALB_TEST_MODE   string

	AWSTESTER_EKS_ALB_TEST_SERVER_REPLICAS        string
	AWSTESTER_EKS_ALB_TEST_SERVER_ROUTES          string
	AWSTESTER_EKS_ALB_TEST_CLIENTS                string
	AWSTESTER_EKS_ALB_TEST_CLIENT_REQUESTS        string
	AWSTESTER_EKS_ALB_TEST_RESPONSE_SIZE          string
	AWSTESTER_EKS_ALB_TEST_CLIENT_ERROR_THRESHOLD string
	AWSTESTER_EKS_ALB_TEST_EXPECT_QPS             string
}
