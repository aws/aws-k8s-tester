// Package etcdconfig defines etcd test configuration.
package etcdconfig

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
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

// Config defines etcd test configuration.
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

	// Logs is a list of etcd node log file paths, fetched via SSH.
	Logs map[string]string `json:"logs,omitempty"`

	// UploadTesterLogs is true to auto-upload log files.
	UploadTesterLogs bool `json:"upload-tester-logs"`

	// EC2 defines ec2 instance configuration.
	// Ignored for local tests.
	EC2 *ec2config.Config `json:"ec2"`

	// ClusterSize is the number of etcd nodes.
	ClusterSize int `json:"cluster-size"`
	// Cluster is the shared etcd configuration for initial cluster setup.
	// "DataDir" and "URLs" fields should not be set.
	// Will automatically be updated after EC2 creation.
	Cluster *ETCD `json:"cluster"`
	// ClusterState maps ID to etcd instance.
	ClusterState map[string]ETCD `json:"cluster-state"`
}

// ETCD defines etcd-specific configuration.
// TODO: support TLS
type ETCD struct {
	// Version is the etcd version.
	Version  string `json:"version"`
	features map[string]bool
	// TopLevel is true if this is only used for top-level configuration.
	TopLevel bool `json:"top-level"`

	SSHPrivateKeyPath string `json:"ssh-private-key-path,omitempty"`
	PublicIP          string `json:"public-ip,omitempty"`
	PublicDNSName     string `json:"public-dns-name,omitempty"`

	Name                string `json:"name,omitempty"`
	DataDir             string `json:"data-dir,omitempty"`
	ListenClientURLs    string `json:"listen-client-urls,omitempty"`
	AdvertiseClientURLs string `json:"advertise-client-urls,omitempty"`
	ListenPeerURLs      string `json:"listen-peer-urls,omitempty"`
	AdvertisePeerURLs   string `json:"advertise-peer-urls,omitempty" etcd:"initial-advertise-peer-urls"`
	InitialCluster      string `json:"initial-cluster,omitempty"`
	InitialClusterState string `json:"initial-cluster-state,omitempty"`

	InitialClusterToken string `json:"initial-cluster-token"`
	SnapshotCount       int    `json:"snapshot-count"`
	HeartbeatMS         int    `json:"heartbeat-ms" etcd:"heartbeat-interval"`
	ElectionTimeoutMS   int    `json:"election-timeout-ms" etcd:"election-timeout"`
	QuotaBackendGB      int    `json:"quota-backend-gb" etcd:"quota-backend-bytes"`
	EnablePprof         bool   `json:"enable-pprof"`

	// flags for each version

	InitialElectionTickAdvance bool `json:"initial-election-tick-advance"`
}

var skipFlags = map[string]struct{}{
	"Version":           {},
	"TopLevel":          {},
	"SSHPrivateKeyPath": {},
	"PublicIP":          {},
	"PublicDNSName":     {},
}

