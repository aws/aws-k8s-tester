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

	os.Setenv("AWS_K8S_TESTER_EC2_S3_BUCKET_NAME", `my-bucket`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_S3_BUCKET_NAME")
	os.Setenv("AWS_K8S_TESTER_EC2_S3_BUCKET_CREATE", `false`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_S3_BUCKET_CREATE")
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
	os.Setenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_KEY_CREATE", `false`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_KEY_CREATE")
	os.Setenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_KEY_NAME", `my-key`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_KEY_NAME")
	os.Setenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_PRIVATE_KEY_PATH", `a`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_PRIVATE_KEY_PATH")
	os.Setenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_USER_NAME", `my-user`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_REMOTE_ACCESS_USER_NAME")
	os.Setenv("AWS_K8S_TESTER_EC2_ASGS_FETCH_LOGS", "false")
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_ASGS_FETCH_LOGS")
	os.Setenv("AWS_K8S_TESTER_EC2_ASGS_LOGS_DIR", "hello")
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_ASGS_LOGS_DIR")
	os.Setenv("AWS_K8S_TESTER_EC2_ASGS", `{"test-asg":{"name":"test-asg","ssm-document-create":true,"ssm-document-name":"my-doc","ssm-document-command-id":"no","ssm-document-cfn-stack-id":"invalid","ssm-document-commands":"echo 123; echo 456;","ssm-document-execution-timeout-in-seconds":10,"launch-configuration-name":"aaa","image-id":"123","image-id-ssm-parameter":"777","asg-launch-configuration-cfn-stack-id":"none","asg-cfn-stack-id":"bbb","install-ssm":false,"asg-min-size":30,"asg-max-size":30,"asg-desired-capacity":30,"volume-size":120,"instance-type":"c5.xlarge"}}`)
	defer os.Unsetenv("AWS_K8S_TESTER_EC2_ASGS")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}

	if cfg.S3BucketName != "my-bucket" {
		t.Fatalf("unexpected cfg.S3BucketName %q", cfg.S3BucketName)
	}
	if cfg.S3BucketCreate {
		t.Fatalf("unexpected cfg.S3BucketCreate %v", cfg.S3BucketCreate)
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

	if cfg.ASGsFetchLogs {
		t.Fatalf("unexpected cfg.ASGsFetchLogs %v", cfg.ASGsFetchLogs)
	}
	if cfg.ASGsLogsDir != "hello" {
		t.Fatalf("unexpected cfg.ASGsLogsDir %q", cfg.ASGsLogsDir)
	}
	expectedASGs := map[string]ASG{
		"test-asg": {
			Name:                               "test-asg",
			LaunchConfigurationName:            "aaa",
			InstallSSM:                         false,
			SSMDocumentCreate:                  true,
			SSMDocumentName:                    "my-doc",
			SSMDocumentCommands:                "echo 123; echo 456;",
			SSMDocumentExecutionTimeoutSeconds: 10,
			ImageID:                            "123",
			ImageIDSSMParameter:                "777",
			ASGMinSize:                         30,
			ASGMaxSize:                         30,
			ASGDesiredCapacity:                 30,
			InstanceType:                       "c5.xlarge",
			VolumeSize:                         120,
		},
	}
	if !reflect.DeepEqual(cfg.ASGs, expectedASGs) {
		t.Fatalf("expected cfg.ASGs\n%+v\n\ngot\n%+v", expectedASGs, cfg.ASGs)
	}

	if err := cfg.ValidateAndSetDefaults(); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
}
