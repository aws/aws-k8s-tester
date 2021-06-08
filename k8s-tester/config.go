package k8s_tester

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-k8s-tester/client"
	cloudwatch_agent "github.com/aws/aws-k8s-tester/k8s-tester/cloudwatch-agent"
	"github.com/aws/aws-k8s-tester/k8s-tester/clusterloader"
	"github.com/aws/aws-k8s-tester/k8s-tester/configmaps"
	"github.com/aws/aws-k8s-tester/k8s-tester/conformance"
	csi_ebs "github.com/aws/aws-k8s-tester/k8s-tester/csi-ebs"
	"github.com/aws/aws-k8s-tester/k8s-tester/csrs"
	fluent_bit "github.com/aws/aws-k8s-tester/k8s-tester/fluent-bit"
	jobs_echo "github.com/aws/aws-k8s-tester/k8s-tester/jobs-echo"
	jobs_pi "github.com/aws/aws-k8s-tester/k8s-tester/jobs-pi"
	kubernetes_dashboard "github.com/aws/aws-k8s-tester/k8s-tester/kubernetes-dashboard"
	metrics_server "github.com/aws/aws-k8s-tester/k8s-tester/metrics-server"
	nlb_hello_world "github.com/aws/aws-k8s-tester/k8s-tester/nlb-hello-world"
	php_apache "github.com/aws/aws-k8s-tester/k8s-tester/php-apache"
	"github.com/aws/aws-k8s-tester/k8s-tester/secrets"
	"github.com/aws/aws-k8s-tester/k8s-tester/stress"
	stress_in_cluster "github.com/aws/aws-k8s-tester/k8s-tester/stress/in-cluster"
	aws_v1_ecr "github.com/aws/aws-k8s-tester/utils/aws/v1/ecr"
	"github.com/aws/aws-k8s-tester/utils/file"
	"github.com/aws/aws-k8s-tester/utils/log"
	"github.com/aws/aws-k8s-tester/utils/rand"
	utils_time "github.com/aws/aws-k8s-tester/utils/time"
	"github.com/mitchellh/colorstring"
	"sigs.k8s.io/yaml"
)