var etcdVersions = map[string]map[uint64]map[string]bool{
	// master branch
	// https://github.com/etcd-io/etcd/blob/master/CHANGELOG-3.4.md
	"3.4": {
		0: {
			"initial-election-tick-advance": true,
		},
	},

	// https://github.com/etcd-io/etcd/blob/master/CHANGELOG-3.3.md
	"3.3": {
		10: {
			"initial-election-tick-advance": true,
		},
		9: {
			"initial-election-tick-advance": true,
		},
		8: {
			"initial-election-tick-advance": true,
		},
		7: {
			"initial-election-tick-advance": true,
		},
		6: {
			"initial-election-tick-advance": true,
		},
		5: {
			"initial-election-tick-advance": true,
		},
		4: {
			"initial-election-tick-advance": true,
		},
		3: {
			"initial-election-tick-advance": false,
		},
		2: {
			"initial-election-tick-advance": false,
		},
		1: {
			"initial-election-tick-advance": false,
		},
		0: {
			"initial-election-tick-advance": false,
		},
	},

	// https://github.com/etcd-io/etcd/blob/master/CHANGELOG-3.2.md
	"3.2": {
		25: {
			"initial-election-tick-advance": true,
		},
		24: {
			"initial-election-tick-advance": true,
		},
		23: {
			"initial-election-tick-advance": true,
		},
		22: {
			"initial-election-tick-advance": true,
		},
		21: {
			"initial-election-tick-advance": true,
		},
		20: {
			"initial-election-tick-advance": true,
		},
		19: {
			"initial-election-tick-advance": true,
		},
		18: {
			"initial-election-tick-advance": false,
		},
		17: {
			"initial-election-tick-advance": false,
		},
		16: {
			"initial-election-tick-advance": false,
		},
		15: {
			"initial-election-tick-advance": false,
		},
		14: {
			"initial-election-tick-advance": false,
		},
		13: {
			"initial-election-tick-advance": false,
		},
		12: {
			"initial-election-tick-advance": false,
		},
		11: {
			"initial-election-tick-advance": false,
		},
		10: {
			"initial-election-tick-advance": false,
		},
		9: {
			"initial-election-tick-advance": false,
		},
		8: {
			"initial-election-tick-advance": false,
		},
		7: {
			"initial-election-tick-advance": false,
		},
		6: {
			"initial-election-tick-advance": false,
		},
		5: {
			"initial-election-tick-advance": false,
		},
		4: {
			"initial-election-tick-advance": false,
		},
		3: {
			"initial-election-tick-advance": false,
		},
		2: {
			"initial-election-tick-advance": false,
		},
		1: {
			"initial-election-tick-advance": false,
		},
		0: {
			"initial-election-tick-advance": false,
		},
	},

	// https://github.com/etcd-io/etcd/blob/master/CHANGELOG-3.1.md
	"3.1": {
		20: {
			"initial-election-tick-advance": true,
		},
		19: {
			"initial-election-tick-advance": true,
		},
		18: {
			"initial-election-tick-advance": true,
		},
		17: {
			"initial-election-tick-advance": true,
		},
		16: {
			"initial-election-tick-advance": true,
		},
		15: {
			"initial-election-tick-advance": true,
		},
		14: {
			"initial-election-tick-advance": true,
		},
		13: {
			"initial-election-tick-advance": false,
		},
		12: {
			"initial-election-tick-advance": false,
		},
	},
}

// Flags returns the list of etcd flags.
// Make sure to validate the configuration with "ValidateAndSetDefaults".
func (e *ETCD) Flags() (flags []string, err error) {
	tp, vv := reflect.TypeOf(e).Elem(), reflect.ValueOf(e).Elem()
	for i := 0; i < tp.NumField(); i++ {
		k := tp.Field(i).Tag.Get("json")
		if k == "" {
			continue
		}
		k = strings.Replace(k, ",omitempty", "", -1)
		if ek := tp.Field(i).Tag.Get("etcd"); ek != "" {
			k = strings.Replace(ek, ",omitempty", "", -1)
		}
		if v, ok := e.features[k]; ok && !v {
			continue
		}

		fieldName := tp.Field(i).Name
		if _, ok := skipFlags[fieldName]; ok {
			continue
		}
		switch vv.Field(i).Type().Kind() {
		case reflect.String:
			flags = append(flags, fmt.Sprintf("--%s=%s", k, vv.Field(i).String()))

		case reflect.Bool:
			flags = append(flags, fmt.Sprintf("--%s=%v", k, vv.Field(i).Bool()))

		case reflect.Int, reflect.Int32, reflect.Int64:
			v := vv.Field(i).Int()
			if fieldName == "QuotaBackendGB" {
				// 2 * 1024 * 1024 * 1024 == 2147483648 == 2 GB
				v = v * 1024 * 1024 * 1024
			}
			flags = append(flags, fmt.Sprintf("--%s=%d", k, v))

		default:
			return nil, fmt.Errorf("unknown %q", k)
		}
	}
	return flags, nil
}

