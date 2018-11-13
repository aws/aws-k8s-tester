// Package kubeadmconfig defines kubeadm configuration.
package kubeadmconfig

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"

	"github.com/blang/semver"
	gyaml "github.com/ghodss/yaml"
)

// Config defines kubeadm test configuration.
type Config struct {
	// Tag is the tag used for S3 bucket name.
	// If empty, deployer auto-populates it.
	Tag string `json:"tag,omitempty"`
	// ClusterName is the cluster name.
	// If empty, deployer auto-populates it.
	ClusterName string `json:"cluster-name,omitempty"`

	// WaitBeforeDown is the duration to sleep before cluster tear down.
	WaitBeforeDown time.Duration `json:"wait-before-down,omitempty"`
	// Down is true to automatically tear down cluster in "test".
	// Deployer implementation should not call "Down" inside "Up" method.
	// This is meant to be used as a flag for test.
	Down bool `json:"down"`

	// ConfigPath is the configuration file path.
	// Must be left empty, and let deployer auto-populate this field.
	// Deployer is expected to update this file with latest status,
	// and to make a backup of original configuration
	// with the filename suffix ".backup.yaml" in the same directory.
	ConfigPath       string `json:"config-path,omitempty"`
	ConfigPathBucket string `json:"config-path-bucket,omitempty"`
	ConfigPathURL    string `json:"config-path-url,omitempty"`

	// LogDebug is true to enable debug level logging.
	LogDebug bool `json:"log-debug"`
	// LogOutputs is a list of log outputs. Valid values are 'default', 'stderr', 'stdout', or file names.
	// Logs are appended to the existing file, if any.
	// Multiple values are accepted. If empty, it sets to 'default', which outputs to stderr.
	// See https://godoc.org/go.uber.org/zap#Open and https://godoc.org/go.uber.org/zap#Config for more details.
	LogOutputs []string `json:"log-outputs,omitempty"`
	// LogOutputToUploadPath is the aws-k8s-tester log file path to upload to cloud storage.
	// Must be left empty.
	// This will be overwritten by cluster name.
	LogOutputToUploadPath       string `json:"log-output-to-upload-path,omitempty"`
	LogOutputToUploadPathBucket string `json:"log-output-to-upload-path-bucket,omitempty"`
	LogOutputToUploadPathURL    string `json:"log-output-to-upload-path-url,omitempty"`

	// Logs is a list of node log file paths, fetched via SSH.
	Logs map[string]string `json:"logs,omitempty"`

	// UploadTesterLogs is true to auto-upload log files.
	UploadTesterLogs bool `json:"upload-tester-logs"`

	// EC2 defines ec2 instance configuration.
	EC2 *ec2config.Config `json:"ec2"`

	// ClusterSize is the number of nodes.
	ClusterSize int `json:"cluster-size"`
	// Cluster is the shared kubeadm configuration for initial cluster setup.
	// "DataDir" and "URLs" fields should not be set.
	// Will automatically be updated after EC2 creation.
	Cluster *Kubeadm `json:"cluster"`

	// TestTimeout is the test operation timeout.
	TestTimeout time.Duration `json:"test-timeout,omitempty"`
}

// Kubeadm defines kubeadm-specific configuration.
// TODO: support TLS
type Kubeadm struct {
	// Version is the kubeadm version.
	Version string `json:"version"`

	InitPodNetworkCIDR string `json:"init-pod-network-cidr,omitempty" kubeadm:"pod-network-cidr"`

	JoinTarget                   string
	JoinToken                    string `json:"join-token,omitempty" kubeadm:"token"`
	JoinDiscoveryTokenCACertHash string `json:"join-discovery-token-ca-cert-hash,omitempty" kubeadm:"discovery-token-ca-cert-hash"`
	JoinIgnorePreflightErrors    string `json:"join-ignore-preflight-errors,omitempty" kubeadm:"ignore-preflight-errors"`
}

var skipFlags = map[string]struct{}{
	"Version": {},
}

