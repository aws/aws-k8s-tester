// package secureCN installs securecn tester.
package secureCN

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"reflect"
	"strings"
	"time"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/go-openapi/strfmt"

	api_client "github.com/Portshift/escher-client/client"
	"github.com/Portshift/escher-client/escher_api/model"
	"github.com/Portshift/escher-client/utils"
	"github.com/aws/aws-k8s-tester/client"
	k8s_tester "github.com/aws/aws-k8s-tester/k8s-tester/tester"
)

type Config struct {
	Enable bool `json:"enable"`

	Stopc     chan struct{} `json:"-"`
	Logger    *zap.Logger   `json:"-"`
	LogWriter io.Writer     `json:"-"`
	Client    client.Client `json:"-"`

	// Access key of the SecureCN service user in order to authenticate to the management
	AccessKey string `json:"access_key"`
	// Secret key of the SecureCN service user in order to authenticate to the management
	SecretKey string `json:"secret_key"`
	// URL of the SecureCN management
	URL string `json:"url"`
	// The name of the cluster to be created
	ClusterName string `json:"cluster_name"`
}

const (
	SecureCNBundleYaml         = "securecn_bundle.yml"
	SecureCNInstallBundle      = "install_bundle.sh"
	SecureCNBundleTarGz        = "securecn_bundle.tar.gz"
	SecureCNIstioAGBundleName  = "istio_init_bundle_ag.yml"
	SecureCNAGBundleName       = "securecn_bundle_ag.yml"
	SecureCNDoneIstioSeparator = "####### DONE ISTIO INIT PART  ########"
	SecureCNUninstallScript    = "uninstall.sh"
)

func NewDefault() *Config {
	return &Config{
		Enable: false,
		URL:    "https://securecn.cisco.com",
	}
}

func New(cfg *Config) k8s_tester.Tester {
	return &tester{
		cfg: cfg,
	}
}

type tester struct {
	cfg *Config
}

var pkgName = path.Base(reflect.TypeOf(tester{}).PkgPath())

func Env() string {
	return "ADD_ON_" + strings.ToUpper(strings.Replace(pkgName, "-", "_", -1))
}

func (ts *tester) Name() string { return pkgName }

func (ts *tester) Enabled() bool { return ts.cfg.Enable }

func (ts *tester) Apply() error {
	trueBool := true
	falseBool := false
	orchType := "EKS"

	defer cleanInstallationFiles()
	// create cluster
	cmh := api_client.NewHttpClient(ts.cfg.AccessKey, ts.cfg.SecretKey, ts.cfg.URL)
	res, err := cmh.EscherClient.CreateKubernetesCluster(context.TODO(), cmh.HttpClient, &model.KubernetesCluster{
		AgentFailClose:                    &falseBool,
		APIIntelligenceDAST:               &falseBool,
		AutoLabelEnabled:                  &trueBool,
		CiImageValidation:                 &trueBool,
		ClusterPodDefinitionSource:        model.ClusterPodDefinitionSourceKUBERNETES,
		EnableConnectionsControl:          &trueBool,
		EnableVenafiIntegration:           &falseBool,
		IsHoldApplicationUntilProxyStarts: &falseBool,
		IsIstioIngressEnabled:             &falseBool,
		IsMultiCluster:                    &falseBool,
		IsPersistent:                      &falseBool,
		Name:                              &ts.cfg.ClusterName,
		OrchestrationType:                 &orchType,
		PreserveOriginalSourceIP:          &falseBool,
	})
	if err != nil {
		return fmt.Errorf("failed to create kubernetes cluster. %v", err)
	}

	if err := downloadAndExtractBundle(cmh, res.Payload.ID); err != nil {
		return fmt.Errorf("failed to download and extract bundle. %v", err)
	}

	bundle, err := os.ReadFile(SecureCNBundleYaml)
	if err != nil {
		return fmt.Errorf("failed to read file %v. %v", SecureCNBundleYaml, err)
	}
	// split bundle
	bundleStr := string(bundle)
	bundleParts := strings.Split(bundleStr, SecureCNDoneIstioSeparator)
	// install Istio CRDs
	ts.cfg.Logger.Info("Installing istio CRDs")
	err = os.WriteFile(SecureCNIstioAGBundleName, []byte(bundleParts[0]), 0644)
	if err != nil {
		return fmt.Errorf("failed to write istio file: %v. %v", SecureCNIstioAGBundleName, err)
	}
	applyIstioArgs := []string{
		ts.cfg.Client.Config().KubectlPath,
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"apply",
		"-f",
		SecureCNIstioAGBundleName,
	}
	applyIstioCmd := strings.Join(applyIstioArgs, " ")
	out, err := exec.New().CommandContext(context.TODO(), applyIstioArgs[0], applyIstioArgs[1:]...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute command: %v. command output: %s. %v", applyIstioCmd, out, err)
	}

	// wait for istio CRDs to be ready
	time.Sleep(time.Minute * 1)

	// install securecn
	ts.cfg.Logger.Info("Installing securecn parts")
	err = os.WriteFile(SecureCNAGBundleName, []byte(bundleParts[1]), 0644)
	if err != nil {
		return fmt.Errorf("failed to write SecureCN file: %v. %v", SecureCNAGBundleName, err)
	}
	applySecureCNArgs := []string{
		ts.cfg.Client.Config().KubectlPath,
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"apply",
		"-f",
		SecureCNAGBundleName,
	}
	applySecureCNCmd := strings.Join(applySecureCNArgs, " ")
	out, err = exec.New().CommandContext(context.TODO(), applySecureCNArgs[0], applySecureCNArgs[1:]...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute command: %v. command output: %s. %v", applySecureCNCmd, out, err)
	}

	waitForAgentArgs := []string{
		ts.cfg.Client.Config().KubectlPath,
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"wait",
		"-n",
		"portshift",
		"pod",
		"-l",
		"app=portshift-agent",
		"--for",
		"condition=ready",
		"--timeout",
		"10m",
	}
	waitForAgentCmd := strings.Join(waitForAgentArgs, " ")
	out, err = exec.New().CommandContext(context.TODO(), waitForAgentArgs[0], waitForAgentArgs[1:]...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute command: %v. command output: %s. %v", waitForAgentCmd, out, err)
	}

	ts.cfg.Logger.Info("Successfully installed securecn controller")
	return nil
}