// Config defines k8s-tester configurations.
// tester order is defined as https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eks/eks.go#L617
// By default, it uses the environmental variables as https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eksconfig/env.go.
// TODO: support https://github.com/onsi/ginkgo.
type Config struct {
	mu *sync.RWMutex `json:"-"`

	// Prompt is true to enable prompt mode.
	Prompt bool `json:"prompt"`

	// ClusterName is the Kubernetes cluster name.
	ClusterName string `json:"cluster_name"`
	// ConfigPath is the configuration file path.
	ConfigPath string `json:"config_path"`

	// LogColor is true to output logs in color.
	LogColor bool `json:"log_color"`
	// LogColorOverride is not empty to override "LogColor" setting.
	// If not empty, the automatic color check is not even run and use this value instead.
	// For instance, github action worker might not support color device,
	// thus exiting color check with the exit code 1.
	// Useful to output in color in HTML based log outputs (e.g., Prow).
	// Useful to skip terminal color check when there is no color device (e.g., Github action worker).
	LogColorOverride string `json:"log_color_override"`
	// LogLevel configures log level. Only supports debug, info, warn, error, panic, or fatal. Default 'info'.
	LogLevel string `json:"log_level"`
	// LogOutputs is a list of log outputs. Valid values are 'default', 'stderr', 'stdout', or file names.
	// Logs are appended to the existing file, if any.
	// Multiple values are accepted. If empty, it sets to 'default', which outputs to stderr.
	// See https://pkg.go.dev/go.uber.org/zap#Open and https://pkg.go.dev/go.uber.org/zap#Config for more details.
	LogOutputs []string `json:"log_outputs"`

	KubectlDownloadURL string `json:"kubectl_download_url"`
	KubectlPath        string `json:"kubectl_path"`
	KubeconfigPath     string `json:"kubeconfig_path"`
	KubeconfigContext  string `json:"kubeconfig_context"`

	// Clients is the number of kubernetes clients to create.
	// Default is 1.
	// This field is used for "eks/stresser" tester. Configure accordingly.
	// Rate limit is done via "k8s.io/client-go/util/flowcontrol.NewTokenBucketRateLimiter".
	Clients int `json:"clients"`
	// ClientQPS is the QPS for kubernetes client.
	// To use while talking with kubernetes apiserver.
	//
	// Kubernetes client DefaultQPS is 5.
	// Kubernetes client DefaultBurst is 10.
	// ref. https://github.com/kubernetes/kubernetes/blob/4d0e86f0b8d1eae00a202009858c8739e4c9402e/staging/src/k8s.io/client-go/rest/config.go#L43-L46
	//
	// kube-apiserver default inflight requests limits are:
	// FLAG: --max-mutating-requests-inflight="200"
	// FLAG: --max-requests-inflight="400"
	// ref. https://github.com/kubernetes/kubernetes/blob/4d0e86f0b8d1eae00a202009858c8739e4c9402e/staging/src/k8s.io/apiserver/pkg/server/config.go#L300-L301
	//
	// This field is used for "eks/stresser" tester. Configure accordingly.
	// Rate limit is done via "k8s.io/client-go/util/flowcontrol.NewTokenBucketRateLimiter".
	ClientQPS float32 `json:"client_qps"`
	// ClientBurst is the burst for kubernetes client.
	// To use while talking with kubernetes apiserver
	//
	// Kubernetes client DefaultQPS is 5.
	// Kubernetes client DefaultBurst is 10.
	// ref. https://github.com/kubernetes/kubernetes/blob/4d0e86f0b8d1eae00a202009858c8739e4c9402e/staging/src/k8s.io/client-go/rest/config.go#L43-L46
	//
	// kube-apiserver default inflight requests limits are:
	// FLAG: --max-mutating-requests-inflight="200"
	// FLAG: --max-requests-inflight="400"
	// ref. https://github.com/kubernetes/kubernetes/blob/4d0e86f0b8d1eae00a202009858c8739e4c9402e/staging/src/k8s.io/apiserver/pkg/server/config.go#L300-L301
	//
	// This field is used for "eks/stresser" tester. Configure accordingly.
	// Rate limit is done via "k8s.io/client-go/util/flowcontrol.NewTokenBucketRateLimiter".
	ClientBurst int `json:"client_burst"`
	// ClientTimeout is the client timeout.
	ClientTimeout       time.Duration `json:"client_timeout"`
	ClientTimeoutString string        `json:"client_timeout_string,omitempty" read-only:"true"`

	// MinimumNodes is the minimum number of Kubernetes nodes required for installing this addon.
	MinimumNodes int `json:"minimum_nodes"`
	// TotalNodes is the total number of nodes from all node groups.
	TotalNodes int `json:"total_nodes" read-only:"true"`

	// tester order is defined as https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eks/eks.go#L617
	AddOnCloudwatchAgent     *cloudwatch_agent.Config     `json:"add_on_cloudwatch_agent"`
	AddOnFluentBit           *fluent_bit.Config           `json:"add_on_fluent_bit"`
	AddOnMetricsServer       *metrics_server.Config       `json:"add_on_metrics_server"`
	AddOnConformance         *conformance.Config          `json:"add_on_conformance"`
	AddOnCSIEBS              *csi_ebs.Config              `json:"add_on_csi_ebs"`
	AddOnKubernetesDashboard *kubernetes_dashboard.Config `json:"add_on_kubernetes_dashboard"`
	AddOnPHPApache           *php_apache.Config           `json:"add_on_php_apache"`
	AddOnNLBHelloWorld       *nlb_hello_world.Config      `json:"add_on_nlb_hello_world"`
	AddOnJobsPi              *jobs_pi.Config              `json:"add_on_jobs_pi"`
	AddOnJobsEcho            *jobs_echo.Config            `json:"add_on_jobs_echo"`
	AddOnCronJobsEcho        *jobs_echo.Config            `json:"add_on_cron_jobs_echo"`
	AddOnCSRs                *csrs.Config                 `json:"add_on_csrs"`
	AddOnConfigmaps          *configmaps.Config           `json:"add_on_configmaps"`
	AddOnSecrets             *secrets.Config              `json:"add_on_secrets"`
	AddOnClusterloader       *clusterloader.Config        `json:"add_on_clusterloader"`
	AddOnStress              *stress.Config               `json:"add_on_stress"`
	AddOnStressInCluster     *stress_in_cluster.Config    `json:"add_on_stress_in_cluster"`
}

const (
	// DefaultClients is the default number of clients to create.
	DefaultClients = 2
	// DefaultClientQPS is the default client QPS.
	DefaultClientQPS float32 = 10
	// DefaultClientBurst is the default client burst.
	DefaultClientBurst = 20
	// DefaultClientTimeout is the default client timeout.
	DefaultClientTimeout = 15 * time.Second
	DefaultMinimumNodes  = 1
)