// ScriptInit returns the service file setup script.
func (ka *Kubeadm) ScriptInit() (s string, err error) {
	var fs []string
	fs, err = ka.FlagsInit()
	if err != nil {
		return "", err
	}
	return createScriptInit(scriptInit{
		Exec:  "/usr/bin/kubeadm",
		Flags: strings.Join(fs, " "),
	})
}

func createScriptInit(si scriptInit) (string, error) {
	tpl := template.Must(template.New("scriptInitTmpl").Parse(scriptInitTmpl))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, si); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type scriptInit struct {
	Exec  string
	Flags string
}

const scriptInitTmpl = `#!/usr/bin/env bash

printf "\n"
sudo kubeadm init {{ .Flags }} 1>>/var/log/kubeadm-init.log 2>&1

mkdir -p /home/ec2-user/.kube
sudo cp -i /etc/kubernetes/admin.conf /home/ec2-user/.kube/config
sudo chown $(id -u):$(id -g) /home/ec2-user/.kube/config
find /home/ec2-user/.kube/ 1>>/var/log/kubeadm-init.log 2>&1
`

// FlagsInit returns the list of "kubeadm init" flags.
// Make sure to validate the configuration with "ValidateAndSetDefaults".
func (ka *Kubeadm) FlagsInit() (flags []string, err error) {
	tp, vv := reflect.TypeOf(ka).Elem(), reflect.ValueOf(ka).Elem()
	for i := 0; i < tp.NumField(); i++ {
		k := tp.Field(i).Tag.Get("json")
		if k == "" {
			continue
		}
		k = strings.Replace(k, ",omitempty", "", -1)
		if ek := tp.Field(i).Tag.Get("kubeadm"); ek != "" {
			k = strings.Replace(ek, ",omitempty", "", -1)
		}

		fieldName := tp.Field(i).Name
		if _, ok := skipFlags[fieldName]; ok {
			continue
		}
		if !strings.HasPrefix(fieldName, "Init") {
			continue
		}

		switch vv.Field(i).Type().Kind() {
		case reflect.String:
			if vv.Field(i).String() != "" {
				flags = append(flags, fmt.Sprintf("--%s=%s", k, vv.Field(i).String()))
			}

		case reflect.Bool:
			flags = append(flags, fmt.Sprintf("--%s=%v", k, vv.Field(i).Bool()))

		default:
			return nil, fmt.Errorf("unknown %q", k)
		}
	}
	return flags, nil
}

// FlagsJoin returns the list of "kubeadm join" flags.
// Make sure to validate the configuration with "ValidateAndSetDefaults".
func (ka *Kubeadm) FlagsJoin() (flags []string, err error) {
	arg := ka.JoinTarget
	if arg == "" {
		return nil, errors.New("unknown 'kubeadm join' target")
	}
	tp, vv := reflect.TypeOf(ka).Elem(), reflect.ValueOf(ka).Elem()
	for i := 0; i < tp.NumField(); i++ {
		k := tp.Field(i).Tag.Get("json")
		if k == "" {
			continue
		}
		k = strings.Replace(k, ",omitempty", "", -1)
		if ek := tp.Field(i).Tag.Get("kubeadm"); ek != "" {
			k = strings.Replace(ek, ",omitempty", "", -1)
		}

		fieldName := tp.Field(i).Name
		if _, ok := skipFlags[fieldName]; ok {
			continue
		}
		if !strings.HasPrefix(fieldName, "Join") {
			continue
		}

		switch vv.Field(i).Type().Kind() {
		case reflect.String:
			flags = append(flags, fmt.Sprintf("--%s=%s", k, vv.Field(i).String()))

		case reflect.Bool:
			flags = append(flags, fmt.Sprintf("--%s=%v", k, vv.Field(i).Bool()))

		default:
			return nil, fmt.Errorf("unknown %q", k)
		}
	}
	return append([]string{arg}, flags...), nil
}

