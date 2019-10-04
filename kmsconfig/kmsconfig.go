// Package kmsconfig configures KMS tests.
package kmsconfig

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/awsapi"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"sigs.k8s.io/yaml"
)

// Config defines EKS test configuration.
type Config struct {
	// ID is the ID used for KMS resource creation.
	ID string `json:"id,omitempty"`
	// ConfigPath is the configuration file path.
	// Deployer is expected to update this file with latest status.
	ConfigPath string `json:"config-path,omitempty"`

	// AWSAccountID is the AWS account ID.
	AWSAccountID string `json:"aws-account-id,omitempty"`
	// AWSRegion is the AWS geographic area for EKS deployment.
	// If empty, set default region.
	AWSRegion string `json:"aws-region,omitempty"`

	// LogLevel configures log level. Only supports debug, info, warn, error, panic, or fatal. Default 'info'.
	LogLevel string `json:"log-level"`
	// LogOutputs is a list of log outputs. Valid values are 'default', 'stderr', 'stdout', or file names.
	// Logs are appended to the existing file, if any.
	// Multiple values are accepted. If empty, it sets to 'default', which outputs to stderr.
	// See https://godoc.org/go.uber.org/zap#Open and https://godoc.org/go.uber.org/zap#Config for more details.
	LogOutputs []string `json:"log-outputs,omitempty"`

	// UpdatedAt is the timestamp when the configuration has been updated.
	// Read only to 'Config' struct users.
	UpdatedAt time.Time `json:"updated-at,omitempty"` // read-only to user
	// KeyMetadata is the EKS metadata status.
	KeyMetadata *KeyMetadata `json:"key-meta-data,omitempty"`
	// KeyRotationEnabled is true, if key rotation is enabled.
	KeyRotationEnabled bool `json:"key-rotation-enabled,omitempty"`
}

// KeyMetadata is the key's current metadata.
type KeyMetadata struct {
	AWSAccountID string    `json:"aws-account-id"`
	ARN          string    `json:"arn"`
	CreationDate time.Time `json:"creation-date"`
	Description  string    `json:"description"`
	Enabled      bool      `json:"enabled"`
	KeyID        string    `json:"key-id"`
	KeyManager   string    `json:"key-manager"`
	KeyState     string    `json:"key-state"`
	KeyUsage     string    `json:"key-usage"`
	Origin       string    `json:"origin"`
}

// NewDefault returns a copy of the default configuration.
func NewDefault() *Config {
	vv := defaultConfig
	return &vv
}

// defaultConfig is the default configuration.
//  - empty string creates a non-nil object for pointer-type field
//  - omitting an entire field returns nil value
//  - make sure to check both
var defaultConfig = Config{
	AWSAccountID: "",
	AWSRegion:    "us-west-2",

	LogLevel: logutil.DefaultLogLevel,
	// default, stderr, stdout, or file name
	// log file named with cluster name will be added automatically
	LogOutputs: []string{"stderr"},

	KeyMetadata: &KeyMetadata{},
}

// Load loads configuration from YAML.
// Useful when injecting shared configuration via ConfigMap.
//
// Example usage:
//
//  import "github.com/aws/aws-k8s-tester/kmsconfig"
//  cfg := kmsconfig.Load("test.yaml")
//  err := cfg.ValidateAndSetDefaults()
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
	if err = yaml.Unmarshal(d, cfg); err != nil {
		return nil, err
	}

	if cfg.KeyMetadata == nil {
		cfg.KeyMetadata = &KeyMetadata{}
	}

	if cfg.ConfigPath != p {
		cfg.ConfigPath = p
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

	cfg.UpdatedAt = time.Now().UTC()
	var d []byte
	d, err = yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(cfg.ConfigPath, d, 0600)
}

// genTag generates a tag for cluster name, CloudFormation, and S3 bucket.
// Note that this would be used as S3 bucket name to upload tester logs.
func genTag() string {
	// use UTC time for everything
	now := time.Now().UTC()
	return fmt.Sprintf("kms-%d%02d%02d%02d", now.Year(), int(now.Month()), now.Day(), now.Hour())
}

// ValidateAndSetDefaults returns an error for invalid configurations.
// And updates empty fields with default values.
// At the end, it writes populated YAML to aws-k8s-tester config path.
func (cfg *Config) ValidateAndSetDefaults() error {
	if len(cfg.LogOutputs) == 0 {
		return errors.New("LogOutputs is not empty")
	}
	if cfg.AWSRegion == "" {
		return errors.New("AWSRegion is empty")
	}
	if _, ok := awsapi.RegionToAiport[cfg.AWSRegion]; !ok {
		return fmt.Errorf("%q not found", cfg.AWSRegion)
	}
	if cfg.ID == "" {
		region := cfg.AWSRegion
		airport := awsapi.RegionToAiport[region]
		cfg.ID = genTag() + "-" + strings.ToLower(airport) + "-" + region + "-" + randString(5)
	}

	if cfg.KeyMetadata == nil {
		cfg.KeyMetadata = &KeyMetadata{}
	}

	if cfg.ConfigPath == "" {
		f, err := ioutil.TempFile(os.TempDir(), "kms")
		if err != nil {
			return err
		}
		cfg.ConfigPath, _ = filepath.Abs(f.Name())
		f.Close()
		os.RemoveAll(cfg.ConfigPath)
	}

	return cfg.Sync()
}

const envPfx = "AWS_K8S_TESTER_KMS_"

// UpdateFromEnvs updates fields from environmental variables.
func (cfg *Config) UpdateFromEnvs() error {
	cc := *cfg

	tp, vv := reflect.TypeOf(&cc).Elem(), reflect.ValueOf(&cc).Elem()
	for i := 0; i < tp.NumField(); i++ {
		jv := tp.Field(i).Tag.Get("json")
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
		fieldName := tp.Field(i).Name

		switch vv.Field(i).Type().Kind() {
		case reflect.String:
			vv.Field(i).SetString(sv)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv.Field(i).SetBool(bb)

		case reflect.Int, reflect.Int32, reflect.Int64:
			if fieldName == "DestroyWaitTime" {
				dv, err := time.ParseDuration(sv)
				if err != nil {
					return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
				}
				vv.Field(i).SetInt(int64(dv))
				continue
			}
			iv, err := strconv.ParseInt(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv.Field(i).SetInt(iv)

		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			iv, err := strconv.ParseUint(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv.Field(i).SetUint(iv)

		case reflect.Float32, reflect.Float64:
			fv, err := strconv.ParseFloat(sv, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv.Field(i).SetFloat(fv)

		case reflect.Slice:
			ss := strings.Split(sv, ",")
			slice := reflect.MakeSlice(reflect.TypeOf([]string{}), len(ss), len(ss))
			for i := range ss {
				slice.Index(i).SetString(ss[i])
			}
			vv.Field(i).Set(slice)

		default:
			return fmt.Errorf("%q (%v) is not supported as an env", env, vv.Field(i).Type())
		}
	}
	*cfg = cc

	return nil
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
