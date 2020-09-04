package eksconfig

import "strings"

// ClusterLoaderSpec defines the spec for the Addon
type ClusterLoaderSpec struct {
	Image          string   `json:"image,omitempty"`
	TestConfigUris []string `json:"testConfigUris,omitempty"`
	TestParams     []string `json:"testParams,omitempty"`
	S3Uri          string   `json:"s3Uri,omitempty"`
	TestOverrides  []string `json:"testOverrides,omitempty"`
}

// MetricsServerStatus defines the status for the Addon
type ClusterLoaderStatus struct {
	AddonStatus `json:",inline"`
}

// Validate installs the addon
func (spec *ClusterLoaderSpec) Validate(cfg *Config) error {
	return nil
}

// Default installs the addon
func (spec *ClusterLoaderSpec) Default(cfg *Config) {
	if spec.Image == "" {
		spec.Image = "197575167141.dkr.ecr.us-west-2.amazonaws.com/clusterloader2:latest"
	}
	if len(spec.TestConfigUris) == 0 {
		spec.TestConfigUris = []string{
			"https://raw.githubusercontent.com/aws/aws-k8s-tester/master/eks/cluster-loader/configs/cluster-autoscaler/config.yaml",
			"https://raw.githubusercontent.com/aws/aws-k8s-tester/master/eks/cluster-loader/configs/cluster-autoscaler/deployment.yaml",
		}
	}
	set := make(map[string]bool)
	for _, param := range spec.TestParams {
		tempSplit := strings.Split(param, "=")
		if len(tempSplit) > 1 {
			set[tempSplit[0]] = true
		}
		set[param] = true
	}
	if ok := set["--run-from-cluster"]; !ok {
		spec.TestParams = append(spec.TestParams, "--run-from-cluster")
	}
	if ok := set["--testconfig"]; !ok {
		spec.TestParams = append(spec.TestParams, "--testconfig=/etc/config/config.yaml")
	}
	if ok := set["--provider"]; !ok {
		spec.TestParams = append(spec.TestParams, "--provider=eks")
	}
	if ok := set["--nodes"]; !ok {
		spec.TestParams = append(spec.TestParams, "--nodes=10")
	}
	if ok := set["--alsologtostderr"]; !ok {
		spec.TestParams = append(spec.TestParams, "--alsologtostderr")
	}

}
