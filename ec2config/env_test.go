package ec2config

import (
	"os"
	"reflect"
	"testing"
)

func TestEnv(t *testing.T) {
	cfg := NewDefault()
	defer func() {
		os.RemoveAll(cfg.ConfigPath)
		os.RemoveAll(cfg.RemoteAccessCommandsOutputPath)
	}()

	os.Setenv("AWS_K8S_TESTER_EC2_ROLE_CREATE", `false`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_ROLE_CREATE")
	os.Setenv("AWS_K8S_TESTER_EC2_ROLE_ARN", `role-arn`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_ROLE_ARN")
	os.Setenv("AWS_K8S_TESTER_EC2_VPC_CREATE", `false`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_VPC_CREATE")
	os.Setenv("AWS_K8S_TESTER_EC2_VPC_ID", `vpc-id`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_VPC_ID")
	os.Setenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_KEY_CREATE", `false`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_KEY_CREATE")
	os.Setenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_KEY_NAME", `my-key`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_KEY_NAME")
	os.Setenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_PRIVATE_KEY_PATH", `a`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_PRIVATE_KEY_PATH")
	os.Setenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_USER_NAME", `my-user`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_USER_NAME")
	os.Setenv("AWS_K8S_TESTER_EC2_ASGS", `{"test-asg":{"name":"test-asg","image-id":"123","min-size":30,"max-size":30,"desired-capacity":30,"volume-size":120,"instance-type":"c5.xlarge"}}`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_ASGS")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}

	if cfg.RoleCreate {
		t.Fatalf("unexpected cfg.RoleCreate %v", cfg.RoleCreate)
	}
	if cfg.RoleARN != "role-arn" {
		t.Fatalf("unexpected cfg.RoleARN %q", cfg.RoleARN)
	}
	if cfg.VPCCreate {
		t.Fatalf("unexpected cfg.VPCCreate %v", cfg.VPCCreate)
	}
	if cfg.VPCID != "vpc-id" {
		t.Fatalf("unexpected cfg.VPCID %q", cfg.VPCID)
	}
	if cfg.RemoteAccessKeyCreate {
		t.Fatalf("unexpected cfg.RemoteAccessKeyCreate %v", cfg.RemoteAccessKeyCreate)
	}
	if cfg.RemoteAccessKeyName != "my-key" {
		t.Fatalf("unexpected cfg.RemoteAccessKeyName %q", cfg.RemoteAccessKeyName)
	}
	if cfg.RemoteAccessPrivateKeyPath != "a" {
		t.Fatalf("unexpected cfg.RemoteAccessPrivateKeyPath %q", cfg.RemoteAccessPrivateKeyPath)
	}
	if cfg.RemoteAccessUserName != "my-user" {
		t.Fatalf("unexpected cfg.RemoteAccessUserName %q", cfg.RemoteAccessUserName)
	}

	expectedASGs := map[string]ASG{
		"test-asg": {
			Name:            "test-asg",
			ImageID:         "123",
			MinSize:         30,
			MaxSize:         30,
			DesiredCapacity: 30,
			InstanceType:    "c5.xlarge",
			VolumeSize:      120,
		},
	}
	if !reflect.DeepEqual(cfg.ASGs, expectedASGs) {
		t.Fatalf("expected cfg.ASGs\n%+v\n\ngot\n%+v", expectedASGs, cfg.ASGs)
	}

	if err := cfg.ValidateAndSetDefaults(); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
}
