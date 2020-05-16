package eksconfig

import (
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/stretchr/testify/assert"
)

func TestEnv(t *testing.T) {
	cfg := NewDefault()
	defer func() {
		os.RemoveAll(cfg.ConfigPath)
		os.RemoveAll(cfg.KubectlCommandsOutputPath)
		os.RemoveAll(cfg.RemoteAccessCommandsOutputPath)
	}()

	os.Setenv("AWS_K8S_TESTER_EKS_KUBECTL_COMMANDS_OUTPUT_PATH", "hello-kubectl")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_KUBECTL_COMMANDS_OUTPUT_PATH")
	os.Setenv("AWS_K8S_TESTER_EKS_REMOTE_ACCESS_COMMANDS_OUTPUT_PATH", "hello-ssh")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_REMOTE_ACCESS_COMMANDS_OUTPUT_PATH")
	os.Setenv("AWS_K8S_TESTER_EKS_REGION", "us-east-1")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_REGION")
	os.Setenv("AWS_K8S_TESTER_EKS_LOG_LEVEL", "debug")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_LOG_LEVEL")
	os.Setenv("AWS_K8S_TESTER_EKS_KUBECTL_DOWNLOAD_URL", "https://amazon-eks.s3-us-west-2.amazonaws.com/1.11.5/2018-12-06/bin/linux/amd64/kubectl")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_KUBECTL_DOWNLOAD_URL")
	os.Setenv("AWS_K8S_TESTER_EKS_KUBECTL_PATH", "/tmp/aws-k8s-tester-test/kubectl")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_KUBECTL_PATH")
	os.Setenv("AWS_K8S_TESTER_EKS_KUBECONFIG_PATH", "/tmp/aws-k8s-tester/kubeconfig2")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_KUBECONFIG_PATH")
	os.Setenv("AWS_K8S_TESTER_EKS_ON_FAILURE_DELETE", "false")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ON_FAILURE_DELETE")
	os.Setenv("AWS_K8S_TESTER_EKS_ON_FAILURE_DELETE_WAIT_SECONDS", "780")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ON_FAILURE_DELETE_WAIT_SECONDS")
	os.Setenv("AWS_K8S_TESTER_EKS_COMMAND_AFTER_CREATE_CLUSTER", "echo hello1")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_COMMAND_AFTER_CREATE_CLUSTER")
	os.Setenv("AWS_K8S_TESTER_EKS_COMMAND_AFTER_CREATE_CLUSTER_TIMEOUT", "7m")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_COMMAND_AFTER_CREATE_CLUSTER_TIMEOUT")
	os.Setenv("AWS_K8S_TESTER_EKS_COMMAND_AFTER_CREATE_ADD_ONS", "echo hello2")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_COMMAND_AFTER_CREATE_ADD_ONS")
	os.Setenv("AWS_K8S_TESTER_EKS_COMMAND_AFTER_CREATE_ADD_ONS_TIMEOUT", "17m")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_COMMAND_AFTER_CREATE_ADD_ONS_TIMEOUT")
	os.Setenv("AWS_K8S_TESTER_EKS_S3_BUCKET_CREATE", `true`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_S3_BUCKET_CREATE")
	os.Setenv("AWS_K8S_TESTER_EKS_S3_BUCKET_CREATE_KEEP", `true`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_S3_BUCKET_CREATE_KEEP")
	os.Setenv("AWS_K8S_TESTER_EKS_S3_BUCKET_NAME", `my-bucket`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_S3_BUCKET_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_S3_BUCKET_LIFECYCLE_EXPIRATION_DAYS", `10`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_S3_BUCKET_LIFECYCLE_EXPIRATION_DAYS")
	os.Setenv("AWS_K8S_TESTER_EKS_CLIENTS", `333`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_CLIENTS")
	os.Setenv("AWS_K8S_TESTER_EKS_CLIENT_TIMEOUT", `10m`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_CLIENT_TIMEOUT")
	os.Setenv("AWS_K8S_TESTER_EKS_CLIENT_QPS", `99555.77`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_CLIENT_QPS")
	os.Setenv("AWS_K8S_TESTER_EKS_CLIENT_BURST", `177`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_CLIENT_BURST")

	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_VPC_CREATE", "false")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_VPC_CREATE")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_VPC_ID", "vpc-id")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_VPC_ID")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_VPC_CIDR", "my-cidr")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_VPC_CIDR")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_PUBLIC_SUBNET_CIDR_1", "public-cidr1")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_PUBLIC_SUBNET_CIDR_1")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_PUBLIC_SUBNET_CIDR_2", "public-cidr2")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_PUBLIC_SUBNET_CIDR_2")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_PUBLIC_SUBNET_CIDR_3", "public-cidr3")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_PUBLIC_SUBNET_CIDR_3")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_PRIVATE_SUBNET_CIDR_1", "private-cidr1")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_PRIVATE_SUBNET_CIDR_1")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_PRIVATE_SUBNET_CIDR_2", "private-cidr2")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_PRIVATE_SUBNET_CIDR_2")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_DHCP_OPTIONS_DOMAIN_NAME", `hello.com`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_DHCP_OPTIONS_DOMAIN_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_DHCP_OPTIONS_DOMAIN_NAME_SERVERS", `1.2.3.0,4.5.6.7`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_DHCP_OPTIONS_DOMAIN_NAME_SERVERS")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_TAGS", "to-delete=2019;hello-world=test")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_TAGS")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_REQUEST_HEADER_KEY", "eks-options")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_REQUEST_HEADER_KEY")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_REQUEST_HEADER_VALUE", "kubernetesVersion=1.11")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_REQUEST_HEADER_VALUE")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_RESOLVER_URL", "amazon.com")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_RESOLVER_URL")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_SIGNING_NAME", "a")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_SIGNING_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_ROLE_CREATE", "false")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_ROLE_CREATE")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_ROLE_ARN", "cluster-role-arn")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_ROLE_ARN")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_ROLE_SERVICE_PRINCIPALS", "eks.amazonaws.com,eks-beta-pdx.aws.internal,eks-dev.aws.internal")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_ROLE_SERVICE_PRINCIPALS")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_ROLE_MANAGED_POLICY_ARNS", "arn:aws:iam::aws:policy/AmazonEKSServicePolicy,arn:aws:iam::aws:policy/AmazonEKSClusterPolicy,arn:aws:iam::aws:policy/service-role/AmazonEC2RoleforSSM")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_ROLE_MANAGED_POLICY_ARNS")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_VERSION", "1.16")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_VERSION")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_ENCRYPTION_CMK_CREATE", "false")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_ENCRYPTION_CMK_CREATE")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_ENCRYPTION_CMK_ARN", "key-arn")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_ENCRYPTION_CMK_ARN")

	os.Setenv("AWS_K8S_TESTER_EKS_REMOTE_ACCESS_KEY_CREATE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_REMOTE_ACCESS_KEY_CREATE")
	os.Setenv("AWS_K8S_TESTER_EKS_REMOTE_ACCESS_KEY_NAME", "a")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_REMOTE_ACCESS_KEY_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_REMOTE_ACCESS_PRIVATE_KEY_PATH", "a")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_REMOTE_ACCESS_PRIVATE_KEY_PATH")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_FETCH_LOGS", "false")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_FETCH_LOGS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ROLE_CREATE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ROLE_CREATE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ROLE_NAME", "ng-role-name")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ROLE_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ROLE_ARN", "ng-role-arn")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ROLE_ARN")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ROLE_SERVICE_PRINCIPALS", "ec2.amazonaws.com,eks.amazonaws.com,hello.amazonaws.com")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ROLE_SERVICE_PRINCIPALS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ROLE_MANAGED_POLICY_ARNS", "a,b,c")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ROLE_MANAGED_POLICY_ARNS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ASGS", `{"ng-test-name-cpu":{"name":"ng-test-name-cpu","remote-access-user-name":"ec2-user","ami-type":"AL2_x86_64","image-id-ssm-parameter":"/aws/service/eks/optimized-ami/1.30/amazon-linux-2/recommended/image_id","asg-min-size":17,"kubelet-extra-args":"bbb qq","asg-max-size":99,"asg-desired-capacity":77,"instance-types":["type-cpu-2"],"volume-size":40},"ng-test-name-gpu":{"name":"ng-test-name-gpu","remote-access-user-name":"ec2-user","ami-type":"AL2_x86_64_GPU","asg-min-size":30,"asg-max-size":35,"asg-desired-capacity":34,"instance-types":["type-gpu-2"],"image-id":"my-gpu-ami","volume-size":500, "kubelet-extra-args":"aaa aa"}}`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ASGS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_LOGS_DIR", "a")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_LOGS_DIR")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_FETCH_LOGS", "false")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_FETCH_LOGS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ROLE_CREATE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ROLE_CREATE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ROLE_NAME", "mng-role-name")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ROLE_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ROLE_ARN", "mng-role-arn")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ROLE_ARN")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ROLE_SERVICE_PRINCIPALS", "ec2.amazonaws.com,eks.amazonaws.com,hello.amazonaws.com")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ROLE_SERVICE_PRINCIPALS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ROLE_MANAGED_POLICY_ARNS", "a,b,c")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ROLE_MANAGED_POLICY_ARNS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_REQUEST_HEADER_KEY", "a")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_REQUEST_HEADER_KEY")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_REQUEST_HEADER_VALUE", "b")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_REQUEST_HEADER_VALUE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_RESOLVER_URL", "a")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_RESOLVER_URL")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_SIGNING_NAME", "a")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_SIGNING_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_MNGS", `{"mng-test-name-cpu":{"name":"mng-test-name-cpu","tags":{"cpu":"hello-world"},"remote-access-user-name":"ec2-user","release-version":"test-ver-cpu","ami-type":"AL2_x86_64","asg-min-size":17,"asg-max-size":99,"asg-desired-capacity":77,"instance-types":["type-cpu-1","type-cpu-2"],"volume-size":40},"mng-test-name-gpu":{"name":"mng-test-name-gpu","remote-access-user-name":"ec2-user","tags":{"gpu":"hello-world"},"release-version":"test-ver-gpu","ami-type":"AL2_x86_64_GPU","asg-min-size":30,"asg-max-size":35,"asg-desired-capacity":34,"instance-types":["type-gpu-1","type-gpu-2"],"volume-size":500}}`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_MNGS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_LOGS_DIR", "a")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_LOGS_DIR")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_DEPLOYMENT_REPLICAS", "333")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_DEPLOYMENT_REPLICAS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_NAMESPACE", "test-namespace")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_NAMESPACE")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_DEPLOYMENT_REPLICAS_ALB", "333")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_DEPLOYMENT_REPLICAS_ALB")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_DEPLOYMENT_REPLICAS_2048", "555")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_DEPLOYMENT_REPLICAS_2048")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_NAMESPACE", "test-namespace")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_NAMESPACE")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOBS_PI_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOBS_PI_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOBS_PI_NAMESPACE", "hello1")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOBS_PI_NAMESPACE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOBS_PI_COMPLETES", "100")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOBS_PI_COMPLETES")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOBS_PI_PARALLELS", "10")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOBS_PI_PARALLELS")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOBS_ECHO_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOBS_ECHO_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOBS_ECHO_NAMESPACE", "hello2")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOBS_ECHO_NAMESPACE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOBS_ECHO_COMPLETES", "1000")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOBS_ECHO_COMPLETES")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOBS_ECHO_PARALLELS", "100")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOBS_ECHO_PARALLELS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOBS_ECHO_ECHO_SIZE", "10000")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOBS_ECHO_ECHO_SIZE")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_NAMESPACE", "hello3")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_NAMESPACE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_SCHEDULE", "*/1 * * * *")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_SCHEDULE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_COMPLETES", "100")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_COMPLETES")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_PARALLELS", "10")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_PARALLELS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_SUCCESSFUL_JOBS_HISTORY_LIMIT", "100")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_SUCCESSFUL_JOBS_HISTORY_LIMIT")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_FAILED_JOBS_HISTORY_LIMIT", "1000")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_FAILED_JOBS_HISTORY_LIMIT")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_ECHO_SIZE", "10000")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_ECHO_SIZE")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CSRS_LOCAL_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CSRS_LOCAL_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CSRS_LOCAL_INITIAL_REQUEST_CONDITION_TYPE", "Random")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CSRS_LOCAL_INITIAL_REQUEST_CONDITION_TYPE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CSRS_LOCAL_OBJECTS", "10000")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CSRS_LOCAL_OBJECTS")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_NAMESPACE", "csr-namespace")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_NAMESPACE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_INITIAL_REQUEST_CONDITION_TYPE", "Random")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_INITIAL_REQUEST_CONDITION_TYPE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_OBJECTS", "10000")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_OBJECTS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_REPOSITORY_ACCOUNT_ID", "uri")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_REPOSITORY_ACCOUNT_ID")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_REPOSITORY_NAME", "csrs-repo-name")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_REPOSITORY_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_REPOSITORY_IMAGE_TAG", "csrs-repo-image-tag")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_REPOSITORY_IMAGE_TAG")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFIG_MAPS_LOCAL_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFIG_MAPS_LOCAL_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFIG_MAPS_LOCAL_NAMESPACE", "config-map-namespace")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFIG_MAPS_LOCAL_NAMESPACE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFIG_MAPS_LOCAL_OBJECTS", "10000")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFIG_MAPS_LOCAL_OBJECTS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFIG_MAPS_LOCAL_OBJECT_SIZE", "555")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFIG_MAPS_LOCAL_OBJECT_SIZE")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFIG_MAPS_REMOTE_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFIG_MAPS_REMOTE_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFIG_MAPS_REMOTE_NAMESPACE", "config-map-namespace")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFIG_MAPS_REMOTE_NAMESPACE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFIG_MAPS_REMOTE_OBJECTS", "10000")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFIG_MAPS_REMOTE_OBJECTS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFIG_MAPS_REMOTE_OBJECT_SIZE", "555")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFIG_MAPS_REMOTE_OBJECT_SIZE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFIG_MAPS_REMOTE_REPOSITORY_ACCOUNT_ID", "uri")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFIG_MAPS_REMOTE_REPOSITORY_ACCOUNT_ID")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFIG_MAPS_REMOTE_REPOSITORY_NAME", "config-maps-repo-name")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFIG_MAPS_REMOTE_REPOSITORY_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFIG_MAPS_REMOTE_REPOSITORY_IMAGE_TAG", "config-maps-repo-image-tag")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFIG_MAPS_REMOTE_REPOSITORY_IMAGE_TAG")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_NAMESPACE", "hello")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_NAMESPACE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_OBJECTS", "5")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_OBJECTS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_OBJECT_SIZE", "10")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_OBJECT_SIZE")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_NAMESPACE", "hello")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_NAMESPACE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_OBJECTS", "5")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_OBJECTS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_OBJECT_SIZE", "10")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_OBJECT_SIZE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REPOSITORY_ACCOUNT_ID", "uri")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REPOSITORY_ACCOUNT_ID")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REPOSITORY_NAME", "secrets-repo-name")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REPOSITORY_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REPOSITORY_IMAGE_TAG", "secrets-repo-image-tag")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REPOSITORY_IMAGE_TAG")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_NAMESPACE", "hello")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_NAMESPACE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_ROLE_NAME", "hello")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_ROLE_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_ROLE_SERVICE_PRINCIPALS", "a,b,c")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_ROLE_SERVICE_PRINCIPALS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_ROLE_MANAGED_POLICY_ARNS", "a,b,c")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_ROLE_MANAGED_POLICY_ARNS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_PROFILE_NAME", "hello")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_PROFILE_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_REPOSITORY_ACCOUNT_ID", "uri")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_REPOSITORY_ACCOUNT_ID")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_REPOSITORY_NAME", "fargate-repo-name")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_REPOSITORY_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_REPOSITORY_IMAGE_TAG", "fargate-repo-image-tag")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_REPOSITORY_IMAGE_TAG")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_SECRET_NAME", "HELLO-SECRET")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_SECRET_NAME")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_NAMESPACE", "hello")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_NAMESPACE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_ROLE_NAME", "hello")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_ROLE_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_ROLE_MANAGED_POLICY_ARNS", "a,b,c")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_ROLE_MANAGED_POLICY_ARNS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_S3_KEY", "hello")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_S3_KEY")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_REPOSITORY_ACCOUNT_ID", "uri")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_REPOSITORY_ACCOUNT_ID")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_REPOSITORY_NAME", "irsa-repo-name")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_REPOSITORY_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_REPOSITORY_IMAGE_TAG", "irsa-repo-image-tag")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_REPOSITORY_IMAGE_TAG")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_DEPLOYMENT_RESULT_PATH", "hello-deployment.log")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_DEPLOYMENT_RESULT_PATH")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_NAMESPACE", "hello")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_NAMESPACE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_ROLE_NAME", "hello")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_ROLE_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_ROLE_SERVICE_PRINCIPALS", "a,b,c")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_ROLE_SERVICE_PRINCIPALS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_ROLE_MANAGED_POLICY_ARNS", "a,b,c")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_ROLE_MANAGED_POLICY_ARNS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_S3_KEY", "hello")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_S3_KEY")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_PROFILE_NAME", "hello")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_PROFILE_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_REPOSITORY_ACCOUNT_ID", "uri")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_REPOSITORY_ACCOUNT_ID")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_REPOSITORY_NAME", "irsa-fargate-repo-name")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_REPOSITORY_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_REPOSITORY_IMAGE_TAG", "irsa-fargate-repo-image-tag")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_REPOSITORY_IMAGE_TAG")

	proxySecretToken := randutil.Hex(32)
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JUPYTER_HUB_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JUPYTER_HUB_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JUPYTER_HUB_NAMESPACE", "jhhub")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JUPYTER_HUB_NAMESPACE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JUPYTER_HUB_PROXY_SECRET_TOKEN", proxySecretToken)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JUPYTER_HUB_PROXY_SECRET_TOKEN")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_LOCAL_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_LOCAL_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_LOCAL_NODE_LABEL_PREFIX", "hollow-prefix")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_LOCAL_NODE_LABEL_PREFIX")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_LOCAL_MAX_OPEN_FILES", "333")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_LOCAL_MAX_OPEN_FILES")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_NAMESPACE", "test-hollow-nodes-namespace")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_NAMESPACE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_NODES", "333")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_NODES")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_REPOSITORY_ACCOUNT_ID", "uri")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_REPOSITORY_ACCOUNT_ID")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_REPOSITORY_NAME", "hollow-nodes-repo-name")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_REPOSITORY_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_REPOSITORY_URI", "hollow-nodes-repo-uri")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_REPOSITORY_URI")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_REPOSITORY_IMAGE_TAG", "hollow-nodes-repo-image-tag")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_REPOSITORY_IMAGE_TAG")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_DURATION", "7m30s")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_DURATION")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_DURATION", "7m30s")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_DURATION")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REPOSITORY_ACCOUNT_ID", "uri")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REPOSITORY_ACCOUNT_ID")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REPOSITORY_NAME", "stresser-repo-name")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REPOSITORY_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REPOSITORY_IMAGE_TAG", "stresser-repo-image-tag")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REPOSITORY_IMAGE_TAG")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_DEPLOYMENT_REPLICAS", "500")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_DEPLOYMENT_REPLICAS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_OBJECT_SIZE", "512")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REQUESTS_SUMMARY_WRITES_OUTPUT_NAME_PREFIX", "stresser-out-pfx")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REQUESTS_SUMMARY_WRITES_OUTPUT_NAME_PREFIX")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REQUESTS_SUMMARY_READS_OUTPUT_NAME_PREFIX", "stresser-out-pfx")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REQUESTS_SUMMARY_READS_OUTPUT_NAME_PREFIX")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_NAMESPACE", "conformance-test")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_NAMESPACE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_PATH", "aaaaa")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_PATH")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_DOWNLOAD_URL", "sonobuoy-download-here")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_DOWNLOAD_URL")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_DELETE_TIMEOUT", "10s")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_DELETE_TIMEOUT")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_RUN_TIMEOUT", "10h")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_RUN_TIMEOUT")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_RUN_MODE", "non-disruptive-conformance")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_RUN_MODE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_RUN_KUBE_CONFORMANCE_IMAGE", "hello.com/v1")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_RUN_KUBE_CONFORMANCE_IMAGE")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}

	if cfg.KubectlCommandsOutputPath != "hello-kubectl" {
		t.Fatalf("unexpected %q", cfg.KubectlCommandsOutputPath)
	}
	if cfg.RemoteAccessCommandsOutputPath != "hello-ssh" {
		t.Fatalf("unexpected %q", cfg.RemoteAccessCommandsOutputPath)
	}
	if cfg.Region != "us-east-1" {
		t.Fatalf("unexpected %q", cfg.Region)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("unexpected %q", cfg.LogLevel)
	}
	if cfg.KubectlDownloadURL != "https://amazon-eks.s3-us-west-2.amazonaws.com/1.11.5/2018-12-06/bin/linux/amd64/kubectl" {
		t.Fatalf("unexpected KubectlDownloadURL %q", cfg.KubectlDownloadURL)
	}
	if cfg.KubectlPath != "/tmp/aws-k8s-tester-test/kubectl" {
		t.Fatalf("unexpected KubectlPath %q", cfg.KubectlPath)
	}
	if cfg.KubeConfigPath != "/tmp/aws-k8s-tester/kubeconfig2" {
		t.Fatalf("unexpected KubeConfigPath %q", cfg.KubeConfigPath)
	}
	if cfg.OnFailureDelete {
		t.Fatalf("unexpected OnFailureDelete %v", cfg.OnFailureDelete)
	}
	if cfg.OnFailureDeleteWaitSeconds != 780 {
		t.Fatalf("unexpected OnFailureDeleteWaitSeconds %d", cfg.OnFailureDeleteWaitSeconds)
	}
	if cfg.CommandAfterCreateCluster != "echo hello1" {
		t.Fatalf("unexpected CommandAfterCreateCluster %q", cfg.CommandAfterCreateCluster)
	}
	if cfg.CommandAfterCreateClusterTimeout != 7*time.Minute {
		t.Fatalf("unexpected CommandAfterCreateClusterTimeout %v", cfg.CommandAfterCreateClusterTimeout)
	}
	if cfg.CommandAfterCreateAddOns != "echo hello2" {
		t.Fatalf("unexpected CommandAfterCreateAddOns %q", cfg.CommandAfterCreateAddOns)
	}
	if cfg.CommandAfterCreateAddOnsTimeout != 17*time.Minute {
		t.Fatalf("unexpected CommandAfterCreateAddOnsTimeout %v", cfg.CommandAfterCreateAddOnsTimeout)
	}
	if !cfg.S3BucketCreate {
		t.Fatalf("unexpected cfg.S3BucketCreate %v", cfg.S3BucketCreate)
	}
	if !cfg.S3BucketCreateKeep {
		t.Fatalf("unexpected cfg.S3BucketCreateKeep %v", cfg.S3BucketCreateKeep)
	}
	if cfg.S3BucketName != "my-bucket" {
		t.Fatalf("unexpected cfg.S3BucketName %q", cfg.S3BucketName)
	}
	if cfg.S3BucketLifecycleExpirationDays != 10 {
		t.Fatalf("unexpected cfg.S3BucketLifecycleExpirationDays %d", cfg.S3BucketLifecycleExpirationDays)
	}
	if cfg.Clients != 333 {
		t.Fatalf("unexpected cfg.Clients %d", cfg.Clients)
	}
	if cfg.ClientTimeout != 10*time.Minute {
		t.Fatalf("unexpected cfg.ClientTimeout %v", cfg.ClientTimeout)
	}
	if cfg.ClientQPS != 99555.77 {
		t.Fatalf("unexpected cfg.ClientQPS %f", cfg.ClientQPS)
	}
	if cfg.ClientBurst != 177 {
		t.Fatalf("unexpected cfg.ClientBurst %d", cfg.ClientBurst)
	}

	if cfg.Parameters.VPCCreate {
		t.Fatalf("unexpected Parameters.VPCCreate %v", cfg.Parameters.VPCCreate)
	}
	if cfg.Parameters.VPCID != "vpc-id" {
		t.Fatalf("unexpected Parameters.VPCID %q", cfg.Parameters.VPCID)
	}
	if cfg.Parameters.VPCCIDR != "my-cidr" {
		t.Fatalf("unexpected Parameters.VPCCIDR %q", cfg.Parameters.VPCCIDR)
	}
	if cfg.Parameters.PublicSubnetCIDR1 != "public-cidr1" {
		t.Fatalf("unexpected Parameters.PublicSubnetCIDR1 %q", cfg.Parameters.PublicSubnetCIDR1)
	}
	if cfg.Parameters.PublicSubnetCIDR2 != "public-cidr2" {
		t.Fatalf("unexpected Parameters.PublicSubnetCIDR2 %q", cfg.Parameters.PublicSubnetCIDR2)
	}
	if cfg.Parameters.PublicSubnetCIDR3 != "public-cidr3" {
		t.Fatalf("unexpected Parameters.PublicSubnetCIDR3 %q", cfg.Parameters.PublicSubnetCIDR3)
	}
	if cfg.Parameters.PrivateSubnetCIDR1 != "private-cidr1" {
		t.Fatalf("unexpected Parameters.PrivateSubnetCIDR1 %q", cfg.Parameters.PrivateSubnetCIDR1)
	}
	if cfg.Parameters.PrivateSubnetCIDR2 != "private-cidr2" {
		t.Fatalf("unexpected Parameters.PrivateSubnetCIDR2 %q", cfg.Parameters.PrivateSubnetCIDR2)
	}
	if cfg.Parameters.DHCPOptionsDomainName != "hello.com" {
		t.Fatalf("unexpected cfg.Parameters.DHCPOptionsDomainName %q", cfg.Parameters.DHCPOptionsDomainName)
	}
	if !reflect.DeepEqual(cfg.Parameters.DHCPOptionsDomainNameServers, []string{"1.2.3.0", "4.5.6.7"}) {
		t.Fatalf("unexpected cfg.Parameters.DHCPOptionsDomainNameServers %q", cfg.Parameters.DHCPOptionsDomainNameServers)
	}
	expectedTags := map[string]string{"to-delete": "2019", "hello-world": "test"}
	if !reflect.DeepEqual(cfg.Parameters.Tags, expectedTags) {
		t.Fatalf("Tags expected %v, got %v", expectedTags, cfg.Parameters.Tags)
	}
	if cfg.Parameters.RequestHeaderKey != "eks-options" {
		t.Fatalf("unexpected Parameters.RequestHeaderKey %q", cfg.Parameters.RequestHeaderKey)
	}
	if cfg.Parameters.RequestHeaderValue != "kubernetesVersion=1.11" {
		t.Fatalf("unexpected Parameters.RequestHeaderValue %q", cfg.Parameters.RequestHeaderValue)
	}
	if cfg.Parameters.ResolverURL != "amazon.com" {
		t.Fatalf("unexpected Parameters.ResolverURL %q", cfg.Parameters.ResolverURL)
	}
	if cfg.Parameters.SigningName != "a" {
		t.Fatalf("unexpected Parameters.SigningName %q", cfg.Parameters.SigningName)
	}
	if cfg.Parameters.RoleCreate {
		t.Fatalf("unexpected Parameters.RoleCreate %v", cfg.Parameters.RoleCreate)
	}
	if cfg.Parameters.RoleARN != "cluster-role-arn" {
		t.Fatalf("unexpected Parameters.RoleARN %q", cfg.Parameters.RoleARN)
	}
	expectedRoleServicePrincipals := []string{
		"eks.amazonaws.com",
		"eks-beta-pdx.aws.internal",
		"eks-dev.aws.internal",
	}
	if !reflect.DeepEqual(expectedRoleServicePrincipals, cfg.Parameters.RoleServicePrincipals) {
		t.Fatalf("unexpected Parameters.RoleServicePrincipals %+v", cfg.Parameters.RoleServicePrincipals)
	}
	expectedRoleManagedPolicyARNs := []string{
		"arn:aws:iam::aws:policy/AmazonEKSServicePolicy",
		"arn:aws:iam::aws:policy/AmazonEKSClusterPolicy",
		"arn:aws:iam::aws:policy/service-role/AmazonEC2RoleforSSM",
	}
	if !reflect.DeepEqual(expectedRoleManagedPolicyARNs, cfg.Parameters.RoleManagedPolicyARNs) {
		t.Fatalf("unexpected Parameters.RoleManagedPolicyARNs %+v", cfg.Parameters.RoleManagedPolicyARNs)
	}
	if cfg.Parameters.Version != "1.16" {
		t.Fatalf("unexpected Parameters.Version %q", cfg.Parameters.Version)
	}
	if cfg.Parameters.EncryptionCMKCreate {
		t.Fatalf("unexpected Parameters.EncryptionCMKCreate %v", cfg.Parameters.EncryptionCMKCreate)
	}
	if cfg.Parameters.EncryptionCMKARN != "key-arn" {
		t.Fatalf("unexpected Parameters.EncryptionCMKARN %q", cfg.Parameters.EncryptionCMKARN)
	}

	if !cfg.RemoteAccessKeyCreate {
		t.Fatalf("unexpected cfg.RemoteAccessKeyCreate %v", cfg.RemoteAccessKeyCreate)
	}
	if cfg.RemoteAccessKeyName != "a" {
		t.Fatalf("unexpected cfg.RemoteAccessKeyName %q", cfg.RemoteAccessKeyName)
	}
	if cfg.RemoteAccessPrivateKeyPath != "a" {
		t.Fatalf("unexpected cfg.RemoteAccessPrivateKeyPath %q", cfg.RemoteAccessPrivateKeyPath)
	}

	if !cfg.AddOnNodeGroups.Enable {
		t.Fatalf("unexpected cfg.AddOnNodeGroups.Enable %v", cfg.AddOnNodeGroups.Enable)
	}
	if cfg.AddOnNodeGroups.FetchLogs {
		t.Fatalf("unexpected cfg.AddOnNodeGroups.FetchLogs %v", cfg.AddOnNodeGroups.FetchLogs)
	}
	if !cfg.AddOnNodeGroups.RoleCreate {
		t.Fatalf("unexpected AddOnNodeGroups.RoleCreate %v", cfg.AddOnNodeGroups.RoleCreate)
	}
	if cfg.AddOnNodeGroups.RoleName != "ng-role-name" {
		t.Fatalf("unexpected cfg.AddOnNodeGroups.RoleName %q", cfg.AddOnNodeGroups.RoleName)
	}
	if cfg.AddOnNodeGroups.RoleARN != "ng-role-arn" {
		t.Fatalf("unexpected cfg.AddOnNodeGroups.RoleARN %q", cfg.AddOnNodeGroups.RoleARN)
	}
	expectedNGRoleServicePrincipals := []string{
		"ec2.amazonaws.com",
		"eks.amazonaws.com",
		"hello.amazonaws.com",
	}
	if !reflect.DeepEqual(expectedNGRoleServicePrincipals, cfg.AddOnNodeGroups.RoleServicePrincipals) {
		t.Fatalf("unexpected cfg.AddOnNodeGroups.RoleServicePrincipals %+v", cfg.AddOnNodeGroups.RoleServicePrincipals)
	}
	expectedNGRoleManagedPolicyARNs := []string{
		"a",
		"b",
		"c",
	}
	if !reflect.DeepEqual(expectedNGRoleManagedPolicyARNs, cfg.AddOnNodeGroups.RoleManagedPolicyARNs) {
		t.Fatalf("unexpected cfg.AddOnNodeGroups.RoleManagedPolicyARNs %+v", cfg.AddOnNodeGroups.RoleManagedPolicyARNs)
	}
	cpuName, gpuName := "ng-test-name-cpu", "ng-test-name-gpu"
	expectedASGs := map[string]ASG{
		cpuName: {
			ASG: ec2config.ASG{
				Name:                 cpuName,
				RemoteAccessUserName: "ec2-user",
				AMIType:              "AL2_x86_64",
				ImageIDSSMParameter:  "/aws/service/eks/optimized-ami/1.30/amazon-linux-2/recommended/image_id",
				ASGMinSize:           17,
				ASGMaxSize:           99,
				ASGDesiredCapacity:   77,
				InstanceTypes:        []string{"type-cpu-2"},
				VolumeSize:           40,
			},
			KubeletExtraArgs: "bbb qq",
		},
		gpuName: {
			ASG: ec2config.ASG{
				Name:                 gpuName,
				RemoteAccessUserName: "ec2-user",
				AMIType:              eks.AMITypesAl2X8664Gpu,
				ImageID:              "my-gpu-ami",
				ASGMinSize:           30,
				ASGMaxSize:           35,
				ASGDesiredCapacity:   34,
				InstanceTypes:        []string{"type-gpu-2"},
				VolumeSize:           500,
			},
			KubeletExtraArgs: "aaa aa",
		},
	}
	if !reflect.DeepEqual(cfg.AddOnNodeGroups.ASGs, expectedASGs) {
		t.Fatalf("expected cfg.AddOnNodeGroups.ASGs\n%+v\n\ngot\n%+v\n", expectedASGs, cfg.AddOnNodeGroups.ASGs)
	}
	if cfg.AddOnNodeGroups.LogsDir != "a" {
		t.Fatalf("unexpected cfg.AddOnNodeGroups.LogsDir %q", cfg.AddOnNodeGroups.LogsDir)
	}

	if !cfg.AddOnManagedNodeGroups.Enable {
		t.Fatalf("unexpected cfg.AddOnManagedNodeGroups.Enable %v", cfg.AddOnManagedNodeGroups.Enable)
	}
	if cfg.AddOnManagedNodeGroups.FetchLogs {
		t.Fatalf("unexpected cfg.AddOnManagedNodeGroups.FetchLogs %v", cfg.AddOnManagedNodeGroups.FetchLogs)
	}
	if !cfg.AddOnManagedNodeGroups.RoleCreate {
		t.Fatalf("unexpected AddOnManagedNodeGroups.RoleCreate %v", cfg.AddOnManagedNodeGroups.RoleCreate)
	}
	if cfg.AddOnManagedNodeGroups.RoleName != "mng-role-name" {
		t.Fatalf("unexpected cfg.AddOnManagedNodeGroups.RoleName %q", cfg.AddOnManagedNodeGroups.RoleName)
	}
	if cfg.AddOnManagedNodeGroups.RoleARN != "mng-role-arn" {
		t.Fatalf("unexpected cfg.AddOnManagedNodeGroups.RoleARN %q", cfg.AddOnManagedNodeGroups.RoleARN)
	}
	expectedMNGRoleServicePrincipals := []string{
		"ec2.amazonaws.com",
		"eks.amazonaws.com",
		"hello.amazonaws.com",
	}
	if !reflect.DeepEqual(expectedMNGRoleServicePrincipals, cfg.AddOnManagedNodeGroups.RoleServicePrincipals) {
		t.Fatalf("unexpected cfg.AddOnManagedNodeGroups.RoleServicePrincipals %+v", cfg.AddOnManagedNodeGroups.RoleServicePrincipals)
	}
	expectedMNGRoleManagedPolicyARNs := []string{
		"a",
		"b",
		"c",
	}
	if !reflect.DeepEqual(expectedMNGRoleManagedPolicyARNs, cfg.AddOnManagedNodeGroups.RoleManagedPolicyARNs) {
		t.Fatalf("unexpected cfg.AddOnManagedNodeGroups.RoleManagedPolicyARNs %+v", cfg.AddOnManagedNodeGroups.RoleManagedPolicyARNs)
	}
	if cfg.AddOnManagedNodeGroups.RequestHeaderKey != "a" {
		t.Fatalf("unexpected cfg.AddOnManagedNodeGroups.RequestHeaderKey %q", cfg.AddOnManagedNodeGroups.RequestHeaderKey)
	}
	if cfg.AddOnManagedNodeGroups.RequestHeaderValue != "b" {
		t.Fatalf("unexpected cfg.AddOnManagedNodeGroups.RequestHeaderValue %q", cfg.AddOnManagedNodeGroups.RequestHeaderValue)
	}
	if cfg.AddOnManagedNodeGroups.ResolverURL != "a" {
		t.Fatalf("unexpected cfg.AddOnManagedNodeGroups.ResolverURL %q", cfg.AddOnManagedNodeGroups.ResolverURL)
	}
	if cfg.AddOnManagedNodeGroups.SigningName != "a" {
		t.Fatalf("unexpected cfg.AddOnManagedNodeGroups.SigningName %q", cfg.AddOnManagedNodeGroups.SigningName)
	}
	cpuName, gpuName = "mng-test-name-cpu", "mng-test-name-gpu"
	expectedMNGs := map[string]MNG{
		cpuName: {
			Name:                 cpuName,
			RemoteAccessUserName: "ec2-user",
			Tags:                 map[string]string{"cpu": "hello-world"},
			ReleaseVersion:       "test-ver-cpu",
			AMIType:              "AL2_x86_64",
			ASGMinSize:           17,
			ASGMaxSize:           99,
			ASGDesiredCapacity:   77,
			InstanceTypes:        []string{"type-cpu-1", "type-cpu-2"},
			VolumeSize:           40,
		},
		gpuName: {
			Name:                 gpuName,
			RemoteAccessUserName: "ec2-user",
			Tags:                 map[string]string{"gpu": "hello-world"},
			ReleaseVersion:       "test-ver-gpu",
			AMIType:              eks.AMITypesAl2X8664Gpu,
			ASGMinSize:           30,
			ASGMaxSize:           35,
			ASGDesiredCapacity:   34,
			InstanceTypes:        []string{"type-gpu-1", "type-gpu-2"},
			VolumeSize:           500,
		},
	}
	if !reflect.DeepEqual(cfg.AddOnManagedNodeGroups.MNGs, expectedMNGs) {
		t.Fatalf("expected cfg.AddOnManagedNodeGroups.MNGs %+v, got %+v", expectedMNGs, cfg.AddOnManagedNodeGroups.MNGs)
	}
	if cfg.AddOnManagedNodeGroups.LogsDir != "a" {
		t.Fatalf("unexpected cfg.AddOnManagedNodeGroups.LogsDir %q", cfg.AddOnManagedNodeGroups.LogsDir)
	}

	if !cfg.AddOnNLBHelloWorld.Enable {
		t.Fatalf("unexpected cfg.AddOnNLBHelloWorld.Enable %v", cfg.AddOnNLBHelloWorld.Enable)
	}
	if cfg.AddOnNLBHelloWorld.DeploymentReplicas != 333 {
		t.Fatalf("unexpected cfg.AddOnNLBHelloWorld.DeploymentReplicas %d", cfg.AddOnNLBHelloWorld.DeploymentReplicas)
	}
	if cfg.AddOnNLBHelloWorld.Namespace != "test-namespace" {
		t.Fatalf("unexpected cfg.AddOnNLBHelloWorld.Namespace %q", cfg.AddOnNLBHelloWorld.Namespace)
	}

	if !cfg.AddOnALB2048.Enable {
		t.Fatalf("unexpected cfg.AddOnALB2048.Enable %v", cfg.AddOnALB2048.Enable)
	}
	if cfg.AddOnALB2048.DeploymentReplicasALB != 333 {
		t.Fatalf("unexpected cfg.AddOnALB2048.DeploymentReplicasALB %d", cfg.AddOnALB2048.DeploymentReplicasALB)
	}
	if cfg.AddOnALB2048.DeploymentReplicas2048 != 555 {
		t.Fatalf("unexpected cfg.AddOnALB2048.DeploymentReplicas2048 %d", cfg.AddOnALB2048.DeploymentReplicas2048)
	}
	if cfg.AddOnALB2048.Namespace != "test-namespace" {
		t.Fatalf("unexpected cfg.AddOnALB2048.Namespace %q", cfg.AddOnALB2048.Namespace)
	}

	if !cfg.AddOnJobsPi.Enable {
		t.Fatalf("unexpected cfg.AddOnJobsPi.Enable %v", cfg.AddOnJobsPi.Enable)
	}
	if cfg.AddOnJobsPi.Namespace != "hello1" {
		t.Fatalf("unexpected cfg.AddOnJobsPi.Namespace %q", cfg.AddOnJobsPi.Namespace)
	}
	if cfg.AddOnJobsPi.Completes != 100 {
		t.Fatalf("unexpected cfg.AddOnJobsPi.Completes %v", cfg.AddOnJobsPi.Completes)
	}
	if cfg.AddOnJobsPi.Parallels != 10 {
		t.Fatalf("unexpected cfg.AddOnJobsPi.Parallels %v", cfg.AddOnJobsPi.Parallels)
	}

	if !cfg.AddOnJobsEcho.Enable {
		t.Fatalf("unexpected cfg.AddOnJobsEcho.Enable %v", cfg.AddOnJobsEcho.Enable)
	}
	if cfg.AddOnJobsEcho.Namespace != "hello2" {
		t.Fatalf("unexpected cfg.AddOnJobsEcho.Namespace %q", cfg.AddOnJobsEcho.Namespace)
	}
	if cfg.AddOnJobsEcho.Completes != 1000 {
		t.Fatalf("unexpected cfg.AddOnJobsEcho.Completes %v", cfg.AddOnJobsEcho.Completes)
	}
	if cfg.AddOnJobsEcho.Parallels != 100 {
		t.Fatalf("unexpected cfg.AddOnJobsEcho.Parallels %v", cfg.AddOnJobsEcho.Parallels)
	}
	if cfg.AddOnJobsEcho.EchoSize != 10000 {
		t.Fatalf("unexpected cfg.AddOnJobsEcho.EchoSize %v", cfg.AddOnJobsEcho.EchoSize)
	}

	if !cfg.AddOnCronJobs.Enable {
		t.Fatalf("unexpected cfg.AddOnCronJobs.Enable %v", cfg.AddOnCronJobs.Enable)
	}
	if cfg.AddOnCronJobs.Namespace != "hello3" {
		t.Fatalf("unexpected cfg.AddOnCronJobs.Namespace %q", cfg.AddOnCronJobs.Namespace)
	}
	if cfg.AddOnCronJobs.Schedule != "*/1 * * * *" {
		t.Fatalf("unexpected cfg.AddOnCronJobs.Schedule %q", cfg.AddOnCronJobs.Schedule)
	}
	if cfg.AddOnCronJobs.Completes != 100 {
		t.Fatalf("unexpected cfg.AddOnCronJobs.Completes %v", cfg.AddOnCronJobs.Completes)
	}
	if cfg.AddOnCronJobs.Parallels != 10 {
		t.Fatalf("unexpected cfg.AddOnCronJobs.Parallels %v", cfg.AddOnCronJobs.Parallels)
	}
	if cfg.AddOnCronJobs.SuccessfulJobsHistoryLimit != 100 {
		t.Fatalf("unexpected cfg.AddOnCronJobs.SuccessfulJobsHistoryLimit %d", cfg.AddOnCronJobs.SuccessfulJobsHistoryLimit)
	}
	if cfg.AddOnCronJobs.FailedJobsHistoryLimit != 1000 {
		t.Fatalf("unexpected cfg.AddOnCronJobs.FailedJobsHistoryLimit %d", cfg.AddOnCronJobs.FailedJobsHistoryLimit)
	}
	if cfg.AddOnCronJobs.EchoSize != 10000 {
		t.Fatalf("unexpected cfg.AddOnCronJobs.EchoSize %d", cfg.AddOnCronJobs.EchoSize)
	}

	if !cfg.AddOnCSRsLocal.Enable {
		t.Fatalf("unexpected cfg.AddOnCSRsLocal.Enable %v", cfg.AddOnCSRsLocal.Enable)
	}
	if cfg.AddOnCSRsLocal.InitialRequestConditionType != "Random" {
		t.Fatalf("unexpected cfg.AddOnCSRsLocal.InitialRequestConditionType %q", cfg.AddOnCSRsLocal.InitialRequestConditionType)
	}
	if cfg.AddOnCSRsLocal.Objects != 10000 {
		t.Fatalf("unexpected cfg.AddOnCSRsLocal.Objects %d", cfg.AddOnCSRsLocal.Objects)
	}

	if !cfg.AddOnCSRsRemote.Enable {
		t.Fatalf("unexpected cfg.AddOnCSRsRemote.Enable %v", cfg.AddOnCSRsRemote.Enable)
	}
	if cfg.AddOnCSRsRemote.Namespace != "csr-namespace" {
		t.Fatalf("unexpected cfg.AddOnCSRsRemote.Namespace %q", cfg.AddOnCSRsRemote.Namespace)
	}
	if cfg.AddOnCSRsRemote.InitialRequestConditionType != "Random" {
		t.Fatalf("unexpected cfg.AddOnCSRsRemote.InitialRequestConditionType %q", cfg.AddOnCSRsRemote.InitialRequestConditionType)
	}
	if cfg.AddOnCSRsRemote.Objects != 10000 {
		t.Fatalf("unexpected cfg.AddOnCSRsRemote.Objects %d", cfg.AddOnCSRsRemote.Objects)
	}
	if cfg.AddOnCSRsRemote.RepositoryAccountID != "uri" {
		t.Fatalf("unexpected cfg.AddOnCSRsRemote.RepositoryAccountID %v", cfg.AddOnCSRsRemote.RepositoryAccountID)
	}
	if cfg.AddOnCSRsRemote.RepositoryName != "csrs-repo-name" {
		t.Fatalf("unexpected cfg.AddOnCSRsRemote.RepositoryName %v", cfg.AddOnCSRsRemote.RepositoryName)
	}
	if cfg.AddOnCSRsRemote.RepositoryImageTag != "csrs-repo-image-tag" {
		t.Fatalf("unexpected cfg.AddOnCSRsRemote.RepositoryImageTag %v", cfg.AddOnCSRsRemote.RepositoryImageTag)
	}

	if !cfg.AddOnConfigMapsLocal.Enable {
		t.Fatalf("unexpected cfg.AddOnConfigMapsLocal.Enable %v", cfg.AddOnConfigMapsLocal.Enable)
	}
	if cfg.AddOnConfigMapsLocal.Namespace != "config-map-namespace" {
		t.Fatalf("unexpected cfg.AddOnConfigMapsLocal.Namespace %q", cfg.AddOnConfigMapsLocal.Namespace)
	}
	if cfg.AddOnConfigMapsLocal.Objects != 10000 {
		t.Fatalf("unexpected cfg.AddOnConfigMapsLocal.Objects %d", cfg.AddOnConfigMapsLocal.Objects)
	}
	if cfg.AddOnConfigMapsLocal.ObjectSize != 555 {
		t.Fatalf("unexpected cfg.AddOnConfigMapsLocal.ObjectSize %d", cfg.AddOnConfigMapsLocal.ObjectSize)
	}
	if !cfg.AddOnConfigMapsRemote.Enable {
		t.Fatalf("unexpected cfg.AddOnConfigMapsRemote.Enable %v", cfg.AddOnConfigMapsRemote.Enable)
	}
	if cfg.AddOnConfigMapsRemote.Namespace != "config-map-namespace" {
		t.Fatalf("unexpected cfg.AddOnConfigMapsRemote.Namespace %q", cfg.AddOnConfigMapsRemote.Namespace)
	}
	if cfg.AddOnConfigMapsRemote.Objects != 10000 {
		t.Fatalf("unexpected cfg.AddOnConfigMapsRemote.Objects %d", cfg.AddOnConfigMapsRemote.Objects)
	}
	if cfg.AddOnConfigMapsRemote.ObjectSize != 555 {
		t.Fatalf("unexpected cfg.AddOnConfigMapsRemote.ObjectSize %d", cfg.AddOnConfigMapsRemote.ObjectSize)
	}
	if cfg.AddOnConfigMapsRemote.RepositoryAccountID != "uri" {
		t.Fatalf("unexpected cfg.AddOnConfigMapsRemote.RepositoryAccountID %v", cfg.AddOnConfigMapsRemote.RepositoryAccountID)
	}
	if cfg.AddOnConfigMapsRemote.RepositoryName != "config-maps-repo-name" {
		t.Fatalf("unexpected cfg.AddOnConfigMapsRemote.RepositoryName %v", cfg.AddOnConfigMapsRemote.RepositoryName)
	}
	if cfg.AddOnConfigMapsRemote.RepositoryImageTag != "config-maps-repo-image-tag" {
		t.Fatalf("unexpected cfg.AddOnConfigMapsRemote.RepositoryImageTag %v", cfg.AddOnConfigMapsRemote.RepositoryImageTag)
	}

	if !cfg.AddOnSecretsLocal.Enable {
		t.Fatalf("unexpected cfg.AddOnSecretsLocal.Enable %v", cfg.AddOnSecretsLocal.Enable)
	}
	if cfg.AddOnSecretsLocal.Namespace != "hello" {
		t.Fatalf("unexpected cfg.AddOnSecretsLocal.Namespace %q", cfg.AddOnSecretsLocal.Namespace)
	}
	if cfg.AddOnSecretsLocal.Objects != 5 {
		t.Fatalf("unexpected cfg.AddOnSecretsLocal.Objects %v", cfg.AddOnSecretsLocal.Objects)
	}
	if cfg.AddOnSecretsLocal.ObjectSize != 10 {
		t.Fatalf("unexpected cfg.AddOnSecretsLocal.ObjectSize %v", cfg.AddOnSecretsLocal.ObjectSize)
	}

	if !cfg.AddOnSecretsRemote.Enable {
		t.Fatalf("unexpected cfg.AddOnSecretsRemote.Enable %v", cfg.AddOnSecretsRemote.Enable)
	}
	if cfg.AddOnSecretsRemote.Namespace != "hello" {
		t.Fatalf("unexpected cfg.AddOnSecretsRemote.Namespace %q", cfg.AddOnSecretsRemote.Namespace)
	}
	if cfg.AddOnSecretsRemote.Objects != 5 {
		t.Fatalf("unexpected cfg.AddOnSecretsRemote.Objects %v", cfg.AddOnSecretsRemote.Objects)
	}
	if cfg.AddOnSecretsRemote.ObjectSize != 10 {
		t.Fatalf("unexpected cfg.AddOnSecretsRemote.ObjectSize %v", cfg.AddOnSecretsRemote.ObjectSize)
	}
	if cfg.AddOnSecretsRemote.RepositoryAccountID != "uri" {
		t.Fatalf("unexpected cfg.AddOnSecretsRemote.RepositoryAccountID %v", cfg.AddOnSecretsRemote.RepositoryAccountID)
	}
	if cfg.AddOnSecretsRemote.RepositoryName != "secrets-repo-name" {
		t.Fatalf("unexpected cfg.AddOnSecretsRemote.RepositoryName %v", cfg.AddOnSecretsRemote.RepositoryName)
	}
	if cfg.AddOnSecretsRemote.RepositoryImageTag != "secrets-repo-image-tag" {
		t.Fatalf("unexpected cfg.AddOnSecretsRemote.RepositoryImageTag %v", cfg.AddOnSecretsRemote.RepositoryImageTag)
	}

	if !cfg.AddOnFargate.Enable {
		t.Fatalf("unexpected cfg.AddOnFargate.Enable %v", cfg.AddOnFargate.Enable)
	}
	if cfg.AddOnFargate.Namespace != "hello" {
		t.Fatalf("unexpected cfg.AddOnFargate.Namespace %q", cfg.AddOnFargate.Namespace)
	}
	if cfg.AddOnFargate.RoleName != "hello" {
		t.Fatalf("unexpected cfg.AddOnFargate.RoleName %q", cfg.AddOnFargate.RoleName)
	}
	expectedAddOnFargateRoleServicePrincipals := []string{"a", "b", "c"}
	if !reflect.DeepEqual(cfg.AddOnFargate.RoleServicePrincipals, expectedAddOnFargateRoleServicePrincipals) {
		t.Fatalf("unexpected cfg.AddOnFargate.RoleServicePrincipals %q", cfg.AddOnFargate.RoleServicePrincipals)
	}
	expectedAddOnFargateRoleManagedPolicyARNs := []string{"a", "b", "c"}
	if !reflect.DeepEqual(cfg.AddOnFargate.RoleManagedPolicyARNs, expectedAddOnFargateRoleManagedPolicyARNs) {
		t.Fatalf("unexpected cfg.AddOnFargate.RoleManagedPolicyARNs %q", cfg.AddOnFargate.RoleManagedPolicyARNs)
	}
	if cfg.AddOnFargate.ProfileName != "hello" {
		t.Fatalf("unexpected cfg.AddOnFargate.ProfileName %q", cfg.AddOnFargate.ProfileName)
	}
	if cfg.AddOnFargate.RepositoryAccountID != "uri" {
		t.Fatalf("unexpected cfg.AddOnFargate.RepositoryAccountID %v", cfg.AddOnFargate.RepositoryAccountID)
	}
	if cfg.AddOnFargate.RepositoryName != "fargate-repo-name" {
		t.Fatalf("unexpected cfg.AddOnFargate.RepositoryName %v", cfg.AddOnFargate.RepositoryName)
	}
	if cfg.AddOnFargate.RepositoryImageTag != "fargate-repo-image-tag" {
		t.Fatalf("unexpected cfg.AddOnFargate.RepositoryImageTag %v", cfg.AddOnFargate.RepositoryImageTag)
	}
	if cfg.AddOnFargate.SecretName != "HELLO-SECRET" {
		t.Fatalf("unexpected cfg.AddOnFargate.SecretName %q", cfg.AddOnFargate.SecretName)
	}

	if !cfg.AddOnIRSA.Enable {
		t.Fatalf("unexpected cfg.AddOnIRSA.Enable %v", cfg.AddOnIRSA.Enable)
	}
	if cfg.AddOnIRSA.Namespace != "hello" {
		t.Fatalf("unexpected cfg.AddOnIRSA.Namespace %q", cfg.AddOnIRSA.Namespace)
	}
	if cfg.AddOnIRSA.RoleName != "hello" {
		t.Fatalf("unexpected cfg.AddOnIRSA.RoleName %q", cfg.AddOnIRSA.RoleName)
	}
	expectedAddOnIRSARoleManagedPolicyARNs := []string{"a", "b", "c"}
	if !reflect.DeepEqual(cfg.AddOnIRSA.RoleManagedPolicyARNs, expectedAddOnIRSARoleManagedPolicyARNs) {
		t.Fatalf("unexpected cfg.AddOnIRSA.RoleManagedPolicyARNs %q", cfg.AddOnIRSA.RoleManagedPolicyARNs)
	}
	if cfg.AddOnIRSA.S3Key != "hello" {
		t.Fatalf("unexpected cfg.AddOnIRSA.S3Key %q", cfg.AddOnIRSA.S3Key)
	}
	if cfg.AddOnIRSA.RepositoryAccountID != "uri" {
		t.Fatalf("unexpected cfg.AddOnIRSA.RepositoryAccountID %v", cfg.AddOnIRSA.RepositoryAccountID)
	}
	if cfg.AddOnIRSA.RepositoryName != "irsa-repo-name" {
		t.Fatalf("unexpected cfg.AddOnIRSA.RepositoryName %v", cfg.AddOnIRSA.RepositoryName)
	}
	if cfg.AddOnIRSA.RepositoryImageTag != "irsa-repo-image-tag" {
		t.Fatalf("unexpected cfg.AddOnIRSA.RepositoryImageTag %v", cfg.AddOnIRSA.RepositoryImageTag)
	}
	if cfg.AddOnIRSA.DeploymentResultPath != "hello-deployment.log" {
		t.Fatalf("unexpected cfg.AddOnIRSA.DeploymentResultPath %q", cfg.AddOnIRSA.DeploymentResultPath)
	}

	if !cfg.AddOnIRSAFargate.Enable {
		t.Fatalf("unexpected cfg.AddOnIRSAFargate.Enable %v", cfg.AddOnIRSAFargate.Enable)
	}
	if cfg.AddOnIRSAFargate.Namespace != "hello" {
		t.Fatalf("unexpected cfg.AddOnIRSAFargate.Namespace %q", cfg.AddOnIRSAFargate.Namespace)
	}
	if cfg.AddOnIRSAFargate.RoleName != "hello" {
		t.Fatalf("unexpected cfg.AddOnIRSAFargate.RoleName %q", cfg.AddOnIRSAFargate.RoleName)
	}
	expectedAddOnIRSAFargateRoleServicePrincipals := []string{"a", "b", "c"}
	if !reflect.DeepEqual(cfg.AddOnIRSAFargate.RoleServicePrincipals, expectedAddOnIRSAFargateRoleServicePrincipals) {
		t.Fatalf("unexpected cfg.AddOnIRSAFargate.RoleServicePrincipals %q", cfg.AddOnIRSAFargate.RoleServicePrincipals)
	}
	expectedAddOnIRSAFargateRoleManagedPolicyARNs := []string{"a", "b", "c"}
	if !reflect.DeepEqual(cfg.AddOnIRSAFargate.RoleManagedPolicyARNs, expectedAddOnIRSAFargateRoleManagedPolicyARNs) {
		t.Fatalf("unexpected cfg.AddOnIRSAFargate.RoleManagedPolicyARNs %q", cfg.AddOnIRSAFargate.RoleManagedPolicyARNs)
	}
	if cfg.AddOnIRSAFargate.S3Key != "hello" {
		t.Fatalf("unexpected cfg.AddOnIRSAFargate.S3Key %q", cfg.AddOnIRSAFargate.S3Key)
	}
	if cfg.AddOnIRSAFargate.ProfileName != "hello" {
		t.Fatalf("unexpected cfg.AddOnIRSAFargate.ProfileName %q", cfg.AddOnIRSAFargate.ProfileName)
	}
	if cfg.AddOnIRSAFargate.RepositoryAccountID != "uri" {
		t.Fatalf("unexpected cfg.AddOnIRSAFargate.RepositoryAccountID %v", cfg.AddOnIRSAFargate.RepositoryAccountID)
	}
	if cfg.AddOnIRSAFargate.RepositoryName != "irsa-fargate-repo-name" {
		t.Fatalf("unexpected cfg.AddOnIRSAFargate.RepositoryName %v", cfg.AddOnIRSAFargate.RepositoryName)
	}
	if cfg.AddOnIRSAFargate.RepositoryImageTag != "irsa-fargate-repo-image-tag" {
		t.Fatalf("unexpected cfg.AddOnIRSAFargate.RepositoryImageTag %v", cfg.AddOnIRSAFargate.RepositoryImageTag)
	}

	if !cfg.AddOnJupyterHub.Enable {
		t.Fatalf("unexpected cfg.AddOnJupyterHub.Enable %v", cfg.AddOnJupyterHub.Enable)
	}
	if cfg.AddOnJupyterHub.Namespace != "jhhub" {
		t.Fatalf("unexpected cfg.AddOnJupyterHub.Namespace %q", cfg.AddOnJupyterHub.Namespace)
	}
	if cfg.AddOnJupyterHub.ProxySecretToken != proxySecretToken {
		t.Fatalf("unexpected cfg.AddOnJupyterHub.ProxySecretToken %q", cfg.AddOnJupyterHub.ProxySecretToken)
	}

	if !cfg.AddOnHollowNodesLocal.Enable {
		t.Fatalf("unexpected cfg.AddOnHollowNodesLocal.Enable %v", cfg.AddOnHollowNodesLocal.Enable)
	}
	if cfg.AddOnHollowNodesLocal.NodeLabelPrefix != "hollow-prefix" {
		t.Fatalf("unexpected cfg.AddOnHollowNodesLocal.NodeLabelPrefix %q", cfg.AddOnHollowNodesLocal.NodeLabelPrefix)
	}
	if cfg.AddOnHollowNodesLocal.MaxOpenFiles != 333 {
		t.Fatalf("unexpected cfg.AddOnHollowNodesLocal.MaxOpenFiles %v", cfg.AddOnHollowNodesLocal.MaxOpenFiles)
	}

	if !cfg.AddOnHollowNodesRemote.Enable {
		t.Fatalf("unexpected cfg.AddOnHollowNodesRemote.Enable %v", cfg.AddOnHollowNodesRemote.Enable)
	}
	if cfg.AddOnHollowNodesRemote.Nodes != 333 {
		t.Fatalf("unexpected cfg.AddOnHollowNodesRemote.Nodes %v", cfg.AddOnHollowNodesRemote.Nodes)
	}
	if cfg.AddOnHollowNodesRemote.Namespace != "test-hollow-nodes-namespace" {
		t.Fatalf("unexpected cfg.AddOnHollowNodesRemote.Namespace %v", cfg.AddOnHollowNodesRemote.Namespace)
	}
	if cfg.AddOnHollowNodesRemote.RepositoryAccountID != "uri" {
		t.Fatalf("unexpected cfg.AddOnHollowNodesRemote.RepositoryAccountID %v", cfg.AddOnHollowNodesRemote.RepositoryAccountID)
	}
	if cfg.AddOnHollowNodesRemote.RepositoryName != "hollow-nodes-repo-name" {
		t.Fatalf("unexpected cfg.AddOnHollowNodesRemote.RepositoryName %v", cfg.AddOnHollowNodesRemote.RepositoryName)
	}
	if cfg.AddOnHollowNodesRemote.RepositoryImageTag != "hollow-nodes-repo-image-tag" {
		t.Fatalf("unexpected cfg.AddOnHollowNodesRemote.RepositoryImageTag %v", cfg.AddOnHollowNodesRemote.RepositoryImageTag)
	}

	if !cfg.AddOnStresserLocal.Enable {
		t.Fatalf("unexpected cfg.AddOnStresserLocal.Enable %v", cfg.AddOnStresserLocal.Enable)
	}
	if cfg.AddOnStresserLocal.Duration != 7*time.Minute+30*time.Second {
		t.Fatalf("unexpected cfg.AddOnStresserLocal.Duration %v", cfg.AddOnStresserLocal.Duration)
	}

	if !cfg.AddOnStresserRemote.Enable {
		t.Fatalf("unexpected cfg.AddOnStresserRemote.Enable %v", cfg.AddOnStresserRemote.Enable)
	}
	if cfg.AddOnStresserRemote.Duration != 7*time.Minute+30*time.Second {
		t.Fatalf("unexpected cfg.AddOnStresserRemote.Duration %v", cfg.AddOnStresserRemote.Duration)
	}
	if cfg.AddOnStresserRemote.RepositoryAccountID != "uri" {
		t.Fatalf("unexpected cfg.AddOnStresserRemote.RepositoryAccountID %v", cfg.AddOnStresserRemote.RepositoryAccountID)
	}
	if cfg.AddOnStresserRemote.RepositoryName != "stresser-repo-name" {
		t.Fatalf("unexpected cfg.AddOnStresserRemote.RepositoryName %v", cfg.AddOnStresserRemote.RepositoryName)
	}
	if cfg.AddOnStresserRemote.RepositoryImageTag != "stresser-repo-image-tag" {
		t.Fatalf("unexpected cfg.AddOnStresserRemote.RepositoryImageTag %v", cfg.AddOnStresserRemote.RepositoryImageTag)
	}
	if cfg.AddOnStresserRemote.DeploymentReplicas != 500 {
		t.Fatalf("unexpected cfg.AddOnStresserRemote.DeploymentReplicas %v", cfg.AddOnStresserRemote.DeploymentReplicas)
	}
	if cfg.AddOnStresserRemote.ObjectSize != 512 {
		t.Fatalf("unexpected cfg.AddOnStresserRemote.ObjectSize %v", cfg.AddOnStresserRemote.ObjectSize)
	}
	if cfg.AddOnStresserRemote.RequestsSummaryWritesOutputNamePrefix != "stresser-out-pfx" {
		t.Fatalf("unexpected cfg.AddOnStresserRemote.RequestsSummaryWritesOutputNamePrefix %v", cfg.AddOnStresserRemote.RequestsSummaryWritesOutputNamePrefix)
	}
	if cfg.AddOnStresserRemote.RequestsSummaryReadsOutputNamePrefix != "stresser-out-pfx" {
		t.Fatalf("unexpected cfg.AddOnStresserRemote.RequestsSummaryReadsOutputNamePrefix %v", cfg.AddOnStresserRemote.RequestsSummaryReadsOutputNamePrefix)
	}

	if !cfg.AddOnConformance.Enable {
		t.Fatalf("unexpected cfg.AddOnConformance.Enable %v", cfg.AddOnConformance.Enable)
	}
	if cfg.AddOnConformance.Namespace != "conformance-test" {
		t.Fatalf("unexpected cfg.AddOnConformance.Namespace %q", cfg.AddOnConformance.Namespace)
	}
	if cfg.AddOnConformance.SonobuoyPath != "aaaaa" {
		t.Fatalf("unexpected cfg.AddOnConformance.SonobuoyPath %q", cfg.AddOnConformance.SonobuoyPath)
	}
	if cfg.AddOnConformance.SonobuoyDownloadURL != "sonobuoy-download-here" {
		t.Fatalf("unexpected cfg.AddOnConformance.SonobuoyDownloadURL %q", cfg.AddOnConformance.SonobuoyDownloadURL)
	}
	if cfg.AddOnConformance.SonobuoyDeleteTimeout != 10*time.Second {
		t.Fatalf("unexpected cfg.AddOnConformance.SonobuoyDeleteTimeout %v", cfg.AddOnConformance.SonobuoyDeleteTimeout)
	}
	if cfg.AddOnConformance.SonobuoyRunTimeout != 10*time.Hour {
		t.Fatalf("unexpected cfg.AddOnConformance.SonobuoyRunTimeout %v", cfg.AddOnConformance.SonobuoyRunTimeout)
	}
	if cfg.AddOnConformance.SonobuoyRunMode != "non-disruptive-conformance" {
		t.Fatalf("unexpected cfg.AddOnConformance.SonobuoyRunMode %q", cfg.AddOnConformance.SonobuoyRunMode)
	}
	if cfg.AddOnConformance.SonobuoyRunKubeConformanceImage != "hello.com/v1" {
		t.Fatalf("unexpected cfg.AddOnConformance.SonobuoyRunKubeConformanceImage %q", cfg.AddOnConformance.SonobuoyRunKubeConformanceImage)
	}

	cfg.Parameters.RoleManagedPolicyARNs = nil
	cfg.Parameters.RoleServicePrincipals = nil
	cfg.AddOnManagedNodeGroups.RoleName = ""
	cfg.AddOnManagedNodeGroups.RoleManagedPolicyARNs = nil
	cfg.AddOnManagedNodeGroups.RoleServicePrincipals = nil
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		t.Fatal(err)
	}
	cfg.AddOnNLBHelloWorld.Enable = false
	cfg.AddOnALB2048.Enable = false
	cfg.AddOnJobsEcho.Enable = false
	cfg.AddOnJobsPi.Enable = false
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		t.Fatal(err)
	}

	if cfg.ClientTimeoutString != "10m0s" {
		t.Fatalf("unexpected ClientTimeoutString %q", cfg.ClientTimeoutString)
	}

	if cfg.AddOnFargate.SecretName != "hellosecret" {
		t.Fatalf("unexpected cfg.AddOnFargate.SecretName %q", cfg.AddOnFargate.SecretName)
	}

	d, err := ioutil.ReadFile(cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(d))
}