// Service returns the service file setup script.
func (e *ETCD) Service() (s string, err error) {
	var fs []string
	fs, err = e.Flags()
	if err != nil {
		return "", err
	}
	return createSvcInfo(svcInfo{
		Exec:  "/usr/local/bin/etcd",
		Flags: strings.Join(fs, " "),
	})
}

func createSvcInfo(svc svcInfo) (string, error) {
	tpl := template.Must(template.New("svcTmpl").Parse(svcTmpl))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, svc); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type svcInfo struct {
	Exec  string
	Flags string
}

const svcTmpl = `#!/usr/bin/env bash

# to write service file for etcd
rm -f /tmp/etcd.service

cat > /tmp/etcd.service <<EOF
[Unit]
Description=etcd
Documentation=https://github.com/etcd-io/etcd
Conflicts=etcd.service
Conflicts=etcd2.service

[Service]
Type=notify
Restart=always
RestartSec=5s
LimitNOFILE=40000
TimeoutStartSec=0

ExecStart={{ .Exec }} {{ .Flags }}

[Install]
WantedBy=multi-user.target
EOF

sudo mv /tmp/etcd.service /etc/systemd/system/etcd.service

# to start service
sudo systemctl daemon-reload
sudo systemctl cat etcd.service
sudo systemctl enable etcd.service
sudo systemctl start etcd.service

sleep 3s

# to get logs from service
sudo journalctl --no-pager --output=cat -u etcd.service
`

// ValidateAndSetDefaults returns an error for invalid configurations.
// And updates empty fields with default values.
// At the end, it writes populated YAML to aws-k8s-tester config path.
func (e *ETCD) ValidateAndSetDefaults() (err error) {
	if e.Version == "" {
		return errors.New("expected non-empty Version")
	}
	if strings.HasPrefix(e.Version, "v") {
		e.Version = e.Version[1:]
	}
	ver, err := semver.Make(e.Version)
	if err != nil {
		return err
	}
	if minEtcdVer.Compare(ver) > 0 {
		return fmt.Errorf("expected >= %s, got %s", minEtcdVer, ver)
	}

	majorMinor, patch := fmt.Sprintf("%d.%d", ver.Major, ver.Minor), ver.Patch
	ps, ok := etcdVersions[majorMinor]
	if !ok {
		return fmt.Errorf("unknown version %q", majorMinor)
	}
	e.features, ok = ps[patch]
	if !ok {
		return fmt.Errorf("unknown version %q", ver)
	}
	if e.InitialElectionTickAdvance {
		added, exist := e.features["initial-election-tick-advance"]
		if !exist || !added {
			return fmt.Errorf("InitialElectionTickAdvance invalid for %q", ver)
		}
	}

	if e.TopLevel {
		if e.Name != "" {
			return fmt.Errorf("unexpected Name %q with 'TopLevel'", e.Name)
		}
		if e.DataDir != "" {
			return fmt.Errorf("unexpected DataDir %q with 'TopLevel'", e.DataDir)
		}
		if e.ListenClientURLs != "" {
			return fmt.Errorf("unexpected ListenClientURLs %q with 'TopLevel'", e.ListenClientURLs)
		}
		if e.AdvertiseClientURLs != "" {
			return fmt.Errorf("unexpected AdvertiseClientURLs %q with 'TopLevel'", e.AdvertiseClientURLs)
		}
		if e.ListenPeerURLs != "" {
			return fmt.Errorf("unexpected ListenPeerURLs %q with 'TopLevel'", e.ListenPeerURLs)
		}
		if e.AdvertisePeerURLs != "" {
			return fmt.Errorf("unexpected AdvertisePeerURLs %q with 'TopLevel'", e.AdvertisePeerURLs)
		}
		if e.InitialCluster != "" {
			return fmt.Errorf("unexpected InitialCluster %q with 'TopLevel'", e.InitialCluster)
		}
		if e.InitialClusterState != "" {
			return fmt.Errorf("unexpected InitialClusterState %q with 'TopLevel'", e.InitialClusterState)
		}
	} else {
		if e.Name == "" {
			return errors.New("expected non-empty Name")
		}
		if e.DataDir == "" {
			return errors.New("expected non-empty DataDir")
		}
		if e.ListenClientURLs == "" {
			return errors.New("expected non-empty ListenClientURLs")
		}
		if e.AdvertiseClientURLs == "" {
			return errors.New("expected non-empty AdvertiseClientURLs")
		}
		if e.ListenPeerURLs == "" {
			return errors.New("expected non-empty ListenPeerURLs")
		}
		if e.AdvertisePeerURLs == "" {
			return errors.New("expected non-empty AdvertisePeerURLs")
		}
		if e.InitialCluster == "" {
			return errors.New("expected non-empty InitialCluster")
		}
		if e.InitialClusterState == "" {
			return errors.New("expected non-empty InitialClusterState")
		}
	}

	if e.InitialClusterToken == "" {
		return errors.New("got empty ETCD.InitialClusterToken")
	}
	if e.SnapshotCount == 0 {
		return errors.New("got zero ETCD.SnapshotCount")
	}
	if e.HeartbeatMS < 100 {
		return fmt.Errorf("expected ETCD.HeartbeatMS >= 100ms, got %dms", e.HeartbeatMS)
	}
	if e.ElectionTimeoutMS < 1000 {
		return fmt.Errorf("expected ETCD.ElectionTimeoutMS >= 1000ms, got %dms", e.ElectionTimeoutMS)
	}
	if e.QuotaBackendGB == 0 {
		return errors.New("got zero ETCD.QuotaBackendGB")
	}

	return nil
}

