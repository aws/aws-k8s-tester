package eksconfig

import (
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"time"
)

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
	os.Setenv("AWS_K8S_TESTER_EKS_EKS_RESOLVER_URL", "https://api.beta.us-west-2.wesley.amazonaws.com")
	os.Setenv("AWS_K8S_TESTER_EKS_TAG", "my-test")
	os.Setenv("AWS_K8S_TESTER_EKS_CF_STACK_VPC_PARAMETER_VPC_BLOCK", "192.168.0.0/8")
	os.Setenv("AWS_K8S_TESTER_EKS_CF_STACK_VPC_PARAMETER_SUBNET_01_BLOCK", "192.168.64.0/8")
	os.Setenv("AWS_K8S_TESTER_EKS_CF_STACK_VPC_PARAMETER_SUBNET_02_BLOCK", "192.168.128.0/8")
	os.Setenv("AWS_K8S_TESTER_EKS_CF_STACK_VPC_PARAMETER_SUBNET_03_BLOCK", "192.168.192.0/8")
	os.Setenv("AWS_K8S_TESTER_EKS_VPC_ID", "my-vpc-id")
	os.Setenv("AWS_K8S_TESTER_EKS_SUBNET_IDS", "a,b,c")
	os.Setenv("AWS_K8S_TESTER_EKS_LOG_ACCESS", "true")
	os.Setenv("AWS_K8S_TESTER_EKS_WORKER_NODE_PRIVATE_KEY_PATH", "/tmp/aws-k8s-tester/worker-node.ssh.private.key.3")
	os.Setenv("AWS_K8S_TESTER_EKS_WORKER_NODE_AMI_TYPE", "amazon-linux-2-gpu")
	os.Setenv("AWS_K8S_TESTER_EKS_SECURITY_GROUP_ID", "my-security-id")
	os.Setenv("AWS_K8S_TESTER_EKS_ENABLE_WORKER_NODE_HA", "false")
	os.Setenv("AWS_K8S_TESTER_EKS_ENABLE_WORKER_NODE_SSH", "true")
	os.Setenv("AWS_K8S_TESTER_EKS_ENABLE_WORKER_NODE_PRIVILEGED_PORT_ACCESS", "true")
	os.Setenv("AWS_K8S_TESTER_EKS_CONFIG_PATH", "test-path")
	os.Setenv("AWS_K8S_TESTER_EKS_WORKER_NODE_ASG_MIN", "5")
	os.Setenv("AWS_K8S_TESTER_EKS_WORKER_NODE_ASG_MAX", "10")
	os.Setenv("AWS_K8S_TESTER_EKS_WORKER_NODE_ASG_DESIRED_CAPACITY", "7")
	os.Setenv("AWS_K8S_TESTER_EKS_LOG_LEVEL", "debug")
	os.Setenv("AWS_K8S_TESTER_EKS_UPLOAD_TESTER_LOGS", "true")
	os.Setenv("AWS_K8S_TESTER_EKS_UPLOAD_KUBECONFIG", "true")
	os.Setenv("AWS_K8S_TESTER_EKS_UPLOAD_WORKER_NODE_LOGS", "true")
	os.Setenv("AWS_K8S_TESTER_EKS_UPLOAD_BUCKET_EXPIRE_DAYS", "3")
	os.Setenv("AWS_K8S_TESTER_EKS_DESTROY_AFTER_CREATE", "true")
	os.Setenv("AWS_K8S_TESTER_EKS_DESTROY_WAIT_TIME", "2h")
	os.Setenv("AWS_K8S_TESTER_EKS_EKS_REQUEST_HEADER", "a=b,eks-options=a;b;c")
	os.Setenv("AWS_K8S_TESTER_EKS_WORKER_NODE_CF_TEMPLATE_PATH", "/tmp/template.yaml")
	os.Setenv("AWS_K8S_TESTER_EKS_WORKER_NODE_CF_TEMPLATE_ADDITIONAL_PARAMETER_KEYS", "CertificateAuthorityData,ApiServerEndpoint")

	defer func() {
		os.Unsetenv("AWS_K8S_TESTER_EKS_AWS_K8S_TESTER_DOWNLOAD_URL")
		os.Unsetenv("AWS_K8S_TESTER_EKS_AWS_K8S_TESTER_PATH")
		os.Unsetenv("AWS_K8S_TESTER_EKS_KUBECTL_DOWNLOAD_URL")
		os.Unsetenv("AWS_K8S_TESTER_EKS_KUBECTL_PATH")
		os.Unsetenv("AWS_K8S_TESTER_EKS_KUBECONFIG_PATH")
		os.Unsetenv("AWS_K8S_TESTER_EKS_AWS_IAM_AUTHENTICATOR_DOWNLOAD_URL")
		os.Unsetenv("AWS_K8S_TESTER_EKS_AWS_IAM_AUTHENTICATOR_PATH")
		os.Unsetenv("AWS_K8S_TESTER_EKS_KUBERNETES_VERSION")
		os.Unsetenv("AWS_K8S_TESTER_EKS_EKS_RESOLVER_URL")
		os.Unsetenv("AWS_K8S_TESTER_EKS_TAG")
		os.Unsetenv("AWS_K8S_TESTER_EKS_CF_STACK_VPC_PARAMETER_VPC_BLOCK")
		os.Unsetenv("AWS_K8S_TESTER_EKS_CF_STACK_VPC_PARAMETER_SUBNET_01_BLOCK")
		os.Unsetenv("AWS_K8S_TESTER_EKS_CF_STACK_VPC_PARAMETER_SUBNET_02_BLOCK")
		os.Unsetenv("AWS_K8S_TESTER_EKS_CF_STACK_VPC_PARAMETER_SUBNET_03_BLOCK")
		os.Unsetenv("AWS_K8S_TESTER_EKS_VPC_ID")
		os.Unsetenv("AWS_K8S_TESTER_EKS_SUBNET_IDs")
		os.Unsetenv("AWS_K8S_TESTER_EKS_LOG_ACCESS")
		os.Unsetenv("AWS_K8S_TESTER_EKS_WORKER_NODE_PRIVATE_KEY_PATH")
		os.Unsetenv("AWS_K8S_TESTER_EKS_WORKER_NODE_AMI_TYPE")
		os.Unsetenv("AWS_K8S_TESTER_EKS_SECURITY_GROUP_ID")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ENABLE_WORKER_NODE_HA")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ENABLE_WORKER_NODE_SSH")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ENABLE_WORKER_NODE_PRIVILEGED_PORT_ACCESS")
		os.Unsetenv("AWS_K8S_TESTER_EKS_CONFIG_PATH")
		os.Unsetenv("AWS_K8S_TESTER_EKS_WORKER_NODE_ASG_MIN")
		os.Unsetenv("AWS_K8S_TESTER_EKS_WORKER_NODE_ASG_MAX")
		os.Unsetenv("AWS_K8S_TESTER_EKS_WORKER_NODE_ASG_DESIRED_CAPACITY")
		os.Unsetenv("AWS_K8S_TESTER_EKS_LOG_LEVEL")
		os.Unsetenv("AWS_K8S_TESTER_EKS_UPLOAD_TESTER_LOGS")
		os.Unsetenv("AWS_K8S_TESTER_EKS_UPLOAD_KUBECONFIG")
		os.Unsetenv("AWS_K8S_TESTER_EKS_UPLOAD_WORKER_NODE_LOGS")
		os.Unsetenv("AWS_K8S_TESTER_EKS_UPLOAD_BUCKET_EXPIRE_DAYS")
		os.Unsetenv("AWS_K8S_TESTER_EKS_DESTROY_AFTER_CREATE")
		os.Unsetenv("AWS_K8S_TESTER_EKS_DESTROY_WAIT_TIME")
		os.Unsetenv("AWS_K8S_TESTER_EKS_EKS_REQUEST_HEADER")
		os.Unsetenv("AWS_K8S_TESTER_EKS_WORKER_NODE_CF_TEMPLATE_PATH")
		os.Unsetenv("AWS_K8S_TESTER_EKS_WORKER_NODE_CF_TEMPLATE_ADDITIONAL_PARAMETER_KEYS")
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
	if cfg.EKSResolverURL != "https://api.beta.us-west-2.wesley.amazonaws.com" {
		t.Fatalf("unexpected EKSResolverURL %q", cfg.EKSResolverURL)
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
	if cfg.WorkerNodeAMIType != "amazon-linux-2-gpu" {
		t.Fatalf("WorkerNodeAMIType expected amazon-linux-2-gpu, got %q", cfg.WorkerNodeAMIType)
	}
	if cfg.SecurityGroupID != "my-security-id" {
		t.Fatalf("SecurityGroupID my-id, got %q", cfg.SecurityGroupID)
	}
	if cfg.ConfigPath != "test-path" {
		t.Fatalf("alb configuration path expected 'test-path', got %q", cfg.ConfigPath)
	}
	if !cfg.DestroyAfterCreate {
		t.Fatalf("DestroyAfterCreate expected 'true', got %v", cfg.DestroyAfterCreate)
	}
	if cfg.DestroyWaitTime != 2*time.Hour {
		t.Fatalf("DestroyWaitTime expected 2h, got %v", cfg.DestroyWaitTime)
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
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel unexpected %q", cfg.LogLevel)
	}
	if !cfg.UploadTesterLogs {
		t.Fatalf("UploadTesterLogs expected true, got %v", cfg.UploadTesterLogs)
	}
	if !cfg.UploadKubeConfig {
		t.Fatalf("UploadKubeConfig expected true, got %v", cfg.UploadKubeConfig)
	}
	if !cfg.UploadWorkerNodeLogs {
		t.Fatalf("UploadWorkerNodeLogs expected true, got %v", cfg.UploadWorkerNodeLogs)
	}
	if cfg.UploadBucketExpireDays != 3 {
		t.Fatalf("UploadBucketExpireDays expected 3, got %d", cfg.UploadBucketExpireDays)
	}
	rh := map[string]string{
		"a":           "b",
		"eks-options": "a;b;c",
	}
	if !reflect.DeepEqual(cfg.EKSRequestHeader, rh) {
		t.Fatalf("EKSRequestHeader expected %v, got %v", rh, cfg.EKSRequestHeader)
	}
	if cfg.WorkerNodeCFTemplatePath != "/tmp/template.yaml" {
		t.Fatalf("WorkerNodeCFTemplatePath expected /tmp/template.yaml, got %q", cfg.WorkerNodeCFTemplatePath)
	}
	pks := []string{"CertificateAuthorityData", "ApiServerEndpoint"}
	if !reflect.DeepEqual(cfg.WorkerNodeCFTemplateAdditionalParameterKeys, pks) {
		t.Fatalf("WorkerNodeCFTemplateAdditionalParameterKeys expected %v, got %v", pks, cfg.WorkerNodeCFTemplateAdditionalParameterKeys)
	}

	if err := cfg.Sync(); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(cfg.ConfigPath)
	d, err := ioutil.ReadFile(cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(d))

	fmt.Println(cfg.KubectlCommands())
}