func TestEnvAddOnManagedNodeGroups(t *testing.T) {
	cfg := NewDefault()
	defer func() {
		os.RemoveAll(cfg.ConfigPath)
		os.RemoveAll(cfg.KubectlCommandsOutputPath)
		os.RemoveAll(cfg.RemoteAccessCommandsOutputPath)
	}()

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ENABLE", "false")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE", "false")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}

	if cfg.AddOnNodeGroups.Enable {
		t.Fatal("AddOnNodeGroups.Enable expected false, got true")
	}
	if cfg.AddOnManagedNodeGroups.Enable {
		t.Fatal("AddOnManagedNodeGroups.Enable expected false, got true")
	}

	cfg.AddOnNLBHelloWorld.Enable = true
	if err := cfg.ValidateAndSetDefaults(); !strings.Contains(err.Error(), "AddOnNLBHelloWorld.Enable true") {
		t.Fatalf("expected add-on error, got %v", err)
	}
}

func TestEnvAddOnNodeGroupsGetRef(t *testing.T) {
	cfg := NewDefault()
	defer func() {
		os.RemoveAll(cfg.ConfigPath)
		os.RemoveAll(cfg.KubectlCommandsOutputPath)
		os.RemoveAll(cfg.RemoteAccessCommandsOutputPath)
	}()

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ENABLE", `true`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ASGS", `{"GetRef.Name-ng-for-cni":{"name":"GetRef.Name-ng-for-cni","remote-access-user-name":"ec2-user","ami-type":"AL2_x86_64","asg-min-size":30,"asg-max-size":35,"asg-desired-capacity":34,"image-id":"my-ami",  "ssm-document-create":true,   "instance-types":["type-2"],  "ssm-document-cfn-stack-name":"GetRef.Name-ssm", "ssm-document-name":"GetRef.Name-document",     "kubelet-extra-args":"aaa aa",  "volume-size":500}}`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ASGS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE", `true`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_MNGS", `{"GetRef.Name-mng-for-cni":{"name":"GetRef.Name-mng-for-cni","remote-access-user-name":"ec2-user","tags":{"group":"amazon-vpc-cni-k8s"},"ami-type":"AL2_x86_64","asg-min-size":3,"asg-max-size":3,"asg-desired-capacity":3,"instance-types":["c5.xlarge"]}}`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_MNGS")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		t.Fatal(err)
	}

	expectedNGs := map[string]ASG{
		cfg.Name + "-ng-for-cni": {
			ASG: ec2config.ASG{
				Name:                               cfg.Name + "-ng-for-cni",
				RemoteAccessUserName:               "ec2-user",
				SSMDocumentName:                    regex.ReplaceAllString(cfg.Name+"-document", ""),
				SSMDocumentCFNStackName:            cfg.Name + "-ssm",
				SSMDocumentExecutionTimeoutSeconds: 3600,
				SSMDocumentCreate:                  true,
				ImageID:                            "my-ami",
				AMIType:                            eks.AMITypesAl2X8664,
				InstanceTypes:                      []string{"type-2"},
				VolumeSize:                         500,
				ASGMinSize:                         30,
				ASGMaxSize:                         35,
				ASGDesiredCapacity:                 34,
			},
			KubeletExtraArgs: "aaa aa",
		},
	}
	if !reflect.DeepEqual(cfg.AddOnNodeGroups.ASGs, expectedNGs) {
		t.Fatalf("expected cfg.AddOnNodeGroups.ASGs %+v, got %+v", expectedNGs, cfg.AddOnNodeGroups.ASGs)
	}
	expectedMNGs := map[string]MNG{
		cfg.Name + "-mng-for-cni": {
			Name:                 cfg.Name + "-mng-for-cni",
			RemoteAccessUserName: "ec2-user",
			Tags:                 map[string]string{"group": "amazon-vpc-cni-k8s"},
			AMIType:              "AL2_x86_64",
			ASGMinSize:           3,
			ASGMaxSize:           3,
			ASGDesiredCapacity:   3,
			InstanceTypes:        []string{"c5.xlarge"},
			VolumeSize:           40,
		},
	}
	if !reflect.DeepEqual(cfg.AddOnManagedNodeGroups.MNGs, expectedMNGs) {
		t.Fatalf("expected cfg.AddOnManagedNodeGroups.MNGs %+v, got %+v", expectedMNGs, cfg.AddOnManagedNodeGroups.MNGs)
	}
}