func NewDefault() *Config {
	name := fmt.Sprintf("k8s-%s-%s", utils_time.GetTS(10), rand.String(12))
	if v := os.Getenv(ENV_PREFIX + "CLUSTER_NAME"); v != "" {
		name = v
	}

	return &Config{
		mu: new(sync.RWMutex),

		Prompt:      true,
		ClusterName: name,

		LogColor:         true,
		LogColorOverride: "",
		LogLevel:         log.DefaultLogLevel,
		// default, stderr, stdout, or file name
		// log file named with cluster name will be added automatically
		LogOutputs: []string{"stderr"},

		KubectlPath:        client.DefaultKubectlPath(),
		KubectlDownloadURL: client.DefaultKubectlDownloadURL(),

		// Kubernetes client DefaultQPS is 5.
		// Kubernetes client DefaultBurst is 10.
		// ref. https://github.com/kubernetes/kubernetes/blob/4d0e86f0b8d1eae00a202009858c8739e4c9402e/staging/src/k8s.io/client-go/rest/config.go#L43-L46
		//
		// kube-apiserver default inflight requests limits are:
		// FLAG: --max-mutating-requests-inflight="200"
		// FLAG: --max-requests-inflight="400"
		// ref. https://github.com/kubernetes/kubernetes/blob/4d0e86f0b8d1eae00a202009858c8739e4c9402e/staging/src/k8s.io/apiserver/pkg/server/config.go#L300-L301
		//
		Clients:       DefaultClients,
		ClientQPS:     DefaultClientQPS,
		ClientBurst:   DefaultClientBurst,
		ClientTimeout: DefaultClientTimeout,

		MinimumNodes: DefaultMinimumNodes,

		// tester order is defined as https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eks/eks.go#L617
		AddOnCloudwatchAgent:     cloudwatch_agent.NewDefault(),
		AddOnFluentBit:           fluent_bit.NewDefault(),
		AddOnMetricsServer:       metrics_server.NewDefault(),
		AddOnConformance:         conformance.NewDefault(),
		AddOnCSIEBS:              csi_ebs.NewDefault(),
		AddOnKubernetesDashboard: kubernetes_dashboard.NewDefault(),
		AddOnPHPApache:           php_apache.NewDefault(),
		AddOnNLBHelloWorld:       nlb_hello_world.NewDefault(),
		AddOnJobsPi:              jobs_pi.NewDefault(),
		AddOnJobsEcho:            jobs_echo.NewDefault("Job"),
		AddOnCronJobsEcho:        jobs_echo.NewDefault("CronJob"),
		AddOnCSRs:                csrs.NewDefault(),
		AddOnConfigmaps:          configmaps.NewDefault(),
		AddOnSecrets:             secrets.NewDefault(),
		AddOnClusterloader:       clusterloader.NewDefault(),
		AddOnStress:              stress.NewDefault(),
		AddOnStressInCluster:     stress_in_cluster.NewDefault(),
	}
}

