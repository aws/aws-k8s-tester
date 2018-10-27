package config

import "time"

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
	AWSRegion: "us-west-2",

	WaitBeforeDown: 10 * time.Minute,
	Down:           true,

	LogDebug: false,

	// default, stderr, stdout, or file name
	// log file named with cluster name will be added automatically
	LogOutputs:       []string{"stderr"},
	UploadTesterLogs: false,

	OSDistribution: "ubuntu",
	UserName:       "ubuntu",

	// Ubuntu Server 16.04 LTS (HVM), SSD Volume Type
	ImageID: "ami-ba602bc2",
	Plugins: []string{
		"update-ubuntu",
		"install-go1.11.1",
	},

	// 4 vCPU, 15 GB RAM
	InstanceType: "m3.xlarge",
	Count:        1,

	AssociatePublicIPAddress: true,
}
