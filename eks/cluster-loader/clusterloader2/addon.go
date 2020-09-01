package clusterloader2

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	k8sclient "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	gotemplate "github.com/aws/aws-k8s-tester/pkg/util"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	v1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	}{
		ClusterLoaderSpec: c.Config.Spec.ClusterLoader,
		ConfigMapData:     configMapData,
		TestArgs:          c.buildArgs(),
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

	// Wait for job to complete -- 2 hours because larger tests take a very long time.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	job := &v1.Job{}
	for job.Status.Succeeded < 1 {
		job, err = c.K8sClient.KubernetesClientSet().
			BatchV1().
			Jobs("clusterloader2").
			Get(ctx, "clusterloader2", metav1.GetOptions{})
		if err != nil {
			cancel()
			return fmt.Errorf("failed to get cl2 job (%v)", err)
		}
		time.Sleep(10 * time.Second)
	}
	cancel()
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
	// add report-dir and testOverrides for CL2 tests. If one wants to change, check the gotemplate, and look at the job's initContainer's volumes
	arguments := append(c.Config.Spec.ClusterLoader.TestParams, "--report-dir=/var/reports/cluster-loader")
	if len(c.Config.Spec.ClusterLoader.TestOverrides) > 0 {
		arguments = append(arguments, "--testoverrides=/etc/config/overrides.yaml")
	}
	return fmt.Sprintf("[%s]", strings.Join(arguments, ","))
}