// ValidateAndSetDefaults returns an error for invalid configurations.
// And updates empty fields with default values.
// At the end, it writes populated YAML to aws-k8s-tester config path.
// "read-only" fields cannot be set, causing errors.
func (cfg *Config) ValidateAndSetDefaults() error {
	if cfg.mu == nil {
		cfg.mu = new(sync.RWMutex)
	}
	cfg.mu.Lock()
	defer func() {
		if serr := cfg.unsafeSync(); serr != nil {
			fmt.Fprintf(os.Stderr, "[WARN] failed to sync config files %v\n", serr)
		}
		cfg.mu.Unlock()
	}()

	if err := cfg.validateConfig(); err != nil {
		return fmt.Errorf("validateConfig failed [%v]", err)
	}

	if cfg.AddOnCloudwatchAgent != nil && cfg.AddOnCloudwatchAgent.Enable {
		if err := cfg.AddOnCloudwatchAgent.ValidateAndSetDefaults(cfg.ClusterName); err != nil {
			return err
		}
	}
	if cfg.AddOnFluentBit != nil && cfg.AddOnFluentBit.Enable {
		if err := cfg.AddOnFluentBit.ValidateAndSetDefaults(); err != nil {
			return err
		}
	}
	if cfg.AddOnMetricsServer != nil && cfg.AddOnMetricsServer.Enable {
		if err := cfg.AddOnMetricsServer.ValidateAndSetDefaults(); err != nil {
			return err
		}
	}
	if cfg.AddOnConformance != nil && cfg.AddOnConformance.Enable {
		if err := cfg.AddOnConformance.ValidateAndSetDefaults(); err != nil {
			return err
		}
	}
	if cfg.AddOnCSIEBS != nil && cfg.AddOnCSIEBS.Enable {
		if err := cfg.AddOnCSIEBS.ValidateAndSetDefaults(); err != nil {
			return err
		}
	}
	if cfg.AddOnKubernetesDashboard != nil && cfg.AddOnKubernetesDashboard.Enable {
		if err := cfg.AddOnKubernetesDashboard.ValidateAndSetDefaults(); err != nil {
			return err
		}
	}
	if cfg.AddOnPHPApache != nil && cfg.AddOnPHPApache.Enable {
		if err := cfg.AddOnPHPApache.ValidateAndSetDefaults(); err != nil {
			return err
		}
	}
	if cfg.AddOnNLBHelloWorld != nil && cfg.AddOnNLBHelloWorld.Enable {
		if err := cfg.AddOnNLBHelloWorld.ValidateAndSetDefaults(); err != nil {
			return err
		}
	}
	if cfg.AddOnJobsPi != nil && cfg.AddOnJobsPi.Enable {
		if err := cfg.AddOnJobsPi.ValidateAndSetDefaults(); err != nil {
			return err
		}
	}
	if cfg.AddOnJobsEcho != nil && cfg.AddOnJobsEcho.Enable {
		if err := cfg.AddOnJobsEcho.ValidateAndSetDefaults(); err != nil {
			return err
		}
	}
	if cfg.AddOnCronJobsEcho != nil && cfg.AddOnCronJobsEcho.Enable {
		if err := cfg.AddOnCronJobsEcho.ValidateAndSetDefaults(); err != nil {
			return err
		}
	}
	if cfg.AddOnCSRs != nil && cfg.AddOnCSRs.Enable {
		if err := cfg.AddOnCSRs.ValidateAndSetDefaults(); err != nil {
			return err
		}
	}
	if cfg.AddOnConfigmaps != nil && cfg.AddOnConfigmaps.Enable {
		if err := cfg.AddOnConfigmaps.ValidateAndSetDefaults(); err != nil {
			return err
		}
	}
	if cfg.AddOnSecrets != nil && cfg.AddOnSecrets.Enable {
		if err := cfg.AddOnSecrets.ValidateAndSetDefaults(); err != nil {
			return err
		}
	}
	if cfg.AddOnClusterloader != nil && cfg.AddOnClusterloader.Enable {
		if err := cfg.AddOnClusterloader.ValidateAndSetDefaults(); err != nil {
			return err
		}
	}
	if cfg.AddOnStress != nil && cfg.AddOnStress.Enable {
		if err := cfg.AddOnStress.ValidateAndSetDefaults(); err != nil {
			return err
		}
	}
	if cfg.AddOnStressInCluster != nil && cfg.AddOnStressInCluster.Enable {
		if err := cfg.AddOnStressInCluster.ValidateAndSetDefaults(); err != nil {
			return err
		}
	}

	return nil
}

func (cfg *Config) validateConfig() error {
	if len(cfg.ClusterName) == 0 {
		return errors.New("ClusterName is empty")
	}
	if cfg.ClusterName != strings.ToLower(cfg.ClusterName) {
		return fmt.Errorf("ClusterName %q must be in lower-case", cfg.ClusterName)
	}

	if cfg.Clients == 0 {
		cfg.Clients = DefaultClients
	}
	if cfg.ClientQPS == 0 {
		cfg.ClientQPS = DefaultClientQPS
	}
	if cfg.ClientBurst == 0 {
		cfg.ClientBurst = DefaultClientBurst
	}
	if cfg.ClientTimeout == time.Duration(0) {
		cfg.ClientTimeout = DefaultClientTimeout
	}
	cfg.ClientTimeoutString = cfg.ClientTimeout.String()

	if cfg.ConfigPath == "" {
		rootDir, err := os.Getwd()
		if err != nil {
			rootDir = filepath.Join(os.TempDir(), cfg.ClusterName)
			if err := os.MkdirAll(rootDir, 0700); err != nil {
				return err
			}
		}
		cfg.ConfigPath = filepath.Join(rootDir, cfg.ClusterName+".k8s-tester.yaml")
		var p string
		p, err = filepath.Abs(cfg.ConfigPath)
		if err != nil {
			panic(err)
		}
		cfg.ConfigPath = p
	}
	if err := os.MkdirAll(filepath.Dir(cfg.ConfigPath), 0700); err != nil {
		return err
	}
	if err := file.IsDirWriteable(filepath.Dir(cfg.ConfigPath)); err != nil {
		return err
	}

	if len(cfg.LogOutputs) == 1 && (cfg.LogOutputs[0] == "stderr" || cfg.LogOutputs[0] == "stdout") {
		cfg.LogOutputs = append(cfg.LogOutputs, strings.ReplaceAll(cfg.ConfigPath, ".yaml", "")+".log")
	}
	logFilePath := ""
	for _, fpath := range cfg.LogOutputs {
		if filepath.Ext(fpath) == ".log" {
			logFilePath = fpath
			break
		}
	}
	if logFilePath == "" {
		return fmt.Errorf("*.log file not found in %q", cfg.LogOutputs)
	}

	return nil
}

