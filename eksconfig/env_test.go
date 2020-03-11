package eksconfig

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestEnvAddOnManagedNodeGroups(t *testing.T) {
	cfg := NewDefault()
	defer func() {
		os.RemoveAll(cfg.ConfigPath)
		os.RemoveAll(cfg.KubectlCommandsOutputPath)
		os.RemoveAll(cfg.RemoteAccessCommandsOutputPath)
	}()

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
	defer func() {
		os.RemoveAll(cfg.ConfigPath)
		os.RemoveAll(cfg.KubectlCommandsOutputPath)
		os.RemoveAll(cfg.RemoteAccessCommandsOutputPath)
	}()

	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE", `true`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE")
	os.Setenv("AWS_K8S_TESTER_EKS_REMOTE_ACCESS_PRIVATE_KEY_PATH", `a`)
	defer os.Unsetenv("AWS_K8S_TESTER_EKS_REMOTE_ACCESS_PRIVATE_KEY_PATH")
	os.Setenv("AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_MNGS", `{"test-mng-for-cni":{"name":"test-mng-for-cni","tags":{"group":"amazon-vpc-cni-k8s"},"ami-type":"AL2_x86_64","asg-min-size":3,"asg-max-size":3,"asg-desired-capacity":3,"instance-types":["c5.xlarge"]}}`)
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
