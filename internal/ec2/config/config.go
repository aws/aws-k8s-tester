package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/awstester/internal/ec2/config/plugins"
	ec2types "github.com/aws/awstester/pkg/awsapi/ec2"

	gyaml "github.com/ghodss/yaml"
)

// Config defines EC2 configuration.
type Config struct {
	// AWSAccountID is the AWS account ID.
	AWSAccountID string `json:"aws-account-id,omitempty"`
	// AWSRegion is the AWS region.
	AWSRegion string `json:"aws-region,omitempty"`

	// LogDebug is true to enable debug level logging.
	LogDebug bool `json:"log-debug"`

	// LogOutputs is a list of log outputs. Valid values are 'default', 'stderr', 'stdout', or file names.
	// Logs are appended to the existing file, if any.
	// Multiple values are accepted. If empty, it sets to 'default', which outputs to stderr.
	// See https://godoc.org/go.uber.org/zap#Open and https://godoc.org/go.uber.org/zap#Config for more details.
	LogOutputs []string `json:"log-outputs,omitempty"`
	// LogOutputToUploadPath is the awstester log file path to upload to cloud storage.
	// Must be left empty.
	// This will be overwritten by cluster name.
	LogOutputToUploadPath       string `json:"log-output-to-upload-path,omitempty"`
	LogOutputToUploadPathBucket string `json:"log-output-to-upload-path-bucket,omitempty"`
	LogOutputToUploadPathURL    string `json:"log-output-to-upload-path-url,omitempty"`
	// LogAutoUpload is true to auto-upload log files.
	LogAutoUpload bool `json:"log-auto-upload"`

	// Tag is the tag used for all cloudformation stacks.
	// Must be left empty, and let deployer auto-populate this field.
	Tag string `json:"tag,omitempty"` // read-only to user
	// ID is an unique ID for this configuration.
	// Meant to be auto-generated.
	// Used for debugging purposes only.
	ID string `json:"id,omitempty"` // read-only to user

	// WaitBeforeDown is the duration to sleep before EC2 tear down.
	// This is for "test".
	WaitBeforeDown time.Duration `json:"wait-before-down,omitempty"`
	// Down is true to automatically tear down EC2 in "test".
	// Note that this is meant to be used as a flag in "test".
	// Deployer implementation should not call "Down" inside "Up" method.
	Down bool `json:"down"`

	// ConfigPath is the configuration file path.
	// If empty, it is autopopulated.
	// Deployer is expected to update this file with latest status,
	// and to make a backup of original configuration
	// with the filename suffix ".backup.yaml" in the same directory.
	ConfigPath       string    `json:"config-path,omitempty"`
	ConfigPathBucket string    `json:"config-path-bucket,omitempty"` // read-only to user
	ConfigPathURL    string    `json:"config-path-url,omitempty"`    // read-only to user
	UpdatedAt        time.Time `json:"updated-at,omitempty"`         // read-only to user

	// OSDistribution is either ubuntu or Amazon Linux 2 for now.
	OSDistribution string `json:"os-distribution,omitempty"`
	// UserName is the user name used for running init scripts or SSH access.
	UserName string `json:"user-name,omitempty"`
	// ImageID is the Amazon Machine Image (AMI).
	ImageID string `json:"image-id,omitempty"`
	// Plugins is the list of plugins.
	Plugins []string `json:"plugins,omitempty"`
	// InitScript contains init scripts (run-instance UserData field).
	// Script must be started with "#!/usr/bin/env bash".
	// And will be base64-encoded. Do not base64-encode.
	// Let "ec2" package base64-encode.
	// Outputs are saved in "/var/log/cloud-init-output.log" in EC2 instance.
	// "tail -f /var/log/cloud-init-output.log" to check the progress.
	// Reference: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/user-data.html.
	// Note that if both "Plugins" and "InitScript" are not empty,
	// "InitScript" field is always overwritten by "Plugins" field.
	InitScript string `json:"init-script,omitempty"`

	// InstanceType is the instance type.
	InstanceType string `json:"instance-type,omitempty"`
	// Count is the number of EC2 instances to create.
	Count int `json:"count,omitempty"`

	// KeyName is the name of the key pair used for SSH access.
	// Leave empty to create a temporary one.
	KeyName string `json:"key-name,omitempty"`
	// KeyPath is the file path to the private key.
	KeyPath       string `json:"key-path,omitempty"`
	KeyPathBucket string `json:"key-path-bucket,omitempty"`
	KeyPathURL    string `json:"key-path-url,omitempty"`

	// VPCID is the VPC ID to use.
	// Leave empty to create a temporary one.
	VPCID      string `json:"vpc-id"`
	VPCCreated bool   `json:"vpc-created"`
	// InternetGatewayID is the internet gateway ID.
	InternetGatewayID string `json:"internet-gateway-id,omitempty"`
	// RouteTableIDs is the list of route table IDs.
	RouteTableIDs []string `json:"route-table-ids,omitempty"`

	// SubnetIDs is a list of subnet IDs to use.
	// Leave empty, read-only to user.
	// It will fetch subnets from a given or created VPC.
	// And randomly assign them to instances.
	SubnetIDs                  []string          `json:"subnet-ids,omitempty"`
	SubnetIDToAvailibilityZone map[string]string `json:"subnet-id-to-availability-zone,omitempty"` // read-only to user

	// SecurityGroupIDs is the list of security group IDs.
	// Leave empty to create a temporary one.
	SecurityGroupIDs []string `json:"security-group-ids,omitempty"`

	// AssociatePublicIPAddress is true to associate a public IP address.
	AssociatePublicIPAddress bool `json:"associate-public-ip-address"`

	// Instances is a set of EC2 instances created from this configuration.
	Instances            []Instance          `json:"instances,omitempty"`
	InstanceIDToInstance map[string]Instance `json:"instance-id-to-instance,omitempty"`
}

