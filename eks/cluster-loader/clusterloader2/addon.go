package clusterloader2

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"strings"

	"github.com/aws/aws-k8s-tester/eksconfig"
	k8sclient "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	gotemplate "github.com/aws/aws-k8s-tester/pkg/util"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// IndentedNewline covers formatting issues with gotemplates
const IndentedNewline = "\n    "

// ClusterLoader struct is currently only needed in this directory. May change with future additions.
type ClusterLoader struct {
	Config    *eksconfig.Config
	K8sClient k8sclient.EKS
}

// IsEnabled returns true if enabled
func (c *ClusterLoader) IsEnabled() bool {
	return c.Config.Spec.ClusterLoader != nil
}

// Apply installs the addon
func (c *ClusterLoader) Apply() (err error) {
	configMapData, err := c.buildConfigMapData()
	if err != nil {
		return fmt.Errorf("while forming configmap: %v", err)
	}
	template, err := gotemplate.FromLocalDirectory(struct {
		*eksconfig.ClusterLoaderSpec
		ConfigMapData map[string]string
		TestArgs      string
		Affinities    map[string][]string
	}{
		ClusterLoaderSpec: c.Config.Spec.ClusterLoader,
		ConfigMapData:     configMapData,
		TestArgs:          c.buildArgs(),
		Affinities:        c.Config.Spec.ClusterLoader.Affinities,
	})
	if err != nil {
		return fmt.Errorf("while building templates, %v", err)
	}
	// Get CL2 Job info from goTemplate
	resources := strings.Split(template.String(), "\n---\n")
	jobYaml := resources[len(resources)-1]
	if err := c.K8sClient.Delete(jobYaml); err != nil {
		// Warn that there was an error, but don't stop process.
		zap.S().Warn("Deleting jobYaml errored out, but is fine: %s", zapcore.Field{String: err.Error()})
	}
	if err := c.K8sClient.Apply(template.String()); err != nil {
		return fmt.Errorf("while applying resources, %v", err)
	}
	c.Config.Status.ClusterLoader = &eksconfig.ClusterLoaderStatus{
		AddonStatus: eksconfig.AddonStatus{
			Installed: true,
			Ready:     true,
		},
	}
	return nil
}

// Delete removes the addon
func (c *ClusterLoader) Delete() (err error) {
	template, err := gotemplate.FromLocalDirectory(c.Config.Spec.ClusterLoader)
	if err != nil {
		return fmt.Errorf("while building templates, %v", err)
	}
	if err := c.K8sClient.Delete(template.String()); err != nil {
		return fmt.Errorf("while deleting resources, %v", err)
	}
	c.Config.Status.ClusterLoader = &eksconfig.ClusterLoaderStatus{
		AddonStatus: eksconfig.AddonStatus{
			Installed: false,
			Ready:     false,
		},
	}
	return nil
}

func (c *ClusterLoader) buildConfigMapData() (map[string]string, error) {
	// Any file in this map is at /etc/config in the mounted volume of CL2
	dataMap := make(map[string]string)
	for _, uri := range c.Config.Spec.ClusterLoader.TestConfigUris {
		resp, err := http.Get(uri)
		if err != nil {
			return nil, fmt.Errorf("while downloading uri %s: %v", uri, err)
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("while reading downloaded uri %s content: %v", uri, err)
		}
		// Indents the content by four spaces for valid YAML formatting
		bodyString := strings.ReplaceAll(string(body), "\n", IndentedNewline)
		dataMap[path.Base(uri)] = bodyString
	}
	overrides := c.Config.Spec.ClusterLoader.TestOverrides
	if len(overrides) > 0 {
		dataMap["overrides.yaml"] = strings.Join(overrides, IndentedNewline)
	}
	return dataMap, nil
}

func (c *ClusterLoader) buildArgs() string {
	arguments := []string{}

	// Standard arguments for clusterloader2
	for _, argument := range []string{
		fmt.Sprintf("--nodes=%d", c.Config.Spec.ClusterLoader.Nodes),
		"--provider=eks",
		"--testconfig=/etc/config/config.yaml",
		"--run-from-cluster",
		"--alsologtostderr",
	} {
		arguments = append(arguments, argument)
	}

	// Additional optional arguments
	if len(c.Config.Spec.ClusterLoader.TestOverrides) > 0 {
		arguments = append(arguments, "--testoverrides=/etc/config/overrides.yaml")
	}
	return fmt.Sprintf("[%s]", strings.Join(arguments, ","))
}