// TestEnvAddOnManagedNodeGroupsCNI tests CNI integration test MNG settings.
// https://github.com/aws/amazon-vpc-cni-k8s/blob/master/scripts/lib/cluster.sh
func TestEnvAddOnManagedNodeGroupsCNI(t *testing.T) {
	cfg := NewDefault()
	defer func() {
		os.RemoveAll(cfg.ConfigPath)
		os.RemoveAll(cfg.KubectlCommandsOutputPath)
		os.RemoveAll(cfg.RemoteAccessCommandsOutputPath)
	}()

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE", `true`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_REMOTE_ACCESS_PRIVATE_KEY_PATH", `a`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_REMOTE_ACCESS_PRIVATE_KEY_PATH")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_MNGS", `{"test-mng-for-cni":{"name":"test-mng-for-cni","remote-access-user-name":"ec2-user","tags":{"group":"amazon-vpc-cni-k8s"},"ami-type":"AL2_x86_64","asg-min-size":3,"asg-max-size":3,"asg-desired-capacity":3,"instance-types":["c5.xlarge"]}}`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_MNGS")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		t.Fatal(err)
	}

	if cfg.RemoteAccessPrivateKeyPath != "a" {
		t.Fatalf("unexpected cfg.RemoteAccessPrivateKeyPath %q", cfg.RemoteAccessPrivateKeyPath)
	}
	expectedMNGs := map[string]MNG{
		"test-mng-for-cni": {
			Name:                 "test-mng-for-cni",
			RemoteAccessUserName: "ec2-user",
			Tags:                 map[string]string{"group": "amazon-vpc-cni-k8s"},
			AMIType:              "AL2_x86_64",
			ASGMinSize:           3,
			ASGMaxSize:           3,
			ASGDesiredCapacity:   3,
			InstanceTypes:        []string{"c5.xlarge"},
			VolumeSize:           40,
		},
	}
	if !reflect.DeepEqual(cfg.AddOnManagedNodeGroups.MNGs, expectedMNGs) {
		t.Fatalf("expected cfg.AddOnManagedNodeGroups.MNGs %+v, got %+v", expectedMNGs, cfg.AddOnManagedNodeGroups.MNGs)
	}
}

