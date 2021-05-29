package k8s_tester

import (
	"os"
	"reflect"
	"testing"
)

func TestEnv(t *testing.T) {
	cfg := NewDefault()

	os.Setenv("K8S_TESTER_PROMPT", "false")
	defer os.Unsetenv("K8S_TESTER_PROMPT")
	os.Setenv("K8S_TESTER_CLUSTER_NAME", "hello")
	defer os.Unsetenv("K8S_TESTER_CLUSTER_NAME")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}

	if cfg.ClusterName != "hello" {
		t.Fatalf("unexpected cfg.ClusterName %v", cfg.ClusterName)
	}
}

func TestEnvEmpty(t *testing.T) {
	cfg := NewDefault()

	os.Setenv("K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT_NAMESPACE", "hello")
	defer os.Unsetenv("K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT_NAMESPACE")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}

	// should be empty
	if cfg.CloudwatchAgent.Namespace == "hello" {
		t.Fatalf("unexpected cfg.CloudwatchAgent.Namespace %v", cfg.CloudwatchAgent.Namespace)
	}
}

func TestEnvCloudwatchAgent(t *testing.T) {
	cfg := NewDefault()

	os.Setenv("K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT", "hello")
	defer os.Unsetenv("K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT")
	os.Setenv("K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT_NAMESPACE", "hello")
	defer os.Unsetenv("K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT_NAMESPACE")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}

	if cfg.CloudwatchAgent.Namespace != "hello" {
		t.Fatalf("unexpected cfg.CloudwatchAgent.Namespace %v", cfg.CloudwatchAgent.Namespace)
	}
}

func TestEnvMetricsServer(t *testing.T) {
	cfg := NewDefault()

	os.Setenv("K8S_TESTER_ADD_ON_METRICS_SERVER", "hello")
	defer os.Unsetenv("K8S_TESTER_ADD_ON_METRICS_SERVER")
	os.Setenv("K8S_TESTER_ADD_ON_METRICS_SERVER_NAMESPACE", "hello")
	defer os.Unsetenv("K8S_TESTER_ADD_ON_METRICS_SERVER_NAMESPACE")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}

	if cfg.MetricsServer.Namespace != "hello" {
		t.Fatalf("unexpected cfg.MetricsServer.Namespace %v", cfg.MetricsServer.Namespace)
	}
}

func TestEnvFluentBit(t *testing.T) {
	cfg := NewDefault()

	os.Setenv("K8S_TESTER_ADD_ON_FLUENT_BIT", "hello")
	defer os.Unsetenv("K8S_TESTER_ADD_ON_FLUENT_BIT")
	os.Setenv("K8S_TESTER_ADD_ON_FLUENT_BIT_NAMESPACE", "hello")
	defer os.Unsetenv("K8S_TESTER_ADD_ON_FLUENT_BIT_NAMESPACE")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}

	if cfg.FluentBit.Namespace != "hello" {
		t.Fatalf("unexpected cfg.FluentBit.Namespace %v", cfg.FluentBit.Namespace)
	}
}

func TestEnvKubernetesDashboard(t *testing.T) {
	cfg := NewDefault()

	os.Setenv("K8S_TESTER_ADD_ON_KUBERNETES_DASHBOARD", "hello")
	defer os.Unsetenv("K8S_TESTER_ADD_ON_KUBERNETES_DASHBOARD")
	os.Setenv("K8S_TESTER_ADD_ON_KUBERNETES_DASHBOARD_MINIMUM_NODES", "10")
	defer os.Unsetenv("K8S_TESTER_ADD_ON_KUBERNETES_DASHBOARD_MINIMUM_NODES")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}

	if cfg.KubernetesDashboard.MinimumNodes != 10 {
		t.Fatalf("unexpected cfg.KubernetesDashboard.MinimumNodes %v", cfg.KubernetesDashboard.MinimumNodes)
	}
}