// ValidateAndSetDefaults returns an error for invalid configurations.
// And updates empty fields with default values.
// At the end, it writes populated YAML to aws-k8s-tester config path.
func (ka *Kubeadm) ValidateAndSetDefaults() (err error) {
	if ka.Version == "" {
		return errors.New("expected non-empty Version")
	}
	if strings.HasPrefix(ka.Version, "v") {
		ka.Version = ka.Version[1:]
	}
	return nil
}

// NewDefault returns a copy of the default configuration.
func NewDefault() *Config {
	vv := defaultConfig
	return &vv
}

func init() {
	kubeadmVer, err := semver.Make("1.10.9")
	if err != nil {
		panic(err)
	}
	defaultKubeadm.Version = kubeadmVer.String()

	defaultConfig.Cluster = &defaultKubeadm
	defaultConfig.Tag = genTag()
	defaultConfig.ClusterName = defaultConfig.Tag + "-" + randString(7)

	// package "internal/ec2" defaults
	// Amazon Linux 2 AMI (HVM), SSD Volume Type
	// ImageID:  "ami-061e7ebbc234015fe"
	// UserName: "ec2-user"
	defaultConfig.EC2.Plugins = []string{
		"update-amazon-linux-2",
		"install-start-docker-amazon-linux-2",
		"install-start-kubeadm-amazon-linux-2-" + defaultKubeadm.Version,
	}
	defaultConfig.EC2.ClusterSize = 3
	defaultConfig.EC2.Wait = true
	defaultConfig.EC2.Tag = defaultConfig.Tag
	defaultConfig.EC2.ClusterName = defaultConfig.ClusterName
	defaultConfig.EC2.IngressRulesTCP = map[string]string{
		"22":          "0.0.0.0/0",
		"6443":        "192.168.0.0/16",
		"2379-2380":   "192.168.0.0/16",
		"10250-10252": "192.168.0.0/16",
		"30000-32767": "192.168.0.0/16",
	}
}

// genTag generates a tag for cluster name, CloudFormation, and S3 bucket.
// Note that this would be used as S3 bucket name to upload tester logs.
func genTag() string {
	// use UTC time for everything
	now := time.Now().UTC()
	return fmt.Sprintf("awsk8stester-kubeadm-%d%02d%02d", now.Year(), now.Month(), now.Day())
}

var defaultConfig = Config{
	WaitBeforeDown: time.Minute,
	Down:           true,

	LogDebug: false,
	// default, stderr, stdout, or file name
	// log file named with cluster name will be added automatically
	LogOutputs:       []string{"stderr"},
	UploadTesterLogs: false,

	EC2: ec2config.NewDefault(),

	ClusterSize: 2,

	TestTimeout: 10 * time.Second,
}

var defaultKubeadm = Kubeadm{
	Version:                   "1.10.9",
	InitPodNetworkCIDR:        "10.244.0.0/16",
	JoinIgnorePreflightErrors: "cri",
}

// Load loads configuration from YAML.
// Useful when injecting shared configuration via ConfigMap.
//
// Example usage:
//
//  import "github.com/aws/aws-k8s-tester/kubeadmconfig"
//  cfg := kubeadmconfig.Load("test.yaml")
//  p, err := cfg.BackupConfig()
//  err = cfg.ValidateAndSetDefaults()
//
// Do not set default values in this function.
// "ValidateAndSetDefaults" must be called separately,
// to prevent overwriting previous data when loaded from disks.
func Load(p string) (cfg *Config, err error) {
	var d []byte
	d, err = ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}
	cfg = new(Config)
	if err = gyaml.Unmarshal(d, cfg); err != nil {
		return nil, err
	}

	cfg.ConfigPath, err = filepath.Abs(p)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// Sync persists current configuration and states to disk.
func (cfg *Config) Sync() (err error) {
	if !filepath.IsAbs(cfg.ConfigPath) {
		cfg.ConfigPath, err = filepath.Abs(cfg.ConfigPath)
		if err != nil {
			return err
		}
	}
	var d []byte
	d, err = gyaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(cfg.ConfigPath, d, 0600)
}

