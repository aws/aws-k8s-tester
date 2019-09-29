package pkg

type TestConfig struct {
	Cluster         *Cluster `yaml:"cluster"`
	Region          string   `yaml:"region"`
	BuildScript     string   `yaml:"build"`
	InstallScript   string   `yaml:"install"`
	UninstallScript string   `yaml:"uninstall"`
	TestScript      string   `yaml:"test"`
}

type Cluster struct {
	Kops *KopsCluster `yaml:"kops"`
	Eks  *EksCluster  `yaml:"eks"`
}

type KopsCluster struct {
	StateFile         string `yaml:"stateFile"`
	Region            string `yaml:"region"`
	Zones             string `yaml:"zones"`
	NodeCount         int    `yaml:"nodeCount"`
	NodeSize          string `yaml:"nodeSize"`
	KubernetesVersion string `yaml:"kubernetesVersion"`
	FeatureGates      string `yaml:"featureGates"`
	IamPolicies       string `yaml:"iamPolicies"`
}

type EksCluster struct {
	Region            string `yaml:"region"`
	NodeCount         int    `yaml:"nodeCount"`
	NodeSize          string `yaml:"nodeSize"`
	KubernetesVersion string `yaml:"kubernetesVersion"`
}
