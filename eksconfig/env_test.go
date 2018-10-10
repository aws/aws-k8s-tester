package eksconfig

import (
	"os"
	"testing"
	"time"
)

func TestEnv(t *testing.T) {
	cfg := NewDefault()

	os.Setenv("AWSTESTER_EKS_CONFIG_PATH", "test-path")
	os.Setenv("AWSTESTER_EKS_DOWN", "false")
	os.Setenv("AWSTESTER_EKS_ALB_TARGET_TYPE", "ip")
	os.Setenv("AWSTESTER_EKS_WORKER_NODE_ASG_MIN", "10")
	os.Setenv("AWSTESTER_EKS_WORKER_NODE_ASG_MAX", "10")
	os.Setenv("AWSTESTER_EKS_EMBEDDED", "true")
	os.Setenv("AWSTESTER_EKS_WAIT_BEFORE_DOWN", "2h")
	os.Setenv("AWSTESTER_EKS_ALB_TEST_EXPECT_QPS", "123.45")
	os.Setenv("AWSTESTER_EKS_ALB_TEST_MODE", "nginx")
	os.Setenv("AWSTESTER_EKS_ALB_ENABLE", "true")
	os.Setenv("AWSTESTER_EKS_ALB_ENABLE_SCALABILITY_TEST", "false")

	defer func() {
		os.Unsetenv("AWSTESTER_EKS_CONFIG_PATH")
		os.Unsetenv("AWSTESTER_EKS_DOWN")
		os.Unsetenv("AWSTESTER_EKS_ALB_TARGET_TYPE")
		os.Unsetenv("AWSTESTER_EKS_WORKER_NODE_ASG_MIN")
		os.Unsetenv("AWSTESTER_EKS_WORKER_NODE_ASG_MAX")
		os.Unsetenv("AWSTESTER_EKS_EMBEDDED")
		os.Unsetenv("AWSTESTER_EKS_WAIT_BEFORE_DOWN")
		os.Unsetenv("AWSTESTER_EKS_ALB_TEST_EXPECT_QPS")
		os.Unsetenv("AWSTESTER_EKS_ALB_TEST_MODE")
		os.Unsetenv("AWSTESTER_EKS_ALB_ENABLE")
		os.Unsetenv("AWSTESTER_EKS_ALB_ENABLE_SCALABILITY_TEST")
	}()

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}

	if cfg.ConfigPath != "test-path" {
		t.Fatalf("alb configuration path expected 'test-path', got %q", cfg.ConfigPath)
	}
	if cfg.Down {
		t.Fatalf("cfg.Down expected 'false', got %v", cfg.Down)
	}
	if cfg.WorkderNodeASGMin != 10 {
		t.Fatalf("worker nodes min expected 10, got %q", cfg.WorkderNodeASGMin)
	}
	if cfg.WorkderNodeASGMax != 10 {
		t.Fatalf("worker nodes min expected 10, got %q", cfg.WorkderNodeASGMax)
	}
	if cfg.ALBIngressController.TargetType != "ip" {
		t.Fatalf("alb target type expected ip, got %q", cfg.ALBIngressController.TargetType)
	}
	if !cfg.Embedded {
		t.Fatalf("embedded expected true, got %v", cfg.Embedded)
	}
	if cfg.WaitBeforeDown != 2*time.Hour {
		t.Fatalf("wait before down expected 2h, got %v", cfg.WaitBeforeDown)
	}
	if cfg.ALBIngressController.TestExpectQPS != 123.45 {
		t.Fatalf("cfg.ALBIngressController.TestExpectQPS expected 123.45, got %v", cfg.ALBIngressController.TestExpectQPS)
	}
	if cfg.ALBIngressController.TestMode != "nginx" {
		t.Fatalf("cfg.ALBIngressController.TestMode expected 'nginx', got %v", cfg.ALBIngressController.TestMode)
	}
	if !cfg.ALBIngressController.Enable {
		t.Fatalf("cfg.ALBIngressController.Enable expected 'true', got %v", cfg.ALBIngressController.Enable)
	}
	if cfg.ALBIngressController.EnableScalabilityTest {
		t.Fatalf("cfg.ALBIngressController.EnableScalabilityTest expected 'false', got %v", cfg.ALBIngressController.EnableScalabilityTest)
	}
}