func TestEnvNLBHelloWorld(t *testing.T) {
	cfg := NewDefault()

	os.Setenv("K8S_TESTER_ADD_ON_NLB_HELLO_WORLD", "hello")
	defer os.Unsetenv("K8S_TESTER_ADD_ON_NLB_HELLO_WORLD")
	os.Setenv("K8S_TESTER_ADD_ON_NLB_HELLO_WORLD_NAMESPACE", "hello")
	defer os.Unsetenv("K8S_TESTER_ADD_ON_NLB_HELLO_WORLD_NAMESPACE")
	os.Setenv("K8S_TESTER_ADD_ON_NLB_HELLO_WORLD_MINIMUM_NODES", "100")
	defer os.Unsetenv("K8S_TESTER_ADD_ON_NLB_HELLO_WORLD_MINIMUM_NODES")
	os.Setenv("K8S_TESTER_ADD_ON_NLB_HELLO_WORLD_DEPLOYMENT_NODE_SELECTOR", `{"a":"b","c":"d"}`)
	defer os.Unsetenv("K8S_TESTER_ADD_ON_NLB_HELLO_WORLD_DEPLOYMENT_NODE_SELECTOR")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}

	if cfg.NLBHelloWorld.MinimumNodes != 100 {
		t.Fatalf("unexpected cfg.NLBHelloWorld.MinimumNodes %v", cfg.NLBHelloWorld.MinimumNodes)
	}
	if cfg.NLBHelloWorld.Namespace != "hello" {
		t.Fatalf("unexpected cfg.NLBHelloWorld.Namespace %v", cfg.NLBHelloWorld.Namespace)
	}
	if !reflect.DeepEqual(cfg.NLBHelloWorld.DeploymentNodeSelector, map[string]string{"a": "b", "c": "d"}) {
		t.Fatalf("unexpected cfg.NLBHelloWorld.DeploymentNodeSelector %v", cfg.NLBHelloWorld.DeploymentNodeSelector)
	}
}

func TestEnvJobsPi(t *testing.T) {
	cfg := NewDefault()

	os.Setenv("K8S_TESTER_ADD_ON_JOBS_PI", "hello")
	defer os.Unsetenv("K8S_TESTER_ADD_ON_JOBS_PI")
	os.Setenv("K8S_TESTER_ADD_ON_JOBS_PI_NAMESPACE", "hello")
	defer os.Unsetenv("K8S_TESTER_ADD_ON_JOBS_PI_NAMESPACE")
	os.Setenv("K8S_TESTER_ADD_ON_JOBS_PI_MINIMUM_NODES", "100")
	defer os.Unsetenv("K8S_TESTER_ADD_ON_JOBS_PI_MINIMUM_NODES")
	os.Setenv("K8S_TESTER_ADD_ON_JOBS_PI_COMPLETES", `222`)
	defer os.Unsetenv("K8S_TESTER_ADD_ON_JOBS_PI_COMPLETES")
	os.Setenv("K8S_TESTER_ADD_ON_JOBS_PI_PARALLELS", `333`)
	defer os.Unsetenv("K8S_TESTER_ADD_ON_JOBS_PI_PARALLELS")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}

	if cfg.JobsPi.MinimumNodes != 100 {
		t.Fatalf("unexpected cfg.JobsPi.MinimumNodes %v", cfg.JobsPi.MinimumNodes)
	}
	if cfg.JobsPi.Namespace != "hello" {
		t.Fatalf("unexpected cfg.JobsPi.Namespace %v", cfg.JobsPi.Namespace)
	}
	if cfg.JobsPi.Completes != 222 {
		t.Fatalf("unexpected cfg.JobsPi.Completes %v", cfg.JobsPi.Completes)
	}
	if cfg.JobsPi.Parallels != 333 {
		t.Fatalf("unexpected cfg.JobsPi.Parallels %v", cfg.JobsPi.Parallels)
	}
}

func TestEnvJobsEcho(t *testing.T) {
	cfg := NewDefault()

	os.Setenv("K8S_TESTER_ADD_ON_JOBS_ECHO", "hello")
	defer os.Unsetenv("K8S_TESTER_ADD_ON_JOBS_ECHO")
	os.Setenv("K8S_TESTER_ADD_ON_JOBS_ECHO_NAMESPACE", "hello")
	defer os.Unsetenv("K8S_TESTER_ADD_ON_JOBS_ECHO_NAMESPACE")
	os.Setenv("K8S_TESTER_ADD_ON_JOBS_ECHO_MINIMUM_NODES", "100")
	defer os.Unsetenv("K8S_TESTER_ADD_ON_JOBS_ECHO_MINIMUM_NODES")
	os.Setenv("K8S_TESTER_ADD_ON_JOBS_ECHO_COMPLETES", `222`)
	defer os.Unsetenv("K8S_TESTER_ADD_ON_JOBS_ECHO_COMPLETES")
	os.Setenv("K8S_TESTER_ADD_ON_JOBS_ECHO_PARALLELS", `333`)
	defer os.Unsetenv("K8S_TESTER_ADD_ON_JOBS_ECHO_PARALLELS")
	os.Setenv("K8S_TESTER_ADD_ON_JOBS_ECHO_ECHO_SIZE", `555`)
	defer os.Unsetenv("K8S_TESTER_ADD_ON_JOBS_ECHO_ECHO_SIZE")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}

	if cfg.JobsEcho.MinimumNodes != 100 {
		t.Fatalf("unexpected cfg.JobsEcho.MinimumNodes %v", cfg.JobsEcho.MinimumNodes)
	}
	if cfg.JobsEcho.Namespace != "hello" {
		t.Fatalf("unexpected cfg.JobsEcho.Namespace %v", cfg.JobsEcho.Namespace)
	}
	if cfg.JobsEcho.Completes != 222 {
		t.Fatalf("unexpected cfg.JobsEcho.Completes %v", cfg.JobsEcho.Completes)
	}
	if cfg.JobsEcho.Parallels != 333 {
		t.Fatalf("unexpected cfg.JobsEcho.Parallels %v", cfg.JobsEcho.Parallels)
	}
	if cfg.JobsEcho.EchoSize != 555 {
		t.Fatalf("unexpected cfg.JobsEcho.EchoSize %v", cfg.JobsEcho.EchoSize)
	}
}

