package eksconfig

import (
	"os"
	"reflect"
	"testing"
	"time"
)

func TestConfig(t *testing.T) {
	cfg := NewDefault()

	// supports only 58 pods per node
	cfg.WorkerNodeInstanceType = "m5.xlarge"
	cfg.WorkerNodeASGMin = 10
	cfg.WorkerNodeASGMax = 10
	cfg.ALBIngressController.TestServerReplicas = 600

	err := cfg.ValidateAndSetDefaults()
	if err == nil {
		t.Fatal("expected error")
	}
	t.Log(err)
}

func TestEnv(t *testing.T) {
	cfg := NewDefault()

	os.Setenv("AWS_K8S_TESTER_EKS_AWS_K8S_TESTER_DOWNLOAD_URL", "https://github.com/aws/aws-k8s-tester/releases/download/0.1.5/aws-k8s-tester-0.1.5-linux-amd64")
	os.Setenv("AWS_K8S_TESTER_EKS_AWS_K8S_TESTER_PATH", "/tmp/aws-k8s-tester-test/aws-k8s-tester")
	os.Setenv("AWS_K8S_TESTER_EKS_KUBECTL_DOWNLOAD_URL", "https://amazon-eks.s3-us-west-2.amazonaws.com/1.11.5/2018-12-06/bin/linux/amd64/kubectl")
	os.Setenv("AWS_K8S_TESTER_EKS_KUBECTL_PATH", "/tmp/aws-k8s-tester-test/kubectl")
	os.Setenv("AWS_K8S_TESTER_EKS_KUBECONFIG_PATH", "/tmp/aws-k8s-tester/kubeconfig2")
	os.Setenv("AWS_K8S_TESTER_EKS_AWS_IAM_AUTHENTICATOR_DOWNLOAD_URL", "https://amazon-eks.s3-us-west-2.amazonaws.com/1.11.5/2018-12-06/bin/linux/amd64/aws-iam-authenticator")
	os.Setenv("AWS_K8S_TESTER_EKS_AWS_IAM_AUTHENTICATOR_PATH", "/tmp/aws-k8s-tester-test/aws-iam-authenticator")
	os.Setenv("AWS_K8S_TESTER_EKS_KUBERNETES_VERSION", "1.11")
	os.Setenv("AWS_K8S_TESTER_EKS_TAG", "my-test")
	os.Setenv("AWS_K8S_TESTER_EKS_CF_STACK_VPC_PARAMETER_VPC_BLOCK", "192.168.0.0/8")
	os.Setenv("AWS_K8S_TESTER_EKS_CF_STACK_VPC_PARAMETER_SUBNET_01_BLOCK", "192.168.64.0/8")
	os.Setenv("AWS_K8S_TESTER_EKS_CF_STACK_VPC_PARAMETER_SUBNET_02_BLOCK", "192.168.128.0/8")
	os.Setenv("AWS_K8S_TESTER_EKS_CF_STACK_VPC_PARAMETER_SUBNET_03_BLOCK", "192.168.192.0/8")
	os.Setenv("AWS_K8S_TESTER_EKS_VPC_ID", "my-vpc-id")
	os.Setenv("AWS_K8S_TESTER_EKS_SUBNET_IDS", "a,b,c")
	os.Setenv("AWS_K8S_TESTER_EKS_LOG_ACCESS", "true")
	os.Setenv("AWS_K8S_TESTER_EKS_WORKER_NODE_PRIVATE_KEY_PATH", "/tmp/aws-k8s-tester/worker-node.ssh.private.key.3")
	os.Setenv("AWS_K8S_TESTER_EKS_WORKER_NODE_AMI", "test-ami")
	os.Setenv("AWS_K8S_TESTER_EKS_SECURITY_GROUP_ID", "my-security-id")
	os.Setenv("AWS_K8S_TESTER_EKS_ENABLE_WORKER_NODE_HA", "false")
	os.Setenv("AWS_K8S_TESTER_EKS_ENABLE_WORKER_NODE_SSH", "true")
	os.Setenv("AWS_K8S_TESTER_EKS_ENABLE_WORKER_NODE_PRIVILEGED_PORT_ACCESS", "true")
	os.Setenv("AWS_K8S_TESTER_EKS_CONFIG_PATH", "test-path")
	os.Setenv("AWS_K8S_TESTER_EKS_DOWN", "false")
	os.Setenv("AWS_K8S_TESTER_EKS_ALB_TARGET_TYPE", "ip")
	os.Setenv("AWS_K8S_TESTER_EKS_WORKER_NODE_ASG_MIN", "5")
	os.Setenv("AWS_K8S_TESTER_EKS_WORKER_NODE_ASG_MAX", "10")
	os.Setenv("AWS_K8S_TESTER_EKS_WORKER_NODE_ASG_DESIRED_CAPACITY", "7")
	os.Setenv("AWS_K8S_TESTER_EKS_LOG_DEBUG", "true")
	os.Setenv("AWS_K8S_TESTER_EKS_UPLOAD_TESTER_LOGS", "true")
	os.Setenv("AWS_K8S_TESTER_EKS_UPLOAD_KUBECONFIG", "true")
	os.Setenv("AWS_K8S_TESTER_EKS_UPLOAD_WORKER_NODE_LOGS", "true")
	os.Setenv("AWS_K8S_TESTER_EKS_UPLOAD_BUCKET_EXPIRE_DAYS", "3")
	os.Setenv("AWS_K8S_TESTER_EKS_WAIT_BEFORE_DOWN", "2h")
	os.Setenv("AWS_K8S_TESTER_EKS_ALB_TEST_SCALABILITY_MINUTES", "3")
	os.Setenv("AWS_K8S_TESTER_EKS_ALB_UPLOAD_TESTER_LOGS", "true")
	os.Setenv("AWS_K8S_TESTER_EKS_ALB_TEST_EXPECT_QPS", "123.45")
	os.Setenv("AWS_K8S_TESTER_EKS_ALB_ENABLE", "true")
	os.Setenv("AWS_K8S_TESTER_EKS_ALB_TEST_SCALABILITY", "false")
	os.Setenv("AWS_K8S_TESTER_EKS_ALB_TEST_METRICS", "false")
	os.Setenv("AWS_K8S_TESTER_EKS_ALB_INGRESS_CONTROLLER_IMAGE", "quay.io/coreos/alb-ingress-controller:1.0-beta.7")

	defer func() {
		os.Unsetenv("AWS_K8S_TESTER_EKS_AWS_K8S_TESTER_DOWNLOAD_URL")
		os.Unsetenv("AWS_K8S_TESTER_EKS_AWS_K8S_TESTER_PATH")
		os.Unsetenv("AWS_K8S_TESTER_EKS_KUBECTL_DOWNLOAD_URL")
		os.Unsetenv("AWS_K8S_TESTER_EKS_KUBECTL_PATH")
		os.Unsetenv("AWS_K8S_TESTER_EKS_KUBECONFIG_PATH")
		os.Unsetenv("AWS_K8S_TESTER_EKS_AWS_IAM_AUTHENTICATOR_DOWNLOAD_URL")
		os.Unsetenv("AWS_K8S_TESTER_EKS_AWS_IAM_AUTHENTICATOR_PATH")
		os.Unsetenv("AWS_K8S_TESTER_EKS_KUBERNETES_VERSION")
		os.Unsetenv("AWS_K8S_TESTER_EKS_TAG")
		os.Unsetenv("AWS_K8S_TESTER_EKS_CF_STACK_VPC_PARAMETER_VPC_BLOCK")
		os.Unsetenv("AWS_K8S_TESTER_EKS_CF_STACK_VPC_PARAMETER_SUBNET_01_BLOCK")
		os.Unsetenv("AWS_K8S_TESTER_EKS_CF_STACK_VPC_PARAMETER_SUBNET_02_BLOCK")
		os.Unsetenv("AWS_K8S_TESTER_EKS_CF_STACK_VPC_PARAMETER_SUBNET_03_BLOCK")
		os.Unsetenv("AWS_K8S_TESTER_EKS_VPC_ID")
		os.Unsetenv("AWS_K8S_TESTER_EKS_SUBNET_IDs")
		os.Unsetenv("AWS_K8S_TESTER_EKS_LOG_ACCESS")
		os.Unsetenv("AWS_K8S_TESTER_EKS_WORKER_NODE_PRIVATE_KEY_PATH")
		os.Unsetenv("AWS_K8S_TESTER_EKS_WORKER_NODE_AMI")
		os.Unsetenv("AWS_K8S_TESTER_EKS_SECURITY_GROUP_ID")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ENABLE_WORKER_NODE_HA")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ENABLE_WORKER_NODE_SSH")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ENABLE_WORKER_NODE_PRIVILEGED_PORT_ACCESS")
		os.Unsetenv("AWS_K8S_TESTER_EKS_CONFIG_PATH")
		os.Unsetenv("AWS_K8S_TESTER_EKS_DOWN")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ALB_TARGET_TYPE")
		os.Unsetenv("AWS_K8S_TESTER_EKS_WORKER_NODE_ASG_MIN")
		os.Unsetenv("AWS_K8S_TESTER_EKS_WORKER_NODE_ASG_MAX")
		os.Unsetenv("AWS_K8S_TESTER_EKS_WORKER_NODE_ASG_DESIRED_CAPACITY")
		os.Unsetenv("AWS_K8S_TESTER_EKS_LOG_DEBUG")
		os.Unsetenv("AWS_K8S_TESTER_EKS_UPLOAD_TESTER_LOGS")
		os.Unsetenv("AWS_K8S_TESTER_EKS_UPLOAD_KUBECONFIG")
		os.Unsetenv("AWS_K8S_TESTER_EKS_UPLOAD_WORKER_NODE_LOGS")
		os.Unsetenv("AWS_K8S_TESTER_EKS_UPLOAD_BUCKET_EXPIRE_DAYS")
		os.Unsetenv("AWS_K8S_TESTER_EKS_WAIT_BEFORE_DOWN")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ALB_TEST_SCALABILITY_MINUTES")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ALB_UPLOAD_TESTER_LOGS")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ALB_TEST_EXPECT_QPS")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ALB_ENABLE")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ALB_TEST_SCALABILITY")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ALB_TEST_METRICS")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ALB_INGRESS_CONTROLLER_IMAGE")
	}()

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}

	if cfg.AWSK8sTesterDownloadURL != "https://github.com/aws/aws-k8s-tester/releases/download/0.1.5/aws-k8s-tester-0.1.5-linux-amd64" {
		t.Fatalf("unexpected AWSK8sTesterDownloadURL %q", cfg.AWSK8sTesterDownloadURL)
	}
	if cfg.AWSK8sTesterPath != "/tmp/aws-k8s-tester-test/aws-k8s-tester" {
		t.Fatalf("unexpected AWSK8sTesterPath %q", cfg.AWSK8sTesterPath)
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
	if cfg.AWSIAMAuthenticatorDownloadURL != "https://amazon-eks.s3-us-west-2.amazonaws.com/1.11.5/2018-12-06/bin/linux/amd64/aws-iam-authenticator" {
		t.Fatalf("unexpected AWSIAMAuthenticatorDownloadURL %q", cfg.AWSIAMAuthenticatorDownloadURL)
	}
	if cfg.AWSIAMAuthenticatorPath != "/tmp/aws-k8s-tester-test/aws-iam-authenticator" {
		t.Fatalf("unexpected AWSIAMAuthenticatorPath %q", cfg.AWSIAMAuthenticatorPath)
	}
	if cfg.KubernetesVersion != "1.11" {
		t.Fatalf("KubernetesVersion 1.11, got %q", cfg.KubernetesVersion)
	}
	if cfg.Tag != "my-test" {
		t.Fatalf("Tag my-test, got %q", cfg.Tag)
	}
	if cfg.CFStackVPCParameterVPCBlock != "192.168.0.0/8" {
		t.Fatalf("CFStackVPCParameterVPCBlock unexpected %q", cfg.CFStackVPCParameterVPCBlock)
	}
	if cfg.CFStackVPCParameterSubnet01Block != "192.168.64.0/8" {
		t.Fatalf("CFStackVPCParameterSubnet01Block unexpected %q", cfg.CFStackVPCParameterSubnet01Block)
	}
	if cfg.CFStackVPCParameterSubnet02Block != "192.168.128.0/8" {
		t.Fatalf("CFStackVPCParameterSubnet02Block unexpected %q", cfg.CFStackVPCParameterSubnet02Block)
	}
	if cfg.CFStackVPCParameterSubnet03Block != "192.168.192.0/8" {
		t.Fatalf("CFStackVPCParameterSubnet03Block unexpected %q", cfg.CFStackVPCParameterSubnet03Block)
	}
	if cfg.VPCID != "my-vpc-id" {
		t.Fatalf("VPCID my-vpc-id, got %q", cfg.VPCID)
	}
	if !reflect.DeepEqual(cfg.SubnetIDs, []string{"a", "b", "c"}) {
		t.Fatalf("SubnetIDs expected a,b,c, got %q", cfg.SubnetIDs)
	}
	if !cfg.LogAccess {
		t.Fatalf("LogAccess expected true, got %v", cfg.LogAccess)
	}
	if cfg.WorkerNodePrivateKeyPath != "/tmp/aws-k8s-tester/worker-node.ssh.private.key.3" {
		t.Fatalf("WorkerNodePrivateKeyPath expected /tmp/aws-k8s-tester/worker-node.ssh.private.key.3, got %q", cfg.WorkerNodePrivateKeyPath)
	}
	if cfg.WorkerNodeAMI != "test-ami" {
		t.Fatalf("WorkerNodeAMI expected test-ami, got %q", cfg.WorkerNodeAMI)
	}
	if cfg.SecurityGroupID != "my-security-id" {
		t.Fatalf("SecurityGroupID my-id, got %q", cfg.SecurityGroupID)
	}
	if cfg.ConfigPath != "test-path" {
		t.Fatalf("alb configuration path expected 'test-path', got %q", cfg.ConfigPath)
	}
	if cfg.Down {
		t.Fatalf("cfg.Down expected 'false', got %v", cfg.Down)
	}
	if cfg.EnableWorkerNodeHA {
		t.Fatalf("cfg.EnableWorkerNodeHA expected 'false', got %v", cfg.EnableWorkerNodeHA)
	}
	if !cfg.EnableWorkerNodeSSH {
		t.Fatalf("cfg.EnableWorkerNodeSSH expected 'true', got %v", cfg.EnableWorkerNodeSSH)
	}
	if !cfg.EnableWorkerNodePrivilegedPortAccess {
		t.Fatalf("cfg.EnableWorkerNodePrivilegedPortAccess expected 'true', got %v", cfg.EnableWorkerNodePrivilegedPortAccess)
	}
	if cfg.WorkerNodeASGMin != 5 {
		t.Fatalf("worker nodes min expected 5, got %q", cfg.WorkerNodeASGMin)
	}
	if cfg.WorkerNodeASGMax != 10 {
		t.Fatalf("worker nodes max expected 10, got %q", cfg.WorkerNodeASGMax)
	}
	if cfg.WorkerNodeASGDesiredCapacity != 7 {
		t.Fatalf("worker nodes desired capacity expected 7, got %q", cfg.WorkerNodeASGDesiredCapacity)
	}
	if cfg.ALBIngressController.TestScalabilityMinutes != 3 {
		t.Fatalf("alb target type expected 3, got %d", cfg.ALBIngressController.TestScalabilityMinutes)
	}
	if cfg.ALBIngressController.TargetType != "ip" {
		t.Fatalf("alb target type expected ip, got %q", cfg.ALBIngressController.TargetType)
	}
	if !cfg.LogDebug {
		t.Fatalf("LogDebug expected true, got %v", cfg.LogDebug)
	}
	if !cfg.UploadTesterLogs {
		t.Fatalf("UploadTesterLogs expected true, got %v", cfg.UploadTesterLogs)
	}
	if !cfg.UploadKubeConfig {
		t.Fatalf("UploadKubeConfig expected true, got %v", cfg.UploadKubeConfig)
	}
	if !cfg.ALBIngressController.UploadTesterLogs {
		t.Fatalf("ALBIngressController.UploadTesterLogs expected true, got %v", cfg.ALBIngressController.UploadTesterLogs)
	}
	if !cfg.UploadWorkerNodeLogs {
		t.Fatalf("UploadWorkerNodeLogs expected true, got %v", cfg.UploadWorkerNodeLogs)
	}
	if cfg.UploadBucketExpireDays != 3 {
		t.Fatalf("UploadBucketExpireDays expected 3, got %d", cfg.UploadBucketExpireDays)
	}
	if cfg.WaitBeforeDown != 2*time.Hour {
		t.Fatalf("wait before down expected 2h, got %v", cfg.WaitBeforeDown)
	}
	if cfg.ALBIngressController.IngressControllerImage != "quay.io/coreos/alb-ingress-controller:1.0-beta.7" {
		t.Fatalf("cfg.ALBIngressController.IngressControllerImage expected 'quay.io/coreos/alb-ingress-controller:1.0-beta.7', got %q", cfg.ALBIngressController.IngressControllerImage)
	}
	if cfg.ALBIngressController.TestExpectQPS != 123.45 {
		t.Fatalf("cfg.ALBIngressController.TestExpectQPS expected 123.45, got %v", cfg.ALBIngressController.TestExpectQPS)
	}
	if !cfg.ALBIngressController.Enable {
		t.Fatalf("cfg.ALBIngressController.Enable expected 'true', got %v", cfg.ALBIngressController.Enable)
	}
	if cfg.ALBIngressController.TestScalability {
		t.Fatalf("cfg.ALBIngressController.TestScalability expected 'false', got %v", cfg.ALBIngressController.TestScalability)
	}
	if cfg.ALBIngressController.TestMetrics {
		t.Fatalf("cfg.ALBIngressController.TestMetrics expected 'false', got %v", cfg.ALBIngressController.TestMetrics)
	}
}