// BackupConfig stores the original aws-k8s-tester configuration
// file to backup, suffixed with ".backup.yaml".
// Otherwise, deployer will overwrite its state back to YAML.
// Useful when the original configuration would be reused
// for other tests.
func (cfg *Config) BackupConfig() (p string, err error) {
	var d []byte
	d, err = ioutil.ReadFile(cfg.ConfigPath)
	if err != nil {
		return "", err
	}
	p = fmt.Sprintf("%s.%X.backup.yaml",
		cfg.ConfigPath,
		time.Now().UTC().UnixNano(),
	)
	return p, ioutil.WriteFile(p, d, 0600)
}

const (
	envPfx        = "AWS_K8S_TESTER_KUBEADM_"
	envPfxCluster = "AWS_K8S_TESTER_KUBEADM_CLUSTER_"
)

// UpdateFromEnvs updates fields from environmental variables.
func (cfg *Config) UpdateFromEnvs() error {
	if err := cfg.EC2.UpdateFromEnvs(); err != nil {
		return err
	}

	cc := *cfg

	tp1, vv1 := reflect.TypeOf(&cc).Elem(), reflect.ValueOf(&cc).Elem()
	for i := 0; i < tp1.NumField(); i++ {
		jv := tp1.Field(i).Tag.Get("json")
		if jv == "" {
			continue
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.Replace(jv, "-", "_", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := envPfx + jv
		if os.Getenv(env) == "" {
			continue
		}
		sv := os.Getenv(env)

		switch vv1.Field(i).Type().Kind() {
		case reflect.String:
			vv1.Field(i).SetString(sv)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv1.Field(i).SetBool(bb)

		case reflect.Int, reflect.Int32, reflect.Int64:
			if tp1.Field(i).Name == "WaitBeforeDown" ||
				tp1.Field(i).Name == "TestTimeout" {
				dv, err := time.ParseDuration(sv)
				if err != nil {
					return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
				}
				vv1.Field(i).SetInt(int64(dv))
				continue
			}
			iv, err := strconv.ParseInt(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv1.Field(i).SetInt(iv)

		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			iv, err := strconv.ParseUint(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv1.Field(i).SetUint(iv)

		case reflect.Float32, reflect.Float64:
			fv, err := strconv.ParseFloat(sv, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv1.Field(i).SetFloat(fv)

		case reflect.Slice:
			ss := strings.Split(sv, ",")
			slice := reflect.MakeSlice(reflect.TypeOf([]string{}), len(ss), len(ss))
			for i := range ss {
				slice.Index(i).SetString(ss[i])
			}
			vv1.Field(i).Set(slice)

		default:
			return fmt.Errorf("%q (%v) is not supported as an env", env, vv1.Field(i).Type())
		}
	}
	*cfg = cc

	av := *cc.Cluster
	tp2, vv2 := reflect.TypeOf(&av).Elem(), reflect.ValueOf(&av).Elem()
	for i := 0; i < tp2.NumField(); i++ {
		jv := tp2.Field(i).Tag.Get("json")
		if jv == "" {
			continue
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := envPfxCluster + jv
		if os.Getenv(env) == "" {
			continue
		}
		sv := os.Getenv(env)

		switch vv2.Field(i).Type().Kind() {
		case reflect.String:
			vv2.Field(i).SetString(sv)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv2.Field(i).SetBool(bb)

		case reflect.Int, reflect.Int32, reflect.Int64:
			iv, err := strconv.ParseInt(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv2.Field(i).SetInt(iv)

		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			iv, err := strconv.ParseUint(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv2.Field(i).SetUint(iv)

		case reflect.Float32, reflect.Float64:
			fv, err := strconv.ParseFloat(sv, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv2.Field(i).SetFloat(fv)

		default:
			return fmt.Errorf("%q (%v) is not supported as an env", env, vv2.Field(i).Type())
		}
	}
	cfg.Cluster = &av

	return nil
}

var kubeadmPorts = []string{"22", "6443", "2379-2380", "10250-10252", "30000-32767"}

// ValidateAndSetDefaults returns an error for invalid configurations.
// And updates empty fields with default values.
// At the end, it writes populated YAML to aws-k8s-tester config path.
func (cfg *Config) ValidateAndSetDefaults() (err error) {
	if err = cfg.Cluster.ValidateAndSetDefaults(); err != nil {
		return err
	}
	if cfg.EC2 == nil {
		return errors.New("EC2 configuration not found")
	}
	cfg.EC2.ClusterSize = cfg.ClusterSize
	if err = cfg.EC2.ValidateAndSetDefaults(); err != nil {
		return err
	}
	for _, p := range kubeadmPorts {
		_, ok := cfg.EC2.IngressRulesTCP[p]
		if !ok {
			return fmt.Errorf("kubeadm expects port %q but not found from %v", p, cfg.EC2.IngressRulesTCP)
		}
	}

	okAMZLnx, okDocker, okKubeadm := false, false, false
	for i, v := range cfg.EC2.Plugins {
		if v == "update-amazon-linux-2" {
			okAMZLnx = true
			continue
		}
		if strings.HasPrefix(v, "install-start-docker-amazon-linux-2") {
			okDocker = true
			continue
		}
		if strings.HasPrefix(v, "install-start-kubeadm-amazon-linux-2-") {
			okKubeadm = true
			cfg.EC2.Plugins[i] = "install-start-kubeadm-amazon-linux-2-" + cfg.Cluster.Version
			continue
		}
	}
	if !okAMZLnx {
		return errors.New("EC2 Plugin 'update-amazon-linux-2' not found")
	}
	if !okDocker {
		return errors.New("EC2 Plugin 'install-start-docker-amazon-linux-2' not found")
	}
	if !okKubeadm {
		return errors.New("EC2 Plugin 'install-start-kubeadm-amazon-linux-2' not found")
	}

	if !cfg.EC2.Wait {
		return errors.New("Set EC2 Wait to true")
	}
	if cfg.EC2.UserName != "ec2-user" {
		return fmt.Errorf("expected 'ec2-user' user name, got %q", cfg.EC2.UserName)
	}

	if cfg.ClusterSize < 1 {
		return fmt.Errorf("ClusterSize expected at least 1, got %d", cfg.ClusterSize)
	}

	if cfg.Tag == "" {
		return errors.New("Tag is empty")
	}
	if cfg.ClusterName == "" {
		return errors.New("ClusterName is empty")
	}

	// populate all paths on disks and on remote storage
	if cfg.ConfigPath == "" {
		f, err := ioutil.TempFile(os.TempDir(), "awsk8stester-kubeadmconfig")
		if err != nil {
			return err
		}
		cfg.ConfigPath, _ = filepath.Abs(f.Name())
		f.Close()
		os.RemoveAll(cfg.ConfigPath)
	}
	cfg.ConfigPathBucket = filepath.Join(cfg.ClusterName, "awsk8stester-kubeadmconfig.yaml")

	cfg.LogOutputToUploadPath = filepath.Join(os.TempDir(), fmt.Sprintf("%s.log", cfg.ClusterName))
	logOutputExist := false
	for _, lv := range cfg.LogOutputs {
		if cfg.LogOutputToUploadPath == lv {
			logOutputExist = true
			break
		}
	}
	if !logOutputExist {
		// auto-insert generated log output paths to zap logger output list
		cfg.LogOutputs = append(cfg.LogOutputs, cfg.LogOutputToUploadPath)
	}
	cfg.LogOutputToUploadPathBucket = filepath.Join(cfg.ClusterName, "awsk8stester-kubeadm.log")

	return cfg.Sync()
}

const ll = "0123456789abcdefghijklmnopqrstuvwxyz"

func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		rand.Seed(time.Now().UTC().UnixNano())
		b[i] = ll[rand.Intn(len(ll))]
	}
	return string(b)
}