// NewDefault returns a copy of the default configuration.
func NewDefault() *Config {
	vv := defaultConfig
	return &vv
}

var (
	// minimum recommended etcd versions to run in production is 3.1.11+, 3.2.10+, and 3.3.0+
	// https://groups.google.com/forum/#!msg/etcd-dev/nZQl17RjxHQ/FkC_rZ_4AwAJT
	minEtcdVer semver.Version
)

func init() {
	var err error
	minEtcdVer, err = semver.Make("3.1.12")
	if err != nil {
		panic(err)
	}
	defaultETCD.Version = minEtcdVer.String()

	defaultConfig.Cluster = &defaultETCD
	defaultConfig.Tag = genTag()
	defaultConfig.ClusterName = defaultConfig.Tag + "-" + randString(7)

	// package "internal/ec2" defaults
	// Amazon Linux 2 AMI (HVM), SSD Volume Type
	// ImageID:  "ami-061e7ebbc234015fe"
	// UserName: "ec2-user"
	defaultConfig.EC2.Plugins = []string{
		"update-amazon-linux-2",
		"install-etcd-3.1.12",
	}

	defaultConfig.EC2.Wait = true
	defaultConfig.EC2.Tag = defaultConfig.Tag
	defaultConfig.EC2.ClusterName = defaultConfig.ClusterName
}

// genTag generates a tag for cluster name, CloudFormation, and S3 bucket.
// Note that this would be used as S3 bucket name to upload tester logs.
func genTag() string {
	// use UTC time for everything
	now := time.Now().UTC()
	return fmt.Sprintf("aws-k8s-tester-etcd-%d%02d%02d", now.Year(), now.Month(), now.Day())
}

var defaultConfig = Config{
	WaitBeforeDown: time.Minute,
	Down:           true,

	LogDebug: false,
	// default, stderr, stdout, or file name
	// log file named with cluster name will be added automatically
	LogOutputs:       []string{"stderr"},
	UploadTesterLogs: true,

	EC2:          ec2config.NewDefault(),
	ClusterSize:  1,
	ClusterState: make(map[string]ETCD),
}

var defaultETCD = ETCD{
	TopLevel: true,

	InitialClusterToken: "tkn",

	SnapshotCount: 10000,

	HeartbeatMS:       100,
	ElectionTimeoutMS: 1000,

	QuotaBackendGB: 2,
	EnablePprof:    true,
}