func cleanInstallationFiles() {
	_ = os.Remove(SecureCNAGBundleName)
	_ = os.Remove(SecureCNIstioAGBundleName)
	_ = os.Remove(SecureCNInstallBundle)
	_ = os.Remove(SecureCNBundleYaml)
}

func downloadAndExtractBundle(clientWrapper api_client.HttpClientWrapper, clusterID strfmt.UUID) error {
	f, err := os.Create(SecureCNBundleTarGz)
	if err != nil {
		return fmt.Errorf("failed to create file: %v. %v", SecureCNBundleTarGz, err)
	}
	buffer := new(bytes.Buffer)
	// download bundle
	err = clientWrapper.EscherClient.DownloadKubernetesSecureCNBundle(context.TODO(), clientWrapper.HttpClient, buffer, clusterID)
	_, err = io.Copy(f, buffer)
	if err != nil {
		return fmt.Errorf("failed to download bundle. %v", err)
	}

	open, err := os.Open(SecureCNBundleTarGz)
	if err != nil {
		return fmt.Errorf("failed to open file: %v. %v", SecureCNBundleTarGz, err)
	}
	defer os.Remove(SecureCNBundleTarGz)

	err = utils.ExtractTarGz(open)
	if err != nil {
		return fmt.Errorf("failed to extract tar gz. %v", err)
	}
	return nil
}

func (ts *tester) Delete() error {
	ts.cfg.Logger.Info("Uninstalling securecn")

	getUninstallerArgs := []string{
		ts.cfg.Client.Config().KubectlPath,
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"get",
		"cm",
		"-n",
		"portshift",
		"portshift-uninstaller",
		"-o",
		"jsonpath='{.data.config}'",
	}
	getUninstallerCmd := strings.Join(getUninstallerArgs, " ")
	out, err := exec.New().CommandContext(context.TODO(), getUninstallerArgs[0], getUninstallerArgs[1:]...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute command: %v. command output: %s. %v", getUninstallerCmd, out, err)
	}

	f, err := os.Create(SecureCNUninstallScript)
	if err != nil {
		return fmt.Errorf("failed to create file: %v. %v", SecureCNUninstallScript, err)
	}
	defer os.Remove(SecureCNUninstallScript)

	outStr := string(out)

	outStr = strings.TrimPrefix(outStr, "'")
	outStr = strings.TrimSuffix(outStr, "'")

	_, err = f.Write([]byte(outStr))

	if err := f.Chmod(0700); err != nil {
		return fmt.Errorf("failed to change file mode. %v", err)
	}
	out, err = exec.New().CommandContext(context.TODO(), "./"+SecureCNUninstallScript).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute uninstall command. command output: %s. %v", out, err)
	}
	ts.cfg.Logger.Info("Finished uninstalling securecn")

	return nil
}

func (cfg *Config) ValidateAndSetDefaults() error {

	return nil
}