// ENV_PREFIX is the environment variable prefix.
const ENV_PREFIX = "K8S_TESTER_"

func Load(p string) (cfg *Config, err error) {
	var d []byte
	d, err = ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}
	cfg = new(Config)
	if err = yaml.Unmarshal(d, cfg, yaml.DisallowUnknownFields); err != nil {
		return nil, err
	}

	cfg.mu = new(sync.RWMutex)
	if cfg.ConfigPath != p {
		cfg.ConfigPath = p
	}

	var ap string
	ap, err = filepath.Abs(p)
	if err != nil {
		return nil, err
	}
	cfg.ConfigPath = ap

	if serr := cfg.unsafeSync(); serr != nil {
		fmt.Fprintf(os.Stderr, "[WARN] failed to sync config files %v\n", serr)
	}

	return cfg, nil
}

// Sync writes the configuration file to disk.
func (cfg *Config) Sync() error {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()
	return cfg.unsafeSync()
}

func (cfg *Config) unsafeSync() error {
	if cfg.ConfigPath == "" {
		return errors.New("empty config path")
	}

	if cfg.ConfigPath != "" && !filepath.IsAbs(cfg.ConfigPath) {
		p, err := filepath.Abs(cfg.ConfigPath)
		if err != nil {
			return fmt.Errorf("failed to 'filepath.Abs(%s)' %v", cfg.ConfigPath, err)
		}
		cfg.ConfigPath = p
	}
	if cfg.KubeconfigPath != "" && !filepath.IsAbs(cfg.KubeconfigPath) {
		p, err := filepath.Abs(cfg.KubeconfigPath)
		if err != nil {
			return fmt.Errorf("failed to 'filepath.Abs(%s)' %v", cfg.KubeconfigPath, err)
		}
		cfg.KubeconfigPath = p
	}

	d, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to 'yaml.Marshal' %v", err)
	}
	err = ioutil.WriteFile(cfg.ConfigPath, d, 0600)
	if err != nil {
		return fmt.Errorf("failed to write file %q (%v)", cfg.ConfigPath, err)
	}

	return nil
}

