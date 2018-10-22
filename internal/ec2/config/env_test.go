package config

import (
	"os"
	"reflect"
	"testing"
)

func TestEnv(t *testing.T) {
	cfg := NewDefault()

	os.Setenv("AWSTESTER_EC2_AWS_REGION", "us-east-1")
	os.Setenv("AWSTESTER_EC2_CONFIG_PATH", "test-path")
	os.Setenv("AWSTESTER_EC2_DOWN", "false")
	os.Setenv("AWSTESTER_EC2_VPC_ID", "aaa")
	os.Setenv("AWSTESTER_EC2_PLUGINS", "update-ubuntu,go1.11.1-ubuntu")
	os.Setenv("AWSTESTER_EC2_INSTANCE_TYPE", "m5d.2xlarge")
	os.Setenv("AWSTESTER_EC2_KEY_NAME", "test-key")
	os.Setenv("AWSTESTER_EC2_ASSOCIATE_PUBLIC_IP_ADDRESS", "false")
	os.Setenv("AWSTESTER_EC2_SUBNET_IDS", "a,b,c")
	os.Setenv("AWSTESTER_EC2_SECURITY_GROUP_IDS", "d,e,f")

	defer func() {
		os.Unsetenv("AWSTESTER_EC2_AWS_REGION")
		os.Unsetenv("AWSTESTER_EC2_CONFIG_PATH")
		os.Unsetenv("AWSTESTER_EC2_DOWN")
		os.Unsetenv("AWSTESTER_EC2_VPC_ID")
		os.Unsetenv("AWSTESTER_EC2_PLUGINS")
		os.Unsetenv("AWSTESTER_EC2_INSTANCE_TYPE")
		os.Unsetenv("AWSTESTER_EC2_KEY_NAME")
		os.Unsetenv("AWSTESTER_EC2_ASSOCIATE_PUBLIC_IP_ADDRESS")
		os.Unsetenv("AWSTESTER_EC2_SUBNET_IDS")
		os.Unsetenv("AWSTESTER_EC2_SECURITY_GROUP_IDS")
	}()

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
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
	if cfg.VPCID != "aaa" {
		t.Fatalf("VPCID unexpected %q", cfg.VPCID)
	}
	if !reflect.DeepEqual(cfg.Plugins, []string{"update-ubuntu", "go1.11.1-ubuntu"}) {
		t.Fatalf("unexpected plugins, got %v", cfg.Plugins)
	}
	if cfg.InstanceType != "m5d.2xlarge" {
		t.Fatalf("InstanceType unexpected %q", cfg.InstanceType)
	}
	if cfg.KeyName != "test-key" {
		t.Fatalf("KeyName unexpected %q", cfg.KeyName)
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
}
