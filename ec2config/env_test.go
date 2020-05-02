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
	os.Setenv("AWS_K8S_TESTER_EC2_DHCP_OPTIONS_DOMAIN_NAME", `hello.com`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_DHCP_OPTIONS_DOMAIN_NAME")
	os.Setenv("AWS_K8S_TESTER_EC2_DHCP_OPTIONS_DOMAIN_NAME_SERVERS", `1.2.3.0,4.5.6.7`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_DHCP_OPTIONS_DOMAIN_NAME_SERVERS")
	os.Setenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_KEY_NAME", `my-key`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_KEY_NAME")
	os.Setenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_PRIVATE_KEY_PATH", `a`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_PRIVATE_KEY_PATH")
	os.Setenv("AWS_K8S_TESTER_EC2_ASGS_FETCH_LOGS", "false")
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_ASGS_FETCH_LOGS")
	os.Setenv("AWS_K8S_TESTER_EC2_ASGS_LOGS_DIR", "hello")
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_ASGS_LOGS_DIR")
	os.Setenv("AWS_K8S_TESTER_EC2_ASGS", `{"test-asg":{"name":"test-asg","ssm-document-create":true,"ssm-document-cfn-stack-name":"my-doc-cfn-stack","ssm-document-name":"my-doc","ssm-document-command-id":"no","ssm-document-cfn-stack-id":"invalid","ssm-document-commands":"echo 123; echo 456;","ssm-document-execution-timeout-in-seconds":10,"remote-access-user-name":"my-user","image-id":"123","image-id-ssm-parameter":"777","asg-launch-configuration-cfn-stack-id":"none","asg-cfn-stack-id":"bbb","ami-type":"BOTTLEROCKET_x86_64","asg-min-size":30,"asg-max-size":30,"asg-desired-capacity":30,"volume-size":120,"instance-types":["c5.xlarge"]}}`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_ASGS")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
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
	if cfg.DHCPOptionsDomainName != "hello.com" {
		t.Fatalf("unexpected cfg.DHCPOptionsDomainName %q", cfg.DHCPOptionsDomainName)
	}
	if !reflect.DeepEqual(cfg.DHCPOptionsDomainNameServers, []string{"1.2.3.0", "4.5.6.7"}) {
		t.Fatalf("unexpected cfg.DHCPOptionsDomainNameServers %q", cfg.DHCPOptionsDomainNameServers)
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
			Name:                               "test-asg",
			RemoteAccessUserName:               "my-user",
			SSMDocumentCreate:                  true,
			SSMDocumentCFNStackName:            "my-doc-cfn-stack",
			SSMDocumentName:                    "my-doc",
			SSMDocumentCommands:                "echo 123; echo 456;",
			SSMDocumentExecutionTimeoutSeconds: 10,
			AMIType:                            "BOTTLEROCKET_x86_64",
			ImageID:                            "123",
			ImageIDSSMParameter:                "777",
			ASGMinSize:                         30,
			ASGMaxSize:                         30,
			ASGDesiredCapacity:                 30,
			InstanceTypes:                      []string{"c5.xlarge"},
			VolumeSize:                         120,
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