// UpdateFromEnvs updates fields from environmental variables.
// Empty values are ignored and do not overwrite fields with empty values.
// WARNING: The environmental variable value always overwrites current field
// values if there's a conflict.
func (cfg *Config) UpdateFromEnvs() (err error) {
	var vv interface{}
	vv, err = parseEnvs(ENV_PREFIX, cfg)
	if err != nil {
		return err
	}
	if av, ok := vv.(*Config); ok {
		cfg = av
	} else {
		return fmt.Errorf("expected *Config, got %T", vv)
	}

	// tester order is defined as https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eks/eks.go#L617
	vv, err = parseEnvs(ENV_PREFIX+cloudwatch_agent.Env()+"_", cfg.AddOnCloudwatchAgent)
	if err != nil {
		return err
	}
	if av, ok := vv.(*cloudwatch_agent.Config); ok {
		cfg.AddOnCloudwatchAgent = av
	} else {
		return fmt.Errorf("expected *cloudwatch_agent.Config, got %T", vv)
	}

	vv, err = parseEnvs(ENV_PREFIX+fluent_bit.Env()+"_", cfg.AddOnFluentBit)
	if err != nil {
		return err
	}
	if av, ok := vv.(*fluent_bit.Config); ok {
		cfg.AddOnFluentBit = av
	} else {
		return fmt.Errorf("expected *fluent_bit.Config, got %T", vv)
	}

	vv, err = parseEnvs(ENV_PREFIX+metrics_server.Env()+"_", cfg.AddOnMetricsServer)
	if err != nil {
		return err
	}
	if av, ok := vv.(*metrics_server.Config); ok {
		cfg.AddOnMetricsServer = av
	} else {
		return fmt.Errorf("expected *metrics_server.Config, got %T", vv)
	}

	vv, err = parseEnvs(ENV_PREFIX+conformance.Env()+"_", cfg.AddOnConformance)
	if err != nil {
		return err
	}
	if av, ok := vv.(*conformance.Config); ok {
		cfg.AddOnConformance = av
	} else {
		return fmt.Errorf("expected *conformance.Config, got %T", vv)
	}

	vv, err = parseEnvs(ENV_PREFIX+csi_ebs.Env()+"_", cfg.AddOnCSIEBS)
	if err != nil {
		return err
	}
	if av, ok := vv.(*csi_ebs.Config); ok {
		cfg.AddOnCSIEBS = av
	} else {
		return fmt.Errorf("expected *csi_ebs.Config, got %T", vv)
	}

	vv, err = parseEnvs(ENV_PREFIX+kubernetes_dashboard.Env()+"_", cfg.AddOnKubernetesDashboard)
	if err != nil {
		return err
	}
	if av, ok := vv.(*kubernetes_dashboard.Config); ok {
		cfg.AddOnKubernetesDashboard = av
	} else {
		return fmt.Errorf("expected *kubernetes_dashboard.Config, got %T", vv)
	}

	vv, err = parseEnvs(ENV_PREFIX+php_apache.Env()+"_", cfg.AddOnPHPApache)
	if err != nil {
		return err
	}
	if av, ok := vv.(*php_apache.Config); ok {
		cfg.AddOnPHPApache = av
	} else {
		return fmt.Errorf("expected *php_apache.Config, got %T", vv)
	}
	if cfg.AddOnPHPApache != nil {
		vv, err = parseEnvs(ENV_PREFIX+php_apache.EnvRepository()+"_", cfg.AddOnPHPApache.Repository)
		if err != nil {
			return err
		}
		if av, ok := vv.(*aws_v1_ecr.Repository); ok {
			cfg.AddOnPHPApache.Repository = av
		} else {
			return fmt.Errorf("expected *aws_v1_ecr.Repository, got %T", vv)
		}
	}

	vv, err = parseEnvs(ENV_PREFIX+nlb_hello_world.Env()+"_", cfg.AddOnNLBHelloWorld)
	if err != nil {
		return err
	}
	if av, ok := vv.(*nlb_hello_world.Config); ok {
		cfg.AddOnNLBHelloWorld = av
	} else {
		return fmt.Errorf("expected *nlb_hello_world.Config, got %T", vv)
	}

	vv, err = parseEnvs(ENV_PREFIX+jobs_pi.Env()+"_", cfg.AddOnJobsPi)
	if err != nil {
		return err
	}
	if av, ok := vv.(*jobs_pi.Config); ok {
		cfg.AddOnJobsPi = av
	} else {
		return fmt.Errorf("expected *jobs_pi.Config, got %T", vv)
	}

	vv, err = parseEnvs(ENV_PREFIX+jobs_echo.Env("Job")+"_", cfg.AddOnJobsEcho)
	if err != nil {
		return err
	}
	if av, ok := vv.(*jobs_echo.Config); ok {
		cfg.AddOnJobsEcho = av
	} else {
		return fmt.Errorf("expected *jobs_echo.Config, got %T", vv)
	}
	if cfg.AddOnJobsEcho != nil {
		vv, err = parseEnvs(ENV_PREFIX+jobs_echo.EnvRepository("Job")+"_", cfg.AddOnJobsEcho.Repository)
		if err != nil {
			return err
		}
		if av, ok := vv.(*aws_v1_ecr.Repository); ok {
			cfg.AddOnJobsEcho.Repository = av
		} else {
			return fmt.Errorf("expected *aws_v1_ecr.Repository, got %T", vv)
		}
	}

	vv, err = parseEnvs(ENV_PREFIX+jobs_echo.Env("CronJob")+"_", cfg.AddOnCronJobsEcho)
	if err != nil {
		return err
	}
	if av, ok := vv.(*jobs_echo.Config); ok {
		cfg.AddOnCronJobsEcho = av
	} else {
		return fmt.Errorf("expected *jobs_echo.Config, got %T", vv)
	}
	if cfg.AddOnCronJobsEcho != nil {
		vv, err = parseEnvs(ENV_PREFIX+jobs_echo.EnvRepository("CronJob")+"_", cfg.AddOnCronJobsEcho.Repository)
		if err != nil {
			return err
		}
		if av, ok := vv.(*aws_v1_ecr.Repository); ok {
			cfg.AddOnCronJobsEcho.Repository = av
		} else {
			return fmt.Errorf("expected *aws_v1_ecr.Repository, got %T", vv)
		}
	}

	vv, err = parseEnvs(ENV_PREFIX+configmaps.Env()+"_", cfg.AddOnConfigmaps)
	if err != nil {
		return err
	}
	if av, ok := vv.(*configmaps.Config); ok {
		cfg.AddOnConfigmaps = av
	} else {
		return fmt.Errorf("expected *configmaps.Config, got %T", vv)
	}

	vv, err = parseEnvs(ENV_PREFIX+csrs.Env()+"_", cfg.AddOnCSRs)
	if err != nil {
		return err
	}
	if av, ok := vv.(*csrs.Config); ok {
		cfg.AddOnCSRs = av
	} else {
		return fmt.Errorf("expected *csrs.Config, got %T", vv)
	}

	vv, err = parseEnvs(ENV_PREFIX+secrets.Env()+"_", cfg.AddOnSecrets)
	if err != nil {
		return err
	}
	if av, ok := vv.(*secrets.Config); ok {
		cfg.AddOnSecrets = av
	} else {
		return fmt.Errorf("expected *secrets.Config, got %T", vv)
	}

	vv, err = parseEnvs(ENV_PREFIX+clusterloader.Env()+"_", cfg.AddOnClusterloader)
	if err != nil {
		return err
	}
	if av, ok := vv.(*clusterloader.Config); ok {
		cfg.AddOnClusterloader = av
	} else {
		return fmt.Errorf("expected *clusterloader.Config, got %T", vv)
	}
	if cfg.AddOnClusterloader != nil {
		vv, err = parseEnvs(ENV_PREFIX+clusterloader.EnvTestOverride()+"_", cfg.AddOnClusterloader.TestOverride)
		if err != nil {
			return err
		}
		if av, ok := vv.(*clusterloader.TestOverride); ok {
			cfg.AddOnClusterloader.TestOverride = av
		} else {
			return fmt.Errorf("expected *clusterloader.TestOverride, got %T", vv)
		}
	}

	vv, err = parseEnvs(ENV_PREFIX+stress.Env()+"_", cfg.AddOnStress)
	if err != nil {
		return err
	}
	if av, ok := vv.(*stress.Config); ok {
		cfg.AddOnStress = av
	} else {
		return fmt.Errorf("expected *stress.Config, got %T", vv)
	}
	if cfg.AddOnStress != nil {
		vv, err = parseEnvs(ENV_PREFIX+stress.EnvRepository()+"_", cfg.AddOnStress.Repository)
		if err != nil {
			return err
		}
		if av, ok := vv.(*aws_v1_ecr.Repository); ok {
			cfg.AddOnStress.Repository = av
		} else {
			return fmt.Errorf("expected *aws_v1_ecr.Repository, got %T", vv)
		}
	}

	vv, err = parseEnvs(ENV_PREFIX+stress_in_cluster.Env()+"_", cfg.AddOnStressInCluster)
	if err != nil {
		return err
	}
	if av, ok := vv.(*stress_in_cluster.Config); ok {
		cfg.AddOnStressInCluster = av
	} else {
		return fmt.Errorf("expected *stress_in_cluster.Config, got %T", vv)
	}
	if cfg.AddOnStressInCluster != nil {
		vv, err = parseEnvs(ENV_PREFIX+stress_in_cluster.EnvK8sTesterStressRepository()+"_", cfg.AddOnStressInCluster.K8sTesterStressRepository)
		if err != nil {
			return err
		}
		if av, ok := vv.(*aws_v1_ecr.Repository); ok {
			cfg.AddOnStressInCluster.K8sTesterStressRepository = av
		} else {
			return fmt.Errorf("expected *aws_v1_ecr.Repository, got %T", vv)
		}

		vv, err = parseEnvs(ENV_PREFIX+stress_in_cluster.EnvK8sTesterStressCLI()+"_", cfg.AddOnStressInCluster.K8sTesterStressCLI)
		if err != nil {
			return err
		}
		if av, ok := vv.(*stress_in_cluster.K8sTesterStressCLI); ok {
			cfg.AddOnStressInCluster.K8sTesterStressCLI = av
		} else {
			return fmt.Errorf("expected *stress_in_cluster.K8sTesterStressCLI, got %T", vv)
		}

		vv, err = parseEnvs(ENV_PREFIX+stress_in_cluster.EnvK8sTesterStressCLIBusyboxRepository()+"_", cfg.AddOnStressInCluster.K8sTesterStressCLI.BusyboxRepository)
		if err != nil {
			return err
		}
		if av, ok := vv.(*aws_v1_ecr.Repository); ok {
			cfg.AddOnStressInCluster.K8sTesterStressCLI.BusyboxRepository = av
		} else {
			return fmt.Errorf("expected *aws_v1_ecr.Repository, got %T", vv)
		}
	}

	return err
}

