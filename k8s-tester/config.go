package k8s_tester

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	fluent_bit "github.com/aws/aws-k8s-tester/k8s-tester/fluent-bit"
	jobs_echo "github.com/aws/aws-k8s-tester/k8s-tester/jobs-echo"
	jobs_pi "github.com/aws/aws-k8s-tester/k8s-tester/jobs-pi"
	kubernetes_dashboard "github.com/aws/aws-k8s-tester/k8s-tester/kubernetes-dashboard"
	metrics_server "github.com/aws/aws-k8s-tester/k8s-tester/metrics-server"
	nlb_hello_world "github.com/aws/aws-k8s-tester/k8s-tester/nlb-hello-world"
	"go.uber.org/zap"
	"sigs.k8s.io/yaml"
)

// Config defines k8s-tester configurations.
// The tester order is defined as https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eksconfig/env.go.
// By default, it uses the environmental variables as https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eksconfig/env.go.
// TODO: support https://github.com/onsi/ginkgo.
type Config struct {
	mu           *sync.RWMutex  `json:"-"`
	Logger       *zap.Logger    `json:"-"`
	LogWriter    io.Writer      `json:"-"`
	Stopc        chan struct{}  `json:"-"`
	ClientConfig *client.Config `json:"-"`

	// ConfigPath is the configuration file path.
	ConfigPath string `json:"config_path"`

	Prompt            bool   `json:"prompt"`
	KubectlPath       string `json:"kubectl_path"`
	KubeconfigPath    string `json:"kubeconfig_path"`
	KubeconfigContext string `json:"kubeconfig_context"`
	ClusterName       string `json:"cluster_name"`

	// MinimumNodes is the minimum number of Kubernetes nodes required for installing this addon.
	MinimumNodes int `json:"minimum_nodes"`

	CloudwatchAgent     *cloudwatch_agent.Config     `json:"add_on_cloudwatch_agent"`
	MetricsServer       *metrics_server.Config       `json:"add_on_metrics_server"`
	FluentBit           *fluent_bit.Config           `json:"add_on_fluent_bit"`
	KubernetesDashboard *kubernetes_dashboard.Config `json:"add_on_kubernetes_dashboard"`

	NLBHelloWorld *nlb_hello_world.Config `json:"add_on_nlb_hello_world"`

	JobsPi       *jobs_pi.Config   `json:"add_on_jobs_pi"`
	JobsEcho     *jobs_echo.Config `json:"add_on_jobs_echo"`
	CronJobsEcho *jobs_echo.Config `json:"add_on_cron_jobs_echo"`
}

const DefaultMinimumNodes = 1

func NewDefault() *Config {
	return &Config{
		mu:     new(sync.RWMutex),
		Prompt: true,

		MinimumNodes: DefaultMinimumNodes,

		CloudwatchAgent:     cloudwatch_agent.NewDefault(),
		MetricsServer:       metrics_server.NewDefault(),
		FluentBit:           fluent_bit.NewDefault(),
		KubernetesDashboard: kubernetes_dashboard.NewDefault(),
		NLBHelloWorld:       nlb_hello_world.NewDefault(),
		JobsPi:              jobs_pi.NewDefault(),
		JobsEcho:            jobs_echo.NewDefault("Job"),
		CronJobsEcho:        jobs_echo.NewDefault("CronJob"),
	}
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

	if v := os.Getenv(ENV_PREFIX + cloudwatch_agent.Env()); v != "" {
		vv, err = parseEnvs(ENV_PREFIX+cloudwatch_agent.Env()+"_", cfg.CloudwatchAgent)
		if err != nil {
			return err
		}
		if av, ok := vv.(*cloudwatch_agent.Config); ok {
			cfg.CloudwatchAgent = av
		} else {
			return fmt.Errorf("expected *cloudwatch_agent.Config, got %T", vv)
		}
	}

	if v := os.Getenv(ENV_PREFIX + metrics_server.Env()); v != "" {
		vv, err = parseEnvs(ENV_PREFIX+metrics_server.Env()+"_", cfg.MetricsServer)
		if err != nil {
			return err
		}
		if av, ok := vv.(*metrics_server.Config); ok {
			cfg.MetricsServer = av
		} else {
			return fmt.Errorf("expected *metrics_server.Config, got %T", vv)
		}
	}

	if v := os.Getenv(ENV_PREFIX + fluent_bit.Env()); v != "" {
		vv, err = parseEnvs(ENV_PREFIX+fluent_bit.Env()+"_", cfg.FluentBit)
		if err != nil {
			return err
		}
		if av, ok := vv.(*fluent_bit.Config); ok {
			cfg.FluentBit = av
		} else {
			return fmt.Errorf("expected *fluent_bit.Config, got %T", vv)
		}
	}

	if v := os.Getenv(ENV_PREFIX + kubernetes_dashboard.Env()); v != "" {
		vv, err = parseEnvs(ENV_PREFIX+kubernetes_dashboard.Env()+"_", cfg.KubernetesDashboard)
		if err != nil {
			return err
		}
		if av, ok := vv.(*kubernetes_dashboard.Config); ok {
			cfg.KubernetesDashboard = av
		} else {
			return fmt.Errorf("expected *kubernetes_dashboard.Config, got %T", vv)
		}
	}

	if v := os.Getenv(ENV_PREFIX + nlb_hello_world.Env()); v != "" {
		vv, err = parseEnvs(ENV_PREFIX+nlb_hello_world.Env()+"_", cfg.NLBHelloWorld)
		if err != nil {
			return err
		}
		if av, ok := vv.(*nlb_hello_world.Config); ok {
			cfg.NLBHelloWorld = av
		} else {
			return fmt.Errorf("expected *nlb_hello_world.Config, got %T", vv)
		}
	}

	if v := os.Getenv(ENV_PREFIX + jobs_pi.Env()); v != "" {
		vv, err = parseEnvs(ENV_PREFIX+jobs_pi.Env()+"_", cfg.JobsPi)
		if err != nil {
			return err
		}
		if av, ok := vv.(*jobs_pi.Config); ok {
			cfg.JobsPi = av
		} else {
			return fmt.Errorf("expected *jobs_pi.Config, got %T", vv)
		}
	}

	if v := os.Getenv(ENV_PREFIX + jobs_echo.Env("Job")); v != "" {
		vv, err = parseEnvs(ENV_PREFIX+jobs_echo.Env("Job")+"_", cfg.JobsEcho)
		if err != nil {
			return err
		}
		if av, ok := vv.(*jobs_echo.Config); ok {
			cfg.JobsEcho = av
		} else {
			return fmt.Errorf("expected *jobs_echo.Config, got %T", vv)
		}
	}

	if v := os.Getenv(ENV_PREFIX + jobs_echo.Env("CronJob")); v != "" {
		vv, err = parseEnvs(ENV_PREFIX+jobs_echo.Env("CronJob")+"_", cfg.CronJobsEcho)
		if err != nil {
			return err
		}
		if av, ok := vv.(*jobs_echo.Config); ok {
			cfg.CronJobsEcho = av
		} else {
			return fmt.Errorf("expected *jobs_echo.Config, got %T", vv)
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