func TestEnvCronJobsEcho(t *testing.T) {
	cfg := NewDefault()

	os.Setenv("K8S_TESTER_ADD_ON_CRON_JOBS_ECHO", "hello")
	defer os.Unsetenv("K8S_TESTER_ADD_ON_CRON_JOBS_ECHO")
	os.Setenv("K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_NAMESPACE", "hello")
	defer os.Unsetenv("K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_NAMESPACE")
	os.Setenv("K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_MINIMUM_NODES", "100")
	defer os.Unsetenv("K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_MINIMUM_NODES")
	os.Setenv("K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_COMPLETES", `222`)
	defer os.Unsetenv("K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_COMPLETES")
	os.Setenv("K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_PARALLELS", `333`)
	defer os.Unsetenv("K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_PARALLELS")
	os.Setenv("K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_ECHO_SIZE", `555`)
	defer os.Unsetenv("K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_ECHO_SIZE")
	os.Setenv("K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_SCHEDULE", `*/10 */10 * * *`)
	defer os.Unsetenv("K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_SCHEDULE")
	os.Setenv("K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_SUCCESSFUL_JOBS_HISTORY_LIMIT", "55555")
	defer os.Unsetenv("K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_SUCCESSFUL_JOBS_HISTORY_LIMIT")
	os.Setenv("K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_FAILED_JOBS_HISTORY_LIMIT", "77777")
	defer os.Unsetenv("K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_FAILED_JOBS_HISTORY_LIMIT")

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}

	if cfg.CronJobsEcho.MinimumNodes != 100 {
		t.Fatalf("unexpected cfg.CronJobsEcho.MinimumNodes %v", cfg.CronJobsEcho.MinimumNodes)
	}
	if cfg.CronJobsEcho.Namespace != "hello" {
		t.Fatalf("unexpected cfg.CronJobsEcho.Namespace %v", cfg.CronJobsEcho.Namespace)
	}
	if cfg.CronJobsEcho.Completes != 222 {
		t.Fatalf("unexpected cfg.CronJobsEcho.Completes %v", cfg.CronJobsEcho.Completes)
	}
	if cfg.CronJobsEcho.Parallels != 333 {
		t.Fatalf("unexpected cfg.CronJobsEcho.Parallels %v", cfg.CronJobsEcho.Parallels)
	}
	if cfg.CronJobsEcho.EchoSize != 555 {
		t.Fatalf("unexpected cfg.CronJobsEcho.EchoSize %v", cfg.CronJobsEcho.EchoSize)
	}
	if cfg.CronJobsEcho.Schedule != "*/10 */10 * * *" {
		t.Fatalf("unexpected cfg.CronJobsEcho.Schedule %v", cfg.CronJobsEcho.Schedule)
	}
	if cfg.CronJobsEcho.SuccessfulJobsHistoryLimit != 55555 {
		t.Fatalf("unexpected cfg.CronJobsEcho.SuccessfulJobsHistoryLimit %v", cfg.CronJobsEcho.SuccessfulJobsHistoryLimit)
	}
	if cfg.CronJobsEcho.FailedJobsHistoryLimit != 77777 {
		t.Fatalf("unexpected cfg.CronJobsEcho.FailedJobsHistoryLimit %v", cfg.CronJobsEcho.FailedJobsHistoryLimit)
	}
}
