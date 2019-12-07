package eksconfig

import (
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestEnv(t *testing.T) {
	cfg := NewDefault()
	kubectlDownloadURL := "https://amazon-eks.s3-us-west-2.amazonaws.com/1.11.5/2018-12-06/bin/linux/amd64/kubectl"
	if runtime.GOOS == "darwin" {
		kubectlDownloadURL = strings.Replace(kubectlDownloadURL, "linux", runtime.GOOS, -1)
	}

	os.Setenv("AWS_K8S_TESTER_EKS_REGION", "us-east-1")
	os.Setenv("AWS_K8S_TESTER_EKS_LOG_LEVEL", "debug")
	os.Setenv("AWS_K8S_TESTER_EKS_KUBECTL_DOWNLOAD_URL", kubectlDownloadURL)
	os.Setenv("AWS_K8S_TESTER_EKS_KUBECTL_PATH", "/tmp/aws-k8s-tester-test/kubectl")
	os.Setenv("AWS_K8S_TESTER_EKS_KUBECONFIG_PATH", "/tmp/aws-k8s-tester/kubeconfig2")

	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_CLUSTER_TAGS", "to-delete=2019")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_MANAGED_NODE_GROUP_TAGS", "hello=world")

	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_CLUSTER_REQUEST_HEADER_KEY", "eks-options")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_CLUSTER_REQUEST_HEADER_VALUE", "kubernetesVersion=1.11")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_MANAGED_NODE_GROUP_REQUEST_HEADER_KEY", "a")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_MANAGED_NODE_GROUP_REQUEST_HEADER_VALUE", "b")

	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_CLUSTER_ROLE_SERVICE_PRINCIPALS", "eks.amazonaws.com,eks-beta-pdx.aws.internal,eks-dev.aws.internal")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_CLUSTER_ROLE_MANAGED_POLICY_ARNS", "arn:aws:iam::aws:policy/AmazonEKSServicePolicy,arn:aws:iam::aws:policy/AmazonEKSClusterPolicy,arn:aws:iam::aws:policy/service-role/AmazonEC2RoleforSSM")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_MANAGED_NODE_GROUP_ROLE_SERVICE_PRINCIPALS", "ec2.amazonaws.com,eks.amazonaws.com,hello.amazonaws.com")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_MANAGED_NODE_GROUP_ROLE_MANAGED_POLICY_ARNS", "a,b,c")

	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_VERSION", "1.11")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_ENABLE", "true")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_URL", "invalid")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_ENABLE", "true")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_URL", "invalid")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_POLICY_NAME", "my-policy")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_POLICY_CFN_STACK_ID", "my-id")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_PERL_ENABLE", "true")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_PERL_COMPLETES", "100")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_PERL_PARALLELS", "10")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_ECHO_ENABLE", "true")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_ECHO_COMPLETES", "1000")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_ECHO_PARALLELS", "100")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_ECHO_SIZE", "10000")

	defer func() {
		os.Unsetenv("AWS_K8S_TESTER_EKS_REGION")
		os.Unsetenv("AWS_K8S_TESTER_EKS_LOG_LEVEL")
		os.Unsetenv("AWS_K8S_TESTER_EKS_KUBECTL_DOWNLOAD_URL")
		os.Unsetenv("AWS_K8S_TESTER_EKS_KUBECTL_PATH")
		os.Unsetenv("AWS_K8S_TESTER_EKS_KUBECONFIG_PATH")

		os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_REQUEST_HEADER")
		os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_CLUSTER_TAGS")
		os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_MANAGED_NODE_GROUP_REQUEST_HEADER")
		os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_MANAGED_NODE_GROUP_TAGS")

		os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_CLUSTER_REQUEST_HEADER_KEY")
		os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_CLUSTER_REQUEST_HEADER_VALUE")
		os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_MANAGED_NODE_GROUP_REQUEST_HEADER_KEY")
		os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_MANAGED_NODE_GROUP_REQUEST_HEADER_VALUE")

		os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_CLUSTER_ROLE_SERVICE_PRINCIPALS")
		os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_CLUSTER_ROLE_MANAGED_POLICY_ARNS")
		os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_MANAGED_NODE_GROUP_ROLE_SERVICE_PRINCIPALS")
		os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_MANAGED_NODE_GROUP_ROLE_MANAGED_POLICY_ARNS")
		os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_VERSION")

		os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_ENABLE")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_URL")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_ENABLE")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_URL")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_POLICY_NAME")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_POLICY_CFN_STACK_ID")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_PERL_ENABLE")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_PERL_COMPLETES")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_PERL_PARALLELS")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_ECHO_ENABLE")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_ECHO_COMPLETES")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_ECHO_PARALLELS")
		os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_ECHO_SIZE")
	}()

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}

	if cfg.Region != "us-east-1" {
		t.Fatalf("unexpected %q", cfg.Region)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("unexpected %q", cfg.LogLevel)
	}

	expectedClusterRoleServicePrincipals := []string{
		"eks.amazonaws.com",
		"eks-beta-pdx.aws.internal",
		"eks-dev.aws.internal",
	}
	if !reflect.DeepEqual(expectedClusterRoleServicePrincipals, cfg.Parameters.ClusterRoleServicePrincipals) {
		t.Fatalf("unexpected Parameters.ClusterRoleServicePrincipals %+v", cfg.Parameters.ClusterRoleServicePrincipals)
	}
	expectedClusterRoleManagedPolicyARNs := []string{
		"arn:aws:iam::aws:policy/AmazonEKSServicePolicy",
		"arn:aws:iam::aws:policy/AmazonEKSClusterPolicy",
		"arn:aws:iam::aws:policy/service-role/AmazonEC2RoleforSSM",
	}
	if !reflect.DeepEqual(expectedClusterRoleManagedPolicyARNs, cfg.Parameters.ClusterRoleManagedPolicyARNs) {
		t.Fatalf("unexpected Parameters.ClusterRoleManagedPolicyARNs %+v", cfg.Parameters.ClusterRoleManagedPolicyARNs)
	}
	expectedManagedNodeGroupRoleServicePrincipals := []string{
		"ec2.amazonaws.com",
		"eks.amazonaws.com",
		"hello.amazonaws.com",
	}
	if !reflect.DeepEqual(expectedManagedNodeGroupRoleServicePrincipals, cfg.Parameters.ManagedNodeGroupRoleServicePrincipals) {
		t.Fatalf("unexpected Parameters.ManagedNodeGroupRoleServicePrincipals %+v", cfg.Parameters.ManagedNodeGroupRoleServicePrincipals)
	}
	expectedManagedNodeGroupRoleManagedPolicyARNs := []string{
		"a",
		"b",
		"c",
	}
	if !reflect.DeepEqual(expectedManagedNodeGroupRoleManagedPolicyARNs, cfg.Parameters.ManagedNodeGroupRoleManagedPolicyARNs) {
		t.Fatalf("unexpected Parameters.ManagedNodeGroupRoleManagedPolicyARNs %+v", cfg.Parameters.ManagedNodeGroupRoleManagedPolicyARNs)
	}

	if cfg.Parameters.Version != "1.11" {
		t.Fatalf("unexpected Parameters.Version %q", cfg.Parameters.Version)
	}

	expectedClusterTags := map[string]string{"to-delete": "2019"}
	if !reflect.DeepEqual(cfg.Parameters.ClusterTags, expectedClusterTags) {
		t.Fatalf("Tags expected %v, got %v", expectedClusterTags, cfg.Parameters.ClusterTags)
	}
	expectedManagedNodeGroupTags := map[string]string{"hello": "world"}
	if !reflect.DeepEqual(cfg.Parameters.ManagedNodeGroupTags, expectedManagedNodeGroupTags) {
		t.Fatalf("Tags expected %v, got %v", expectedManagedNodeGroupTags, cfg.Parameters.ManagedNodeGroupTags)
	}

	if cfg.Parameters.ClusterRequestHeaderKey != "eks-options" {
		t.Fatalf("unexpected Parameters.ClusterRequestHeaderKey %q", cfg.Parameters.ClusterRequestHeaderKey)
	}
	if cfg.Parameters.ClusterRequestHeaderValue != "kubernetesVersion=1.11" {
		t.Fatalf("unexpected Parameters.ClusterRequestHeaderValue %q", cfg.Parameters.ClusterRequestHeaderValue)
	}
	if cfg.Parameters.ManagedNodeGroupRequestHeaderKey != "a" {
		t.Fatalf("unexpected Parameters.ManagedNodeGroupRequestHeaderKey %q", cfg.Parameters.ManagedNodeGroupRequestHeaderKey)
	}
	if cfg.Parameters.ManagedNodeGroupRequestHeaderValue != "b" {
		t.Fatalf("unexpected Parameters.ManagedNodeGroupRequestHeaderValue %q", cfg.Parameters.ManagedNodeGroupRequestHeaderValue)
	}

	if cfg.KubectlDownloadURL != kubectlDownloadURL {
		t.Fatalf("unexpected KubectlDownloadURL %q", cfg.KubectlDownloadURL)
	}
	if cfg.KubectlPath != "/tmp/aws-k8s-tester-test/kubectl" {
		t.Fatalf("unexpected KubectlPath %q", cfg.KubectlPath)
	}
	if cfg.KubeConfigPath != "/tmp/aws-k8s-tester/kubeconfig2" {
		t.Fatalf("unexpected KubeConfigPath %q", cfg.KubeConfigPath)
	}

	if !cfg.AddOnNLBHelloWorld.Enable {
		t.Fatalf("unexpected cfg.AddOnNLBHelloWorld.Enable %v", cfg.AddOnNLBHelloWorld.Enable)
	}
	if cfg.AddOnNLBHelloWorld.URL != "" { // env should be ignored for read-only
		t.Fatalf("unexpected cfg.AddOnNLBHelloWorld.URL %q", cfg.AddOnNLBHelloWorld.URL)
	}
	if !cfg.AddOnALB2048.Enable {
		t.Fatalf("unexpected cfg.AddOnALB2048.Enable %v", cfg.AddOnALB2048.Enable)
	}
	if cfg.AddOnALB2048.URL != "" { // env should be ignored for read-only
		t.Fatalf("unexpected cfg.AddOnALB2048.URL %q", cfg.AddOnALB2048.URL)
	}
	if cfg.AddOnALB2048.PolicyCFNStackID != "" { // env should be ignored for read-only
		t.Fatalf("unexpected cfg.AddOnALB2048.PolicyCFNStackID %q", cfg.AddOnALB2048.PolicyCFNStackID)
	}
	if cfg.AddOnALB2048.PolicyName != "my-policy" { // env should be ignored for read-only
		t.Fatalf("unexpected cfg.AddOnALB2048.PolicyName %q", cfg.AddOnALB2048.PolicyName)
	}
	if !cfg.AddOnJobPerl.Enable {
		t.Fatalf("unexpected cfg.AddOnJobPerl.Enable %v", cfg.AddOnJobPerl.Enable)
	}
	if cfg.AddOnJobPerl.Completes != 100 {
		t.Fatalf("unexpected cfg.AddOnJobPerl.Completes %v", cfg.AddOnJobPerl.Completes)
	}
	if cfg.AddOnJobPerl.Parallels != 10 {
		t.Fatalf("unexpected cfg.AddOnJobPerl.Parallels %v", cfg.AddOnJobPerl.Parallels)
	}
	if !cfg.AddOnJobEcho.Enable {
		t.Fatalf("unexpected cfg.AddOnJobEcho.Enable %v", cfg.AddOnJobEcho.Enable)
	}
	if cfg.AddOnJobEcho.Completes != 1000 {
		t.Fatalf("unexpected cfg.AddOnJobEcho.Completes %v", cfg.AddOnJobEcho.Completes)
	}
	if cfg.AddOnJobEcho.Parallels != 100 {
		t.Fatalf("unexpected cfg.AddOnJobEcho.Parallels %v", cfg.AddOnJobEcho.Parallels)
	}
	if cfg.AddOnJobEcho.Size != 10000 {
		t.Fatalf("unexpected cfg.AddOnJobEcho.Size %v", cfg.AddOnJobEcho.Size)
	}

	if err := cfg.ValidateAndSetDefaults(); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Sync(); err != nil {
		t.Fatal(err)
	}
	fmt.Println(cfg.Name)

	d, err := ioutil.ReadFile(cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(d))
	os.RemoveAll(cfg.ConfigPath)
}

func TestEnvManagedNodeGroup(t *testing.T) {
	cfg := NewDefault()

	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_MANAGED_NODE_GROUP_CREATE", "false")
	defer func() {
		os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_MANAGED_NODE_GROUP_CREATE")
	}()

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}

	if cfg.Parameters.ManagedNodeGroupCreate {
		t.Fatal("Parameters.ManagedNodeGroupCreate expected false, got true")
	}

	if err := cfg.ValidateAndSetDefaults(); !strings.Contains(err.Error(), "AddOnNLBHelloWorld.Enable true") {
		t.Fatalf("expected add-on error, got %v", err)
	}

	cfg.AddOnNLBHelloWorld.Enable = false
	cfg.AddOnALB2048.Enable = false
	cfg.AddOnJobPerl.Enable = false
	cfg.AddOnJobEcho.Enable = false

	if err := cfg.ValidateAndSetDefaults(); err != nil {
		t.Fatal(err)
	}
}
