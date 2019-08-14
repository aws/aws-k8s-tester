package ec2config

import (
	"os"
	"reflect"
	"testing"
	"time"
)

func TestEnv(t *testing.T) {
	cfg := NewDefault()

	os.Setenv("AWS_K8S_TESTER_EC2_WAIT_BEFORE_DOWN", "2h")
	os.Setenv("AWS_K8S_TESTER_EC2_CLUSTER_SIZE", "100")
	os.Setenv("AWS_K8S_TESTER_EC2_AWS_REGION", "us-east-1")
	os.Setenv("AWS_K8S_TESTER_EC2_CONFIG_PATH", "test-path")
	os.Setenv("AWS_K8S_TESTER_EC2_DOWN", "false")
	os.Setenv("AWS_K8S_TESTER_EC2_LOG_LEVEL", "debug")
	os.Setenv("AWS_K8S_TESTER_EC2_UPLOAD_AWS_TESTER_LOGS", "false")
	os.Setenv("AWS_K8S_TESTER_EC2_UPLOAD_BUCKET_EXPIRE_DAYS", "3")
	os.Setenv("AWS_K8S_TESTER_EC2_VPC_ID", "aaa")
	os.Setenv("AWS_K8S_TESTER_EC2_PLUGINS", "update-amazon-linux-2,install-go-1.11.3")
	os.Setenv("AWS_K8S_TESTER_EC2_INSTANCE_TYPE", "m5d.2xlarge")
	os.Setenv("AWS_K8S_TESTER_EC2_KEY_NAME", "test-key")
	os.Setenv("AWS_K8S_TESTER_EC2_KEY_PATH", "/root/.ssh/kube_aws_rsa")
	os.Setenv("AWS_K8S_TESTER_EC2_KEY_CREATED", "true")
	os.Setenv("AWS_K8S_TESTER_EC2_ASSOCIATE_PUBLIC_IP_ADDRESS", "false")
	os.Setenv("AWS_K8S_TESTER_EC2_SUBNET_IDS", "a,b,c")
	os.Setenv("AWS_K8S_TESTER_EC2_SECURITY_GROUP_IDS", "d,e,f")
	os.Setenv("AWS_K8S_TESTER_EC2_WAIT", "true")
	os.Setenv("AWS_K8S_TESTER_EC2_TAGS", "kubernetes.io/cluster/a8-ec2-190222-9dxccww=owned")
	os.Setenv("AWS_K8S_TESTER_EC2_INGRESS_RULES_TCP", "22=0.0.0.0/0,2379-2380=192.168.0.0/8")
	os.Setenv("AWS_K8S_TESTER_EC2_VOLUME_SIZE", "120")
	os.Setenv("AWS_K8S_TESTER_EC2_VPC_CIDR", "192.168.0.0/8")
	os.Setenv("AWS_K8S_TESTER_EC2_INSTANCE_PROFILE_FILE_PATH", "/tmp/aws-k8s-tester-ec2")

	defer func() {
		os.Unsetenv("AWS_K8S_TESTER_EC2_WAIT_BEFORE_DOWN")
		os.Unsetenv("AWS_K8S_TESTER_EC2_CLUSTER_SIZE")
		os.Unsetenv("AWS_K8S_TESTER_EC2_AWS_REGION")
		os.Unsetenv("AWS_K8S_TESTER_EC2_CONFIG_PATH")
		os.Unsetenv("AWS_K8S_TESTER_EC2_DOWN")
		os.Unsetenv("AWS_K8S_TESTER_EC2_LOG_LEVEL")
		os.Unsetenv("AWS_K8S_TESTER_EC2_UPLOAD_AWS_TESTER_LOGS")
		os.Unsetenv("AWS_K8S_TESTER_EC2_UPLOAD_BUCKET_EXPIRE_DAYS")
		os.Unsetenv("AWS_K8S_TESTER_EC2_VPC_ID")
		os.Unsetenv("AWS_K8S_TESTER_EC2_PLUGINS")
		os.Unsetenv("AWS_K8S_TESTER_EC2_INSTANCE_TYPE")
		os.Unsetenv("AWS_K8S_TESTER_EC2_KEY_NAME")
		os.Unsetenv("AWS_K8S_TESTER_EC2_KEY_PATH")
		os.Unsetenv("AWS_K8S_TESTER_EC2_KEY_CREATED")
		os.Unsetenv("AWS_K8S_TESTER_EC2_ASSOCIATE_PUBLIC_IP_ADDRESS")
		os.Unsetenv("AWS_K8S_TESTER_EC2_SUBNET_IDS")
		os.Unsetenv("AWS_K8S_TESTER_EC2_SECURITY_GROUP_IDS")
		os.Unsetenv("AWS_K8S_TESTER_EC2_WAIT")
		os.Unsetenv("AWS_K8S_TESTER_EC2_TAGS")
		os.Unsetenv("AWS_K8S_TESTER_EC2_INGRESS_RULES_TCP")
		os.Unsetenv("AWS_K8S_TESTER_EC2_VOLUME_SIZE")
		os.Unsetenv("AWS_K8S_TESTER_EC2_VPC_CIDR")
		os.Unsetenv("AWS_K8S_TESTER_EC2_INSTANCE_PROFILE_FILE_PATH")
	}()

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}

	if cfg.WaitBeforeDown != 2*time.Hour {
		t.Fatalf("unexpected WaitBeforeDown, got %v", cfg.WaitBeforeDown)
	}
	if cfg.ClusterSize != 100 {
		t.Fatalf("ClusterSize expected 100, got %d", cfg.ClusterSize)
	}
	if cfg.AWSRegion != "us-east-1" {
		t.Fatalf("AWSRegion unexpected %q", cfg.AWSRegion)
	}
	if cfg.ConfigPath != "test-path" {
		t.Fatalf("ConfigPath unexpected %q", cfg.ConfigPath)
	}
	if cfg.Down {
		t.Fatalf("Down unexpected %v", cfg.Down)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel unexpected %q", cfg.LogLevel)
	}
	if cfg.UploadTesterLogs {
		t.Fatalf("UploadTesterLogs unexpected %v", cfg.UploadTesterLogs)
	}
	if cfg.UploadBucketExpireDays != 3 {
		t.Fatalf("UploadBucketExpireDays expected 3, got %d", cfg.UploadBucketExpireDays)
	}
	if cfg.VPCID != "aaa" {
		t.Fatalf("VPCID unexpected %q", cfg.VPCID)
	}
	if !reflect.DeepEqual(cfg.Plugins, []string{"update-amazon-linux-2", "install-go-1.11.3"}) {
		t.Fatalf("unexpected plugins, got %v", cfg.Plugins)
	}
	if cfg.InstanceType != "m5d.2xlarge" {
		t.Fatalf("InstanceType unexpected %q", cfg.InstanceType)
	}
	if cfg.KeyName != "test-key" {
		t.Fatalf("KeyName unexpected %q", cfg.KeyName)
	}
	if cfg.KeyPath != "/root/.ssh/kube_aws_rsa" {
		t.Fatalf("KeyPath unexpected %q", cfg.KeyPath)
	}
	if !cfg.KeyCreated {
		t.Fatalf("KeyCreated unexpected %v", cfg.KeyCreated)
	}
	if cfg.AssociatePublicIPAddress {
		t.Fatalf("AssociatePublicIPAddress unexpected %v", cfg.AssociatePublicIPAddress)
	}
	if !reflect.DeepEqual(cfg.SubnetIDs, []string{"a", "b", "c"}) {
		t.Fatalf("SubnetIDs unexpected %v", cfg.SubnetIDs)
	}
	if !reflect.DeepEqual(cfg.SecurityGroupIDs, []string{"d", "e", "f"}) {
		t.Fatalf("SecurityGroupIDs unexpected %v", cfg.SecurityGroupIDs)
	}
	if !cfg.Wait {
		t.Fatalf("Wait expected true, got %v", cfg.Wait)
	}
	tt := map[string]string{
		"kubernetes.io/cluster/a8-ec2-190222-9dxccww": "owned",
	}
	if !reflect.DeepEqual(cfg.Tags, tt) {
		t.Fatalf("Tags expected %v, got %v", tt, cfg.Tags)
	}
	m := map[string]string{
		"22":        "0.0.0.0/0",
		"2379-2380": "192.168.0.0/8",
	}
	if !reflect.DeepEqual(cfg.IngressRulesTCP, m) {
		t.Fatalf("IngressRulesTCP expected %v, got %v", m, cfg.IngressRulesTCP)
	}
	if cfg.VolumeSize != 120 {
		t.Fatalf("VolumeSize expected 120, got %d", cfg.VolumeSize)
	}
	if cfg.VPCCIDR != "192.168.0.0/8" {
		t.Fatalf("VPCCIDR expected '192.168.0.0/8', got %q", cfg.VPCCIDR)
	}
	if cfg.InstanceProfileFilePath != "/tmp/aws-k8s-tester-ec2" {
		t.Fatalf("InstanceProfileFilePath expected '/tmp/aws-k8s-tester-ec2', got %q", cfg.InstanceProfileFilePath)
	}
}