// TestEnvAddOnManagedNodeGroupsInvalidInstanceType tests invalid instance types.
func TestEnvAddOnManagedNodeGroupsInvalidInstanceType(t *testing.T) {
	cfg := NewDefault()
	defer func() {
		os.RemoveAll(cfg.ConfigPath)
		os.RemoveAll(cfg.KubectlCommandsOutputPath)
		os.RemoveAll(cfg.RemoteAccessCommandsOutputPath)
	}()

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE", `true`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_REMOTE_ACCESS_PRIVATE_KEY_PATH", `a`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_REMOTE_ACCESS_PRIVATE_KEY_PATH")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_MNGS", `{"test-mng-for-cni":{"name":"test-mng-for-cni","tags":{"group":"amazon-vpc-cni-k8s"},"ami-type":"AL2_x86_64","asg-min-size":3,"asg-max-size":3,"asg-desired-capacity":3,"instance-types":["m3.xlarge"]}}`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_MNGS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_ENABLE", `true`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_ENABLE")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}
	err := cfg.ValidateAndSetDefaults()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "older instance type InstanceTypes") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestEnvAddOnCSIEBS(t *testing.T) {
	cfg := NewDefault()
	defer func() {
		os.RemoveAll(cfg.ConfigPath)
		os.RemoveAll(cfg.KubectlCommandsOutputPath)
		os.RemoveAll(cfg.RemoteAccessCommandsOutputPath)
	}()

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE", `true`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CSI_EBS_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CSI_EBS_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CSI_EBS_CHART_REPO_URL", "test-chart-repo")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CSI_EBS_CHART_REPO_URL")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}
	err := cfg.ValidateAndSetDefaults()
	assert.NoError(t, err)

	if !cfg.AddOnCSIEBS.Enable {
		t.Fatalf("unexpected cfg.AddOnCSIEBS.Enable %v", cfg.AddOnCSIEBS.Enable)
	}
	if cfg.AddOnCSIEBS.ChartRepoURL != "test-chart-repo" {
		t.Fatalf("unexpected cfg.AddOnCSIEBS.ChartRepoURL %q", cfg.AddOnCSIEBS.ChartRepoURL)
	}
}

