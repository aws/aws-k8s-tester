package clusterloader2

import (
	//"encoding/json"
	"fmt"
	"github.com/aws/aws-k8s-tester/eksconfig"
	k8sclient "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	gotemplate "github.com/aws/aws-k8s-tester/pkg/util"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io/ioutil"
	"net/http"
	"path"
	"strings"
)

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
	}{
		ClusterLoaderSpec: c.Config.Spec.ClusterLoader,
		ConfigMapData:     configMapData,
		TestArgs:          c.buildArgs(),
	})
	if err != nil {
		return fmt.Errorf("while building templates, %v", err)
	}
	// Get Job info from goTemplate. Currently hardcoded.
	resources := strings.Split(template.String(), "\n---\n")
	jobYaml := resources[len(resources)-1]
	if err := c.K8sClient.Delete(jobYaml); err != nil {
		//Warn that there was an error, but don't stop process.

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
		bodyString := strings.ReplaceAll(string(body), "\n", "\n    ")
		dataMap[path.Base(uri)] = bodyString
	}
	return dataMap, nil
}

func (c *ClusterLoader) buildArgs() string {
	return fmt.Sprintf("[%s]", strings.Join(c.Config.Spec.ClusterLoader.TestParams, ","))
}