// Load loads configuration from YAML.
// Useful when injecting shared configuration via ConfigMap.
//
// Example usage:
//
//  import "github.com/aws/aws-k8s-tester/etcdconfig"
//  cfg := etcdconfig.Load("test.yaml")
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
	// cfg.UpdatedAt = time.Now().UTC()
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
	envPfx        = "AWS_K8S_TESTER_ETCD_"
	envPfxCluster = "AWS_K8S_TESTER_ETCD_TOP_ETCD_"
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
			if tp1.Field(i).Name == "WaitBeforeDown" {
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

// ValidateAndSetDefaults returns an error for invalid configurations.
// And updates empty fields with default values.
// At the end, it writes populated YAML to aws-k8s-tester config path.
func (cfg *Config) ValidateAndSetDefaults() (err error) {
	if cfg.EC2 == nil {
		return errors.New("EC2 configuration not found")
	}
	cfg.EC2.Count = cfg.ClusterSize
	if err = cfg.Cluster.ValidateAndSetDefaults(); err != nil {
		return err
	}
	if err = cfg.EC2.ValidateAndSetDefaults(); err != nil {
		return err
	}

	// expect
	// "update-amazon-linux-2"
	// "install-etcd-3.1.12"
	okAMZLnx, okEtcd := false, false
	for _, v := range cfg.EC2.Plugins {
		if v == "update-amazon-linux-2" {
			okAMZLnx = true
		}
		if strings.HasPrefix(v, "install-etcd-") {
			okEtcd = true
		}
	}
	if !okAMZLnx {
		return errors.New("EC2 Plugin 'update-amazon-linux-2' not found")
	}
	if !okEtcd {
		return errors.New("EC2 Plugin 'install-etcd' not found")
	}

	if !cfg.EC2.Wait {
		return errors.New("Set EC2 Wait to true")
	}
	if cfg.EC2.UserName != "ec2-user" {
		return fmt.Errorf("expected 'ec2-user' user name, got %q", cfg.EC2.UserName)
	}

	if cfg.ClusterSize < 1 || cfg.ClusterSize > 5 {
		return fmt.Errorf("ClusterSize expected between 1 and 5, got %d", cfg.ClusterSize)
	}

	if cfg.Tag == "" {
		return errors.New("Tag is empty")
	}
	if cfg.ClusterName == "" {
		return errors.New("ClusterName is empty")
	}

	// populate all paths on disks and on remote storage
	if cfg.ConfigPath == "" {
		f, err := ioutil.TempFile(os.TempDir(), "aws-k8s-tester-etcdconfig")
		if err != nil {
			return err
		}
		cfg.ConfigPath, _ = filepath.Abs(f.Name())
		f.Close()
		os.RemoveAll(cfg.ConfigPath)
		cfg.ConfigPathBucket = filepath.Join(cfg.ClusterName, "aws-k8s-tester-etcdconfig.yaml")
		if cfg.UploadTesterLogs {
			cfg.ConfigPathURL = genS3URL(cfg.EC2.AWSRegion, cfg.Tag, cfg.ConfigPathBucket)
		}
	}

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
	cfg.LogOutputToUploadPathBucket = filepath.Join(cfg.ClusterName, "aws-k8s-tester-etcd.log")
	if cfg.UploadTesterLogs {
		cfg.LogOutputToUploadPathURL = genS3URL(cfg.EC2.AWSRegion, cfg.Tag, cfg.LogOutputToUploadPathBucket)
	}

	if len(cfg.ClusterState) > 0 && cfg.ClusterSize != len(cfg.ClusterState) {
		return fmt.Errorf("ClusterSize %d != len(ClusterState) %d", cfg.ClusterSize, len(cfg.ClusterState))
	}
	for k, v := range cfg.ClusterState {
		if v.TopLevel {
			return fmt.Errorf("ClusterState has TopLevel set %q=%v", k, v)
		}
	}

	return cfg.Sync()
}

// genS3URL returns S3 URL path.
// e.g. https://s3-us-west-2.amazonaws.com/aws-k8s-tester-20180925/hello-world
func genS3URL(region, bucket, s3Path string) string {
	return fmt.Sprintf("https://s3-%s.amazonaws.com/%s/%s", region, bucket, s3Path)
}
