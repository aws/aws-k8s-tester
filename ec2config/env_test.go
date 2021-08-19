package ec2config

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestEnv(t *testing.T) {
	cfg := NewDefault()
	defer func() {
		os.RemoveAll(cfg.ConfigPath)
		os.RemoveAll(cfg.RemoteAccessCommandsOutputPath)
	}()

	os.Setenv("AWS_K8S_TESTER_EC2_LOG_COLOR", `false`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_LOG_COLOR")
	os.Setenv("AWS_K8S_TESTER_EC2_LOG_COLOR_OVERRIDE", `true`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_LOG_COLOR_OVERRIDE")
	os.Setenv("AWS_K8S_TESTER_EC2_S3_BUCKET_CREATE", `true`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_S3_BUCKET_CREATE")
	os.Setenv("AWS_K8S_TESTER_EC2_S3_BUCKET_CREATE_KEEP", `true`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_S3_BUCKET_CREATE_KEEP")
	os.Setenv("AWS_K8S_TESTER_EC2_S3_BUCKET_NAME", `my-bucket`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_S3_BUCKET_NAME")
	os.Setenv("AWS_K8S_TESTER_EC2_S3_BUCKET_LIFECYCLE_EXPIRATION_DAYS", `10`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_S3_BUCKET_LIFECYCLE_EXPIRATION_DAYS")
	os.Setenv("AWS_K8S_TESTER_EC2_ROLE_CREATE", `false`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_ROLE_CREATE")
	os.Setenv("AWS_K8S_TESTER_EC2_ROLE_ARN", `role-arn`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_ROLE_ARN")
	os.Setenv("AWS_K8S_TESTER_EC2_VPC_CREATE", `false`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_VPC_CREATE")
	os.Setenv("AWS_K8S_TESTER_EC2_VPC_ID", `vpc-id`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_VPC_ID")
	os.Setenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_KEY_CREATE", `true`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_KEY_CREATE")
	os.Setenv("AWS_K8S_TESTER_EC2_VPC_DHCP_OPTIONS_DOMAIN_NAME", `hello.com`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_VPC_DHCP_OPTIONS_DOMAIN_NAME")
	os.Setenv("AWS_K8S_TESTER_EC2_VPC_DHCP_OPTIONS_DOMAIN_NAME_SERVERS", `1.2.3.0,4.5.6.7`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_VPC_DHCP_OPTIONS_DOMAIN_NAME_SERVERS")
	os.Setenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_KEY_NAME", `my-key`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_KEY_NAME")
	os.Setenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_PRIVATE_KEY_PATH", `a`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_PRIVATE_KEY_PATH")
	os.Setenv("AWS_K8S_TESTER_EC2_ASGS_FETCH_LOGS", "false")
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_ASGS_FETCH_LOGS")
	os.Setenv("AWS_K8S_TESTER_EC2_ASGS_LOGS_DIR", "hello")
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_ASGS_LOGS_DIR")
	os.Setenv("AWS_K8S_TESTER_EC2_ASGS", `{"test-asg":{"name":"test-asg","ssm":{"document-create":true,"document-name":"my-doc","document-commands":"echo 123; echo 456;","document-execution-timeout-in-seconds":10},"remote-access-user-name":"my-user","image-id":"123","image-id-ssm-parameter":"777","asg-launch-configuration-cfn-stack-id":"none","asg-cfn-stack-id":"bbb","ami-type":"BOTTLEROCKET_x86_64","asg-min-size":30,"asg-max-size":30,"asg-desired-capacity":30,"volume-size":120,"volume-type":"io1","instance-type":"c5.xlarge"}}`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_ASGS")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}

	if cfg.LogColor {
		t.Fatalf("unexpected cfg.LogColor %v", cfg.LogColor)
	}
	if cfg.LogColorOverride != "true" {
		t.Fatalf("unexpected LogColorOverride %q", cfg.LogColorOverride)
	}
	if !cfg.S3.BucketCreate {
		t.Fatalf("unexpected cfg.S3.BucketCreate %v", cfg.S3.BucketCreate)
	}
	if !cfg.S3.BucketCreateKeep {
		t.Fatalf("unexpected cfg.S3.BucketCreateKeep %v", cfg.S3.BucketCreateKeep)
	}
	if cfg.S3.BucketName != "my-bucket" {
		t.Fatalf("unexpected cfg.S3.BucketName %q", cfg.S3.BucketName)
	}
	if cfg.S3.BucketLifecycleExpirationDays != 10 {
		t.Fatalf("unexpected cfg.S3.BucketLifecycleExpirationDays %d", cfg.S3.BucketLifecycleExpirationDays)
	}

	if cfg.Role.Create {
		t.Fatalf("unexpected cfg.Role.Create %v", cfg.Role.Create)
	}
	if cfg.Role.ARN != "role-arn" {
		t.Fatalf("unexpected cfg.Role.ARN %q", cfg.Role.ARN)
	}
	if cfg.VPC.Create {
		t.Fatalf("unexpected cfg.VPC.Create %v", cfg.VPC.Create)
	}
	if cfg.VPC.ID != "vpc-id" {
		t.Fatalf("unexpected cfg.VPC.ID %q", cfg.VPC.ID)
	}
	if cfg.VPC.DHCPOptionsDomainName != "hello.com" {
		t.Fatalf("unexpected cfg.VPC.DHCPOptionsDomainName %q", cfg.VPC.DHCPOptionsDomainName)
	}
	if !reflect.DeepEqual(cfg.VPC.DHCPOptionsDomainNameServers, []string{"1.2.3.0", "4.5.6.7"}) {
		t.Fatalf("unexpected cfg.VPC.DHCPOptionsDomainNameServers %q", cfg.VPC.DHCPOptionsDomainNameServers)
	}

	if !cfg.RemoteAccessKeyCreate {
		t.Fatalf("unexpected cfg.RemoteAccessKeyCreate %v", cfg.RemoteAccessKeyCreate)
	}
	if cfg.RemoteAccessKeyName != "my-key" {
		t.Fatalf("unexpected cfg.RemoteAccessKeyName %q", cfg.RemoteAccessKeyName)
	}
	if cfg.RemoteAccessPrivateKeyPath != "a" {
		t.Fatalf("unexpected cfg.RemoteAccessPrivateKeyPath %q", cfg.RemoteAccessPrivateKeyPath)
	}

	if cfg.ASGsFetchLogs {
		t.Fatalf("unexpected cfg.ASGsFetchLogs %v", cfg.ASGsFetchLogs)
	}
	if cfg.ASGsLogsDir != "hello" {
		t.Fatalf("unexpected cfg.ASGsLogsDir %q", cfg.ASGsLogsDir)
	}
	expectedASGs := map[string]ASG{
		"test-asg": {
			Name:                 "test-asg",
			RemoteAccessUserName: "my-user",
			SSM: &SSM{
				DocumentCreate:                  true,
				DocumentName:                    "my-doc",
				DocumentCommands:                "echo 123; echo 456;",
				DocumentExecutionTimeoutSeconds: 10,
			},
			AMIType:             "BOTTLEROCKET_x86_64",
			ImageID:             "123",
			ImageIDSSMParameter: "777",
			ASGMinSize:          30,
			ASGMaxSize:          30,
			ASGDesiredCapacity:  30,
			InstanceType:        "c5.xlarge",
			VolumeSize:          120,
			VolumeType:          "io1",
		},
	}
	if !reflect.DeepEqual(cfg.ASGs, expectedASGs) {
		t.Fatalf("expected cfg.ASGs\n%+v\n\ngot\n%+v", expectedASGs, cfg.ASGs)
	}

	err := cfg.ValidateAndSetDefaults()
	if err == nil {
		t.Fatalf("expected error but got %v", err)
	}
	if !strings.Contains(err.Error(), "unexpected RemoteAccessUserName") {
		t.Fatalf("unexpected error %v", err)
	}
}
