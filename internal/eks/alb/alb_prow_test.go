package alb

import (
	"fmt"
	"testing"
)

func TestALBProw(t *testing.T) {
	s, err := CreateProwJobYAML(ConfigProwJobYAML{
		GINKGO_TIMEOUT:                                 "10h",
		GINKGO_VERBOSE:                                 "true",
		AWSTESTER_EKS_WAIT_BEFORE_DOWN:                 "1m",
		AWSTESTER_EKS_DOWN:                             "true",
		AWSTESTER_EKS_AWSTESTER_IMAGE:                  "ACCOUNT-ID.dkr.ecr.us-west-2.amazonaws.com/awstester:latest",
		AWSTESTER_EKS_WORKER_NODE_INSTANCE_TYPE:        "m3.xlarge",
		AWSTESTER_EKS_WORKER_NODE_ASG_MIN:              "1",
		AWSTESTER_EKS_WORKER_NODE_ASG_MAX:              "1",
		AWSTESTER_EKS_ALB_ENABLE:                       "true",
		AWSTESTER_EKS_ALB_TARGET_TYPE:                  "instance",
		AWSTESTER_EKS_ALB_TEST_MODE:                    "nginx",
		AWSTESTER_EKS_ALB_TEST_SCALABILITY:             "true",
		AWSTESTER_EKS_ALB_TEST_SERVER_REPLICAS:         "1",
		AWSTESTER_EKS_ALB_TEST_SERVER_ROUTES:           "1",
		AWSTESTER_EKS_ALB_TEST_CLIENTS:                 "200",
		AWSTESTER_EKS_ALB_TEST_CLIENT_REQUESTS:         "20000",
		AWSTESTER_EKS_ALB_TEST_RESPONSE_SIZE:           "40960",
		AWSTESTER_EKS_ALB_TEST_CLIENT_ERROR_THRESHOLD:  "500",
		AWSTESTER_EKS_ALB_TEST_EXPECT_QPS:              "100",
		AWSTESTER_EKS_ALB_ALB_INGRESS_CONTROLLER_IMAGE: "quay.io/coreos/alb-ingress-controller:1.0-beta.7",
	})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(s))
}
