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
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_REGION")

	os.Setenv("AWS_K8S_TESTER_EKS_LOG_LEVEL", "debug")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_LOG_LEVEL")
	os.Setenv("AWS_K8S_TESTER_EKS_KUBECTL_DOWNLOAD_URL", kubectlDownloadURL)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_KUBECTL_DOWNLOAD_URL")
	os.Setenv("AWS_K8S_TESTER_EKS_KUBECTL_PATH", "/tmp/aws-k8s-tester-test/kubectl")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_KUBECTL_PATH")
	os.Setenv("AWS_K8S_TESTER_EKS_KUBECONFIG_PATH", "/tmp/aws-k8s-tester/kubeconfig2")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_KUBECONFIG_PATH")
	os.Setenv("AWS_K8S_TESTER_EKS_ON_FAILURE_DELETE", "false")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ON_FAILURE_DELETE")
	os.Setenv("AWS_K8S_TESTER_EKS_ON_FAILURE_DELETE_WAIT_SECONDS", "780")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ON_FAILURE_DELETE_WAIT_SECONDS")

	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_VERSION", "1.11")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_VERSION")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_CLUSTER_TAGS", "to-delete=2019;hello-world=test")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_CLUSTER_TAGS")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_CLUSTER_REQUEST_HEADER_KEY", "eks-options")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_CLUSTER_REQUEST_HEADER_KEY")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_CLUSTER_REQUEST_HEADER_VALUE", "kubernetesVersion=1.11")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_CLUSTER_REQUEST_HEADER_VALUE")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_CLUSTER_ROLE_SERVICE_PRINCIPALS", "eks.amazonaws.com,eks-beta-pdx.aws.internal,eks-dev.aws.internal")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_CLUSTER_ROLE_SERVICE_PRINCIPALS")
	os.Setenv("AWS_K8S_TESTER_EKS_PARAMETERS_CLUSTER_ROLE_MANAGED_POLICY_ARNS", "arn:aws:iam::aws:policy/AmazonEKSServicePolicy,arn:aws:iam::aws:policy/AmazonEKSClusterPolicy,arn:aws:iam::aws:policy/service-role/AmazonEC2RoleforSSM")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_PARAMETERS_CLUSTER_ROLE_MANAGED_POLICY_ARNS")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_CREATED", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_CREATED")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ROLE_NAME", "mng-role-name")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ROLE_NAME")
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
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_SSH_KEY_PAIR_NAME", "a")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_SSH_KEY_PAIR_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_REMOTE_ACCESS_PRIVATE_KEY_PATH", "a")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_REMOTE_ACCESS_PRIVATE_KEY_PATH")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_REMOTE_ACCESS_USER_NAME", "a")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_REMOTE_ACCESS_USER_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_MNGS", `{"mng-test-name-cpu":{"name":"mng-test-name-cpu","tags":{"cpu":"hello-world"},"release-version":"test-ver-cpu","ami-type":"AL2_x86_64","asg-min-size":17,"asg-max-size":99,"asg-desired-capacity":77,"instance-types":["type-cpu-1","type-cpu-2"],"volume-size":40},"mng-test-name-gpu":{"name":"mng-test-name-gpu","tags":{"gpu":"hello-world"},"release-version":"test-ver-gpu","ami-type":"AL2_x86_64_GPU","asg-min-size":30,"asg-max-size":35,"asg-desired-capacity":34,"instance-types":["type-gpu-1","type-gpu-2"],"volume-size":500}}`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_MNGS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_LOG_DIR", "a")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_LOG_DIR")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_CREATED", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_CREATED")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_NLB_ARN", "invalid")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_NLB_ARN")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_NLB_NAME", "invalid")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_NLB_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_URL", "invalid")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_URL")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_NAMESPACE", "test-namespace")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_NAMESPACE")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_CREATED", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_CREATED")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_ALB_ARN", "invalid")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_ALB_ARN")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_ALB_NAME", "invalid")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_ALB_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_URL", "invalid")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_URL")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_POLICY_NAME", "my-policy")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_POLICY_NAME")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_POLICY_CFN_STACK_ID", "my-id")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_POLICY_CFN_STACK_ID")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_NAMESPACE", "test-namespace")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_NAMESPACE")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_PERL_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_PERL_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_PERL_CREATED", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_PERL_CREATED")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_PERL_NAMESPACE", "hello1")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_PERL_NAMESPACE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_PERL_COMPLETES", "100")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_PERL_COMPLETES")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_PERL_PARALLELS", "10")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_PERL_PARALLELS")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_ECHO_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_ECHO_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_ECHO_CREATED", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_ECHO_CREATED")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_ECHO_NAMESPACE", "hello2")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_ECHO_NAMESPACE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_ECHO_COMPLETES", "1000")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_ECHO_COMPLETES")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_ECHO_PARALLELS", "100")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_ECHO_PARALLELS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_ECHO_SIZE", "10000")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_JOB_ECHO_SIZE")

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_ENABLE", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_CREATED", "true")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_CREATED")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_NAMESPACE", "hello")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_NAMESPACE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_OBJECTS", "5")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_OBJECTS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_SIZE", "10")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_SIZE")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_SECRET_QPS", "10")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_SECRET_QPS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_SECRET_BURST", "10")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_SECRET_BURST")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_POD_QPS", "10")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_POD_QPS")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_POD_BURST", "10")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_POD_BURST")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_WRITES_RESULT_PATH", "writes.csv")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_WRITES_RESULT_PATH")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_READS_RESULT_PATH", "reads.csv")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_READS_RESULT_PATH")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}

	if cfg.Region != "us-east-1" {
		t.Fatalf("unexpected %q", cfg.Region)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("unexpected %q", cfg.LogLevel)
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
	if cfg.OnFailureDelete {
		t.Fatalf("unexpected OnFailureDelete %v", cfg.OnFailureDelete)
	}
	if cfg.OnFailureDeleteWaitSeconds != 780 {
		t.Fatalf("unexpected OnFailureDeleteWaitSeconds %d", cfg.OnFailureDeleteWaitSeconds)
	}

	if cfg.Parameters.Version != "1.11" {
		t.Fatalf("unexpected Parameters.Version %q", cfg.Parameters.Version)
	}
	expectedClusterTags := map[string]string{"to-delete": "2019", "hello-world": "test"}
	if !reflect.DeepEqual(cfg.Parameters.ClusterTags, expectedClusterTags) {
		t.Fatalf("Tags expected %v, got %v", expectedClusterTags, cfg.Parameters.ClusterTags)
	}
	if cfg.Parameters.ClusterRequestHeaderKey != "eks-options" {
		t.Fatalf("unexpected Parameters.ClusterRequestHeaderKey %q", cfg.Parameters.ClusterRequestHeaderKey)
	}
	if cfg.Parameters.ClusterRequestHeaderValue != "kubernetesVersion=1.11" {
		t.Fatalf("unexpected Parameters.ClusterRequestHeaderValue %q", cfg.Parameters.ClusterRequestHeaderValue)
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

	if cfg.AddOnManagedNodeGroups.Created { // read-only must be ignored
		t.Fatalf("unexpected AddOnManagedNodeGroups.Created %v", cfg.AddOnManagedNodeGroups.Created)
	}
	if cfg.AddOnManagedNodeGroups.RoleName != "mng-role-name" {
		t.Fatalf("unexpected AddOnManagedNodeGroups.RoleName %q", cfg.AddOnManagedNodeGroups.RoleName)
	}
	expectedManagedNodeGroupRoleServicePrincipals := []string{
		"ec2.amazonaws.com",
		"eks.amazonaws.com",
		"hello.amazonaws.com",
	}
	if !reflect.DeepEqual(expectedManagedNodeGroupRoleServicePrincipals, cfg.AddOnManagedNodeGroups.RoleServicePrincipals) {
		t.Fatalf("unexpected AddOnManagedNodeGroups.RoleServicePrincipals %+v", cfg.AddOnManagedNodeGroups.RoleServicePrincipals)
	}
	expectedManagedNodeGroupRoleManagedPolicyARNs := []string{
		"a",
		"b",
		"c",
	}
	if !reflect.DeepEqual(expectedManagedNodeGroupRoleManagedPolicyARNs, cfg.AddOnManagedNodeGroups.RoleManagedPolicyARNs) {
		t.Fatalf("unexpected AddOnManagedNodeGroups.RoleManagedPolicyARNs %+v", cfg.AddOnManagedNodeGroups.RoleManagedPolicyARNs)
	}
	if cfg.AddOnManagedNodeGroups.RequestHeaderKey != "a" {
		t.Fatalf("unexpected AddOnManagedNodeGroups.RequestHeaderKey %q", cfg.AddOnManagedNodeGroups.RequestHeaderKey)
	}
	if cfg.AddOnManagedNodeGroups.RequestHeaderValue != "b" {
		t.Fatalf("unexpected AddOnManagedNodeGroups.RequestHeaderValue %q", cfg.AddOnManagedNodeGroups.RequestHeaderValue)
	}
	if cfg.AddOnManagedNodeGroups.ResolverURL != "a" {
		t.Fatalf("unexpected AddOnManagedNodeGroups.ResolverURL %q", cfg.AddOnManagedNodeGroups.ResolverURL)
	}
	if cfg.AddOnManagedNodeGroups.SigningName != "a" {
		t.Fatalf("unexpected AddOnManagedNodeGroups.SigningName %q", cfg.AddOnManagedNodeGroups.SigningName)
	}
	if cfg.AddOnManagedNodeGroups.SSHKeyPairName != "a" {
		t.Fatalf("unexpected AddOnManagedNodeGroups.SSHKeyPairName %q", cfg.AddOnManagedNodeGroups.SSHKeyPairName)
	}
	if cfg.AddOnManagedNodeGroups.RemoteAccessPrivateKeyPath != "a" {
		t.Fatalf("unexpected AddOnManagedNodeGroups.RemoteAccessPrivateKeyPath %q", cfg.AddOnManagedNodeGroups.RemoteAccessPrivateKeyPath)
	}
	if cfg.AddOnManagedNodeGroups.RemoteAccessUserName != "a" {
		t.Fatalf("unexpected AddOnManagedNodeGroups.RemoteAccessUserName %q", cfg.AddOnManagedNodeGroups.RemoteAccessUserName)
	}
	cpuName, gpuName := "mng-test-name-cpu", "mng-test-name-gpu"
	expectedMNGs := map[string]MNG{
		cpuName: MNG{
			Name:               cpuName,
			Tags:               map[string]string{"cpu": "hello-world"},
			ReleaseVersion:     "test-ver-cpu",
			AMIType:            "AL2_x86_64",
			ASGMinSize:         17,
			ASGMaxSize:         99,
			ASGDesiredCapacity: 77,
			InstanceTypes:      []string{"type-cpu-1", "type-cpu-2"},
			VolumeSize:         40,
		},
		gpuName: MNG{
			Name:               gpuName,
			Tags:               map[string]string{"gpu": "hello-world"},
			ReleaseVersion:     "test-ver-gpu",
			AMIType:            "AL2_x86_64_GPU",
			ASGMinSize:         30,
			ASGMaxSize:         35,
			ASGDesiredCapacity: 34,
			InstanceTypes:      []string{"type-gpu-1", "type-gpu-2"},
			VolumeSize:         500,
		},
	}
	if !reflect.DeepEqual(cfg.AddOnManagedNodeGroups.MNGs, expectedMNGs) {
		t.Fatalf("expected AddOnManagedNodeGroups.MNGs %+v, got %+v", expectedMNGs, cfg.AddOnManagedNodeGroups.MNGs)
	}
	if cfg.AddOnManagedNodeGroups.LogDir != "a" {
		t.Fatalf("unexpected cfg.AddOnManagedNodeGroups.LogDir %q", cfg.AddOnManagedNodeGroups.LogDir)
	}

	if cfg.AddOnNLBHelloWorld.Created { // read-only must be ignored
		t.Fatalf("unexpected AddOnNLBHelloWorld.Created %v", cfg.AddOnNLBHelloWorld.Created)
	}
	if !cfg.AddOnNLBHelloWorld.Enable {
		t.Fatalf("unexpected cfg.AddOnNLBHelloWorld.Enable %v", cfg.AddOnNLBHelloWorld.Enable)
	}
	if cfg.AddOnNLBHelloWorld.NLBARN != "" { // env should be ignored for read-only
		t.Fatalf("unexpected cfg.AddOnNLBHelloWorld.NLBARN %q", cfg.AddOnNLBHelloWorld.NLBARN)
	}
	if cfg.AddOnNLBHelloWorld.NLBName != "" { // env should be ignored for read-only
		t.Fatalf("unexpected cfg.AddOnNLBHelloWorld.NLBName %q", cfg.AddOnNLBHelloWorld.NLBName)
	}
	if cfg.AddOnNLBHelloWorld.URL != "" { // env should be ignored for read-only
		t.Fatalf("unexpected cfg.AddOnNLBHelloWorld.URL %q", cfg.AddOnNLBHelloWorld.URL)
	}
	if cfg.AddOnNLBHelloWorld.Namespace != "test-namespace" {
		t.Fatalf("unexpected cfg.AddOnNLBHelloWorld.Namespace %q", cfg.AddOnNLBHelloWorld.Namespace)
	}

	if cfg.AddOnALB2048.Created { // read-only must be ignored
		t.Fatalf("unexpected AddOnALB2048.Created %v", cfg.AddOnALB2048.Created)
	}
	if !cfg.AddOnALB2048.Enable {
		t.Fatalf("unexpected cfg.AddOnALB2048.Enable %v", cfg.AddOnALB2048.Enable)
	}
	if cfg.AddOnALB2048.ALBARN != "" { // env should be ignored for read-only
		t.Fatalf("unexpected cfg.AddOnALB2048.ALBARN %q", cfg.AddOnALB2048.ALBARN)
	}
	if cfg.AddOnALB2048.ALBName != "" { // env should be ignored for read-only
		t.Fatalf("unexpected cfg.AddOnALB2048.ALBName %q", cfg.AddOnALB2048.ALBName)
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
	if cfg.AddOnALB2048.Namespace != "test-namespace" {
		t.Fatalf("unexpected cfg.AddOnALB2048.Namespace %q", cfg.AddOnALB2048.Namespace)
	}

	if cfg.AddOnJobPerl.Created { // read-only must be ignored
		t.Fatalf("unexpected AddOnJobPerl.Created %v", cfg.AddOnJobPerl.Created)
	}
	if !cfg.AddOnJobPerl.Enable {
		t.Fatalf("unexpected cfg.AddOnJobPerl.Enable %v", cfg.AddOnJobPerl.Enable)
	}
	if cfg.AddOnJobPerl.Namespace != "hello1" {
		t.Fatalf("unexpected cfg.AddOnJobPerl.Namespace %q", cfg.AddOnJobPerl.Namespace)
	}
	if cfg.AddOnJobPerl.Completes != 100 {
		t.Fatalf("unexpected cfg.AddOnJobPerl.Completes %v", cfg.AddOnJobPerl.Completes)
	}
	if cfg.AddOnJobPerl.Parallels != 10 {
		t.Fatalf("unexpected cfg.AddOnJobPerl.Parallels %v", cfg.AddOnJobPerl.Parallels)
	}

	if cfg.AddOnJobEcho.Created { // read-only must be ignored
		t.Fatalf("unexpected AddOnJobEcho.Created %v", cfg.AddOnJobEcho.Created)
	}
	if !cfg.AddOnJobEcho.Enable {
		t.Fatalf("unexpected cfg.AddOnJobEcho.Enable %v", cfg.AddOnJobEcho.Enable)
	}
	if cfg.AddOnJobEcho.Namespace != "hello2" {
		t.Fatalf("unexpected cfg.AddOnJobEcho.Namespace %q", cfg.AddOnJobEcho.Namespace)
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

	if cfg.AddOnSecrets.Created { // read-only must be ignored
		t.Fatalf("unexpected AddOnSecrets.Created %v", cfg.AddOnSecrets.Created)
	}
	if !cfg.AddOnSecrets.Enable {
		t.Fatalf("unexpected cfg.AddOnSecrets.Enable %v", cfg.AddOnSecrets.Enable)
	}
	if cfg.AddOnSecrets.Namespace != "hello" {
		t.Fatalf("unexpected cfg.AddOnSecrets.Namespace %q", cfg.AddOnSecrets.Namespace)
	}
	if cfg.AddOnSecrets.Objects != 5 {
		t.Fatalf("unexpected cfg.AddOnSecrets.Objects %v", cfg.AddOnSecrets.Objects)
	}
	if cfg.AddOnSecrets.Size != 10 {
		t.Fatalf("unexpected cfg.AddOnSecrets.Size %v", cfg.AddOnSecrets.Size)
	}
	if cfg.AddOnSecrets.SecretQPS != 10 {
		t.Fatalf("unexpected cfg.AddOnSecrets.SecretQPS %v", cfg.AddOnSecrets.SecretQPS)
	}
	if cfg.AddOnSecrets.SecretBurst != 10 {
		t.Fatalf("unexpected cfg.AddOnSecrets.SecretBurst %v", cfg.AddOnSecrets.SecretBurst)
	}
	if cfg.AddOnSecrets.PodQPS != 10 {
		t.Fatalf("unexpected cfg.AddOnSecrets.PodQPS %v", cfg.AddOnSecrets.PodQPS)
	}
	if cfg.AddOnSecrets.PodBurst != 10 {
		t.Fatalf("unexpected cfg.AddOnSecrets.PodBurst %v", cfg.AddOnSecrets.PodBurst)
	}
	if cfg.AddOnSecrets.WritesResultPath != "writes.csv" {
		t.Fatalf("unexpected cfg.AddOnSecrets.WritesResultPath %q", cfg.AddOnSecrets.WritesResultPath)
	}
	if cfg.AddOnSecrets.ReadsResultPath != "reads.csv" {
		t.Fatalf("unexpected cfg.AddOnSecrets.ReadsResultPath %q", cfg.AddOnSecrets.ReadsResultPath)
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

func TestEnvAddOnManagedNodeGroups(t *testing.T) {
	cfg := NewDefault()

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE", "false")
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}

	if cfg.AddOnManagedNodeGroups.Enable {
		t.Fatal("AddOnManagedNodeGroups.Enable expected false, got true")
	}

	cfg.AddOnNLBHelloWorld.Enable = true
	if err := cfg.ValidateAndSetDefaults(); !strings.Contains(err.Error(), "AddOnNLBHelloWorld.Enable true") {
		t.Fatalf("expected add-on error, got %v", err)
	}
}

// TestEnvAddOnManagedNodeGroupsCNI tests CNI integration test MNG settings.
// https://github.com/aws/amazon-vpc-cni-k8s/blob/master/scripts/lib/cluster.sh
func TestEnvAddOnManagedNodeGroupsCNI(t *testing.T) {
	cfg := NewDefault()

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_REMOTE_ACCESS_PRIVATE_KEY_PATH", `a`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_REMOTE_ACCESS_PRIVATE_KEY_PATH")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_MNGS", `{"test-mng-for-cni":{"name":"test-mng-for-cni","tags":{"group":"amazon-vpc-cni-k8s"},"ami-type":"AL2_x86_64","asg-min-size":3,"asg-max-size":3,"asg-desired-capacity":3,"instance-types":["c5.xlarge"]}}`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_MNGS")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		t.Fatal(err)
	}

	if cfg.AddOnManagedNodeGroups.RemoteAccessPrivateKeyPath != "a" {
		t.Fatalf("unexpected AddOnManagedNodeGroups.RemoteAccessPrivateKeyPath %q", cfg.AddOnManagedNodeGroups.RemoteAccessPrivateKeyPath)
	}
	expectedMNGs := map[string]MNG{
		"test-mng-for-cni": MNG{
			Name:               "test-mng-for-cni",
			Tags:               map[string]string{"group": "amazon-vpc-cni-k8s"},
			AMIType:            "AL2_x86_64",
			ASGMinSize:         3,
			ASGMaxSize:         3,
			ASGDesiredCapacity: 3,
			InstanceTypes:      []string{"c5.xlarge"},
			VolumeSize:         40,
		},
	}
	if !reflect.DeepEqual(cfg.AddOnManagedNodeGroups.MNGs, expectedMNGs) {
		t.Fatalf("expected AddOnManagedNodeGroups.MNGs %+v, got %+v", expectedMNGs, cfg.AddOnManagedNodeGroups.MNGs)
	}
}

// TestEnvAddOnManagedNodeGroupsInvalidInstanceType tests invalid instance types.
func TestEnvAddOnManagedNodeGroupsInvalidInstanceType(t *testing.T) {
	cfg := NewDefault()

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_REMOTE_ACCESS_PRIVATE_KEY_PATH", `a`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_REMOTE_ACCESS_PRIVATE_KEY_PATH")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_MNGS", `{"test-mng-for-cni":{"name":"test-mng-for-cni","tags":{"group":"amazon-vpc-cni-k8s"},"ami-type":"AL2_x86_64","asg-min-size":3,"asg-max-size":3,"asg-desired-capacity":3,"instance-types":["m3.xlarge"]}}`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_MNGS")

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
