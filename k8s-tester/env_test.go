package k8s_tester

import (
	"os"
	"testing"

	cloudwatch_agent "github.com/aws/aws-k8s-tester/k8s-tester/cloudwatch-agent"
)

func TestEnv(t *testing.T) {
	cfg := &Config{
		CloudWatchAgent: &cloudwatch_agent.Config{},
	}

	os.Setenv("K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT", "hello")
	defer os.Unsetenv("K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT")
	os.Setenv("K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT_NAMESPACE", "hello")
	defer os.Unsetenv("K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT_NAMESPACE")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}

	if cfg.CloudWatchAgent.Namespace != "hello" {
		t.Fatalf("unexpected cfg.CloudWatchAgent.Namespace %v", cfg.CloudWatchAgent.Namespace)
	}
}