func TestEnvAddOnAppMesh(t *testing.T) {
	cfg := NewDefault()
	defer func() {
		os.RemoveAll(cfg.ConfigPath)
		os.RemoveAll(cfg.KubectlCommandsOutputPath)
		os.RemoveAll(cfg.RemoteAccessCommandsOutputPath)
	}()

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE", `true`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_ENABLE", `false`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_IRSA_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_APP_MESH_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_APP_MESH_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_APP_MESH_NAMESPACE", "custom-namespace")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_APP_MESH_NAMESPACE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_APP_MESH_CONTROLLER_IMAGE", "repo/controller:v1.1.3")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_APP_MESH_CONTROLLER_IMAGE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_APP_MESH_INJECTOR_IMAGE", "repo/injector:v1.1.3")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_APP_MESH_INJECTOR_IMAGE")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}
	err := cfg.ValidateAndSetDefaults()
	assert.NoError(t, err)

	assert.True(t, cfg.AddOnAppMesh.Enable)
	assert.Equal(t, cfg.AddOnAppMesh.Namespace, "custom-namespace")
	assert.Equal(t, cfg.AddOnAppMesh.ControllerImage, "repo/controller:v1.1.3")
	assert.Equal(t, cfg.AddOnAppMesh.InjectorImage, "repo/injector:v1.1.3")

	if cfg.AddOnAppMesh.PolicyCFNStackID != "" {
		t.Fatalf("read-only AddOnAppMesh.PolicyCFNStackID is set to %q", cfg.AddOnAppMesh.PolicyCFNStackID)
	}
}