func parseEnvs(pfx string, addOn interface{}) (interface{}, error) {
	tp, vv := reflect.TypeOf(addOn).Elem(), reflect.ValueOf(addOn).Elem()
	for i := 0; i < tp.NumField(); i++ {
		jv := tp.Field(i).Tag.Get("json")
		if jv == "" {
			continue
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := pfx + jv
		sv := os.Getenv(env)
		if sv == "" {
			continue
		}
		if tp.Field(i).Tag.Get("read-only") == "true" { // error when read-only field is set for update
			return nil, fmt.Errorf("'%s=%s' is 'read-only' field; should not be set", env, sv)
		}
		fieldName := tp.Field(i).Name

		switch vv.Field(i).Type().Kind() {
		case reflect.String:
			vv.Field(i).SetString(sv)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %q (field name %q, environmental variable key %q, error %v)", sv, fieldName, env, err)
			}
			vv.Field(i).SetBool(bb)

		case reflect.Int, reflect.Int32, reflect.Int64:
			if vv.Field(i).Type().Name() == "Duration" {
				iv, err := time.ParseDuration(sv)
				if err != nil {
					return nil, fmt.Errorf("failed to parse %q (field name %q, environmental variable key %q, error %v)", sv, fieldName, env, err)
				}
				vv.Field(i).SetInt(int64(iv))
			} else {
				iv, err := strconv.ParseInt(sv, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("failed to parse %q (field name %q, environmental variable key %q, error %v)", sv, fieldName, env, err)
				}
				vv.Field(i).SetInt(iv)
			}

		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			iv, err := strconv.ParseUint(sv, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %q (field name %q, environmental variable key %q, error %v)", sv, fieldName, env, err)
			}
			vv.Field(i).SetUint(iv)

		case reflect.Float32, reflect.Float64:
			fv, err := strconv.ParseFloat(sv, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %q (field name %q, environmental variable key %q, error %v)", sv, fieldName, env, err)
			}
			vv.Field(i).SetFloat(fv)

		case reflect.Slice: // only supports "[]string" for now
			ss := strings.Split(sv, ",")
			if len(ss) < 1 {
				continue
			}
			slice := reflect.MakeSlice(reflect.TypeOf([]string{}), len(ss), len(ss))
			for j := range ss {
				slice.Index(j).SetString(ss[j])
			}
			vv.Field(i).Set(slice)

		case reflect.Map:
			switch fieldName {
			case "Tags",
				"NodeSelector",
				"DeploymentNodeSelector",
				"DeploymentNodeSelector2048":
				vv.Field(i).Set(reflect.ValueOf(make(map[string]string)))
				mm := make(map[string]string)
				if err := json.Unmarshal([]byte(sv), &mm); err != nil {
					return nil, fmt.Errorf("failed to parse %q (field name %q, environmental variable key %q, error %v)", sv, fieldName, env, err)
				}
				vv.Field(i).Set(reflect.ValueOf(mm))

			default:
				return nil, fmt.Errorf("field %q not supported for reflect.Map", fieldName)
			}
		}
	}
	return addOn, nil
}