// ValidateAndSetDefaults returns an error for invalid configurations.
// And updates empty fields with default values.
// At the end, it writes populated YAML to awstester config path.
func (cfg *Config) ValidateAndSetDefaults() (err error) {
	if len(cfg.LogOutputs) == 0 {
		return errors.New("EKS LogOutputs is not specified")
	}
	if cfg.AWSRegion == "" {
		return errors.New("empty AWSRegion")
	}
	if cfg.OSDistribution == "" {
		return errors.New("empty OSDistribution")
	}
	if cfg.UserName == "" {
		return errors.New("empty UserName")
	}
	if cfg.ImageID == "" {
		return errors.New("empty ImageID")
	}

	if len(cfg.Plugins) > 0 {
		cfg.InitScript, err = plugins.Get(cfg.Plugins...)
		if err != nil {
			return err
		}
	}

	if cfg.InstanceType == "" {
		return errors.New("empty InstanceType")
	}
	if cfg.Count < 1 {
		return errors.New("wrong Count")
	}

	if cfg.ID == "" {
		cfg.Tag = genTag()
		cfg.ID = genID()
	}

	if cfg.ConfigPath == "" {
		var f *os.File
		f, err = ioutil.TempFile(os.TempDir(), "awstester-ec2-config")
		if err != nil {
			return err
		}
		cfg.ConfigPath, _ = filepath.Abs(f.Name())
		f.Close()
		os.RemoveAll(cfg.ConfigPath)
		cfg.ConfigPathBucket = filepath.Join(cfg.ID, "awstester-ec2.config.yaml")
		cfg.ConfigPathURL = genS3URL(cfg.AWSRegion, cfg.Tag, cfg.ConfigPathBucket)
	}

	cfg.LogOutputToUploadPath = filepath.Join(os.TempDir(), fmt.Sprintf("%s.log", cfg.ID))
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
	cfg.LogOutputToUploadPathBucket = filepath.Join(cfg.ID, "awstester-ec2.log")
	cfg.LogOutputToUploadPathURL = genS3URL(cfg.AWSRegion, cfg.Tag, cfg.LogOutputToUploadPathBucket)

	if cfg.KeyName == "" {
		cfg.KeyName = cfg.ID
		var f *os.File
		f, err = ioutil.TempFile(os.TempDir(), "awstester-ec2.key")
		if err != nil {
			return err
		}
		cfg.KeyPath, _ = filepath.Abs(f.Name())
		f.Close()
		os.RemoveAll(cfg.KeyPath)
		cfg.KeyPathBucket = filepath.Join(cfg.ID, "awstester-ec2.key")
		cfg.KeyPathURL = genS3URL(cfg.AWSRegion, cfg.Tag, cfg.KeyPathBucket)
	}

	if _, ok := ec2types.InstanceTypes[cfg.InstanceType]; !ok {
		return fmt.Errorf("unexpected InstanceType %q", cfg.InstanceType)
	}

	return nil
}

// Load loads configuration from YAML.
//
// Example usage:
//
//	import "github.com/aws/awstester/internal/ec2/config"
//	cfg := config.Load("test.yaml")
//  p, err := cfg.BackupConfig()
//	err = cfg.ValidateAndSetDefaults()
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

	if cfg.Instances == nil {
		cfg.Instances = make([]Instance, 0)
	}

	if !filepath.IsAbs(cfg.ConfigPath) {
		cfg.ConfigPath, err = filepath.Abs(p)
		if err != nil {
			return nil, err
		}
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
	d, err = gyaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(cfg.ConfigPath, d, 0600)
}

// BackupConfig stores the original awstester configuration
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