func TestEnvAddOnWordpress(t *testing.T) {
	cfg := NewDefault()
	defer func() {
		os.RemoveAll(cfg.ConfigPath)
		os.RemoveAll(cfg.KubectlCommandsOutputPath)
		os.RemoveAll(cfg.RemoteAccessCommandsOutputPath)
	}()

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE", `true`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CSI_EBS_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CSI_EBS_ENABLE")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_WORDPRESS_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_WORDPRESS_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_WORDPRESS_NAMESPACE", "word-press")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_WORDPRESS_NAMESPACE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_WORDPRESS_USER_NAME", "my-user")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_WORDPRESS_USER_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_WORDPRESS_PASSWORD", "my-password")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_WORDPRESS_PASSWORD")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}
	err := cfg.ValidateAndSetDefaults()
	assert.NoError(t, err)

	if !cfg.AddOnWordpress.Enable {
		t.Fatalf("unexpected cfg.AddOnWordpress.Enable %v", cfg.AddOnWordpress.Enable)
	}
	if cfg.AddOnWordpress.Namespace != "word-press" {
		t.Fatalf("unexpected cfg.AddOnWordpress.Namespace %q", cfg.AddOnWordpress.Namespace)
	}
	if cfg.AddOnWordpress.UserName != "my-user" {
		t.Fatalf("unexpected cfg.AddOnWordpress.UserName %q", cfg.AddOnWordpress.UserName)
	}
	if cfg.AddOnWordpress.Password != "my-password" {
		t.Fatalf("unexpected cfg.AddOnWordpress.Password %q", cfg.AddOnWordpress.Password)
	}
}