// Colorize prints colorized input, if color output is supported.
func (cfg *Config) Colorize(input string) string {
	colorize := colorstring.Colorize{
		Colors:  colorstring.DefaultColors,
		Disable: !cfg.LogColor,
		Reset:   true,
	}
	return colorize.Color(input)
}

// KubectlCommand returns the kubectl command.
func (cfg *Config) KubectlCommand() string {
	return fmt.Sprintf("%s --kubeconfig=%s", cfg.KubectlPath, cfg.KubeconfigPath)
}

// KubectlCommands returns the various kubectl commands.
func (cfg *Config) KubectlCommands() (s string) {
	if cfg.KubeconfigPath == "" {
		return ""
	}
	tpl := template.Must(template.New("kubectlTmpl").Parse(kubectlTmpl))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, struct {
		KubeconfigPath string
		KubectlCommand string
	}{
		cfg.KubeconfigPath,
		cfg.KubectlCommand(),
	}); err != nil {
		return ""
	}
	return buf.String()
}

const kubectlTmpl = `
###########################
# kubectl commands
export KUBEcONFIG={{ .KubeconfigPath }}
export KUBECTL="{{ .KubectlCommand }}"

{{ .KubectlCommand }} version
{{ .KubectlCommand }} cluster-info
{{ .KubectlCommand }} get cs
{{ .KubectlCommand }} --namespace=kube-system get pods
{{ .KubectlCommand }} --namespace=kube-system get ds
{{ .KubectlCommand }} get pods
{{ .KubectlCommand }} get csr -o=yaml
{{ .KubectlCommand }} get nodes --show-labels -o=wide
{{ .KubectlCommand }} get nodes -o=wide
###########################
`