func TestEnvAddOnKubernetesDashboard(t *testing.T) {
	cfg := NewDefault()
	defer func() {
		os.RemoveAll(cfg.ConfigPath)
		os.RemoveAll(cfg.KubectlCommandsOutputPath)
		os.RemoveAll(cfg.RemoteAccessCommandsOutputPath)
	}()

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE", `true`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_KUBERNETES_DASHBOARD_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_KUBERNETES_DASHBOARD_ENABLE")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}
	err := cfg.ValidateAndSetDefaults()
	assert.NoError(t, err)

	if !cfg.AddOnKubernetesDashboard.Enable {
		t.Fatalf("unexpected cfg.AddOnKubernetesDashboard.Enable %v", cfg.AddOnKubernetesDashboard.Enable)
	}
	fmt.Println(cfg.KubectlCommands())
}

func TestEnvAddOnPrometheusGrafana(t *testing.T) {
	cfg := NewDefault()
	defer func() {
		os.RemoveAll(cfg.ConfigPath)
		os.RemoveAll(cfg.KubectlCommandsOutputPath)
		os.RemoveAll(cfg.RemoteAccessCommandsOutputPath)
	}()

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE", `true`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_CSI_EBS_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_CSI_EBS_ENABLE")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_PROMETHEUS_GRAFANA_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_PROMETHEUS_GRAFANA_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_PROMETHEUS_GRAFANA_GRAFANA_ADMIN_USER_NAME", "MY_ADMIN_USER_NAME")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_PROMETHEUS_GRAFANA_GRAFANA_ADMIN_USER_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_PROMETHEUS_GRAFANA_GRAFANA_ADMIN_PASSWORD", "MY_ADMIN_PASSWORD")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_PROMETHEUS_GRAFANA_GRAFANA_ADMIN_PASSWORD")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}
	err := cfg.ValidateAndSetDefaults()
	assert.NoError(t, err)

	if !cfg.AddOnPrometheusGrafana.Enable {
		t.Fatalf("unexpected cfg.AddOnPrometheusGrafana.Enable %v", cfg.AddOnPrometheusGrafana.Enable)
	}
	if cfg.AddOnPrometheusGrafana.GrafanaAdminUserName != "MY_ADMIN_USER_NAME" {
		t.Fatalf("unexpected cfg.AddOnPrometheusGrafana.GrafanaAdminUserName %q", cfg.AddOnPrometheusGrafana.GrafanaAdminUserName)
	}
	if cfg.AddOnPrometheusGrafana.GrafanaAdminPassword != "MY_ADMIN_PASSWORD" {
		t.Fatalf("unexpected cfg.AddOnPrometheusGrafana.GrafanaAdminPassword %q", cfg.AddOnPrometheusGrafana.GrafanaAdminPassword)
	}
}

func TestEnvAddOnKubeflow(t *testing.T) {
	cfg := NewDefault()
	defer func() {
		os.RemoveAll(cfg.ConfigPath)
		os.RemoveAll(cfg.KubectlCommandsOutputPath)
		os.RemoveAll(cfg.RemoteAccessCommandsOutputPath)
	}()

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE", `true`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_KUBEFLOW_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_KUBEFLOW_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_KUBEFLOW_NAMESPACE", "kubeflow")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_KUBEFLOW_NAMESPACE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_KUBEFLOW_KFCTL_DOWNLOAD_URL", "kubeflow-download-here")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_KUBEFLOW_KFCTL_DOWNLOAD_URL")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_KUBEFLOW_BASE_DIR", "kubeflow-base-dir")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_KUBEFLOW_BASE_DIR")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}
	err := cfg.ValidateAndSetDefaults()
	assert.NoError(t, err)

	if !cfg.AddOnKubeflow.Enable {
		t.Fatalf("unexpected cfg.AddOnKubeflow.Enable %v", cfg.AddOnKubeflow.Enable)
	}
	if cfg.AddOnKubeflow.KfctlDownloadURL != "kubeflow-download-here" {
		t.Fatalf("unexpected cfg.AddOnKubeflow.KfctlDownloadURL %q", cfg.AddOnKubeflow.KfctlDownloadURL)
	}
	if cfg.AddOnKubeflow.BaseDir != "kubeflow-base-dir" {
		t.Fatalf("unexpected cfg.AddOnKubeflow.BaseDir %q", cfg.AddOnKubeflow.BaseDir)
	}
}
