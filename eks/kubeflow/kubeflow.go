// Package kubeflow implements kubeflow add-on.
// ref. https://www.kubeflow.org/docs/aws/deploy/install-kubeflow/
package kubeflow

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/httputil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/mholt/archiver/v3"
	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

// Config defines Kubeflow configuration.
// ref. https://www.kubeflow.org/docs/aws/deploy/install-kubeflow/
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}

	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

// Tester defines Kubeflow tester.
// ref. https://www.kubeflow.org/docs/aws/deploy/install-kubeflow/
type Tester interface {
	// Create installs Kubeflow.
	Create() error
	// Delete deletes Kubeflow.
	Delete() error
}

func NewTester(cfg Config) (Tester, error) {
	return &tester{cfg: cfg}, nil
}

type tester struct {
	cfg Config
}

func (ts *tester) Create() error {
	if ts.cfg.EKSConfig.AddOnKubeflow.Created {
		ts.cfg.Logger.Info("skipping create AddOnKubeflow")
		return nil
	}

	ts.cfg.EKSConfig.AddOnKubeflow.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()

	defer func() {
		ts.cfg.EKSConfig.AddOnKubeflow.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.AddOnKubeflow.CreateTookString = ts.cfg.EKSConfig.AddOnKubeflow.CreateTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	if err := ts.downloadInstallKfctl(); err != nil {
		return err
	}
	if err := ts.writeKfctlConfig(); err != nil {
		return err
	}
	if err := ts.installKfConfig(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.AddOnKubeflow.Created {
		ts.cfg.Logger.Info("skipping delete AddOnKubeflow")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnKubeflow.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.AddOnKubeflow.DeleteTookString = ts.cfg.EKSConfig.AddOnKubeflow.DeleteTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnKubeflow.Created = false
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) downloadInstallKfctl() (err error) {
	ts.cfg.Logger.Info("mkdir", zap.String("kfctl-path-dir", filepath.Dir(ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath)))
	if err = os.MkdirAll(filepath.Dir(ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath), 0700); err != nil {
		return fmt.Errorf("could not create %q (%v)", filepath.Dir(ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath), err)
	}

	if !fileutil.Exist(ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath) {
		tarPath := filepath.Join(os.TempDir(), fmt.Sprintf("kfctl-%x.tar.gz", time.Now().UnixNano()))
		if err = httputil.Download(ts.cfg.Logger, os.Stderr, ts.cfg.EKSConfig.AddOnKubeflow.KfctlDownloadURL, tarPath); err != nil {
			return err
		}
		tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("kfctl-%x", time.Now().UnixNano()))
		os.RemoveAll(tmpPath)
		defer os.RemoveAll(tmpPath)
		if err = archiver.Unarchive(tarPath, os.TempDir()); err != nil {
			return fmt.Errorf("failed to decompress kfctl tar file %v", err)
		}
		if err = fileutil.Copy(tmpPath, ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath); err != nil {
			return fmt.Errorf("failed to copy file %v", err)
		}
	} else {
		ts.cfg.Logger.Info("skipping kfctl download; already exist", zap.String("kfctl-path", ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath))
	}

	if err = fileutil.EnsureExecutable(ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath); err != nil {
		// file may be already executable while the process does not own the file/directory
		// ref. https://github.com/aws/aws-k8s-tester/issues/66
		ts.cfg.Logger.Warn("failed to ensure executable", zap.Error(err))
		err = nil
	}

	var output []byte
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err = exec.New().CommandContext(ctx, ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath, "help").CombinedOutput()
	cancel()
	out := string(output)
	if err != nil {
		return fmt.Errorf("'kfctl help' failed (output %q, error %v)", out, err)
	}
	fmt.Printf("\n'kfctl help' output:\n\n%s\n\n", out)

	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	output, err = exec.New().CommandContext(ctx, ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath, "apply", "--help").CombinedOutput()
	cancel()
	out = strings.TrimSpace(string(output))
	if err != nil {
		return fmt.Errorf("'kfctl apply --help' failed (output %q, error %v)", out, err)
	}
	fmt.Printf("\n'kfctl apply --help' output:\n\n%s\n\n", out)

	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	output, err = exec.New().CommandContext(ctx, ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath, "version").CombinedOutput()
	cancel()
	out = strings.TrimSpace(string(output))
	if err != nil {
		return fmt.Errorf("'kfctl version' failed (output %q, error %v)", out, err)
	}
	fmt.Printf("\n'kfctl version' output:\n\n%s\n\n", out)

	ts.cfg.Logger.Info(
		"kfctl version",
		zap.String("kfctl-path", ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath),
		zap.String("kfctl-version", out),
	)

	return nil
}

func (ts *tester) writeKfctlConfig() error {
	if ts.cfg.EKSConfig.AddOnKubeflow.KfctlConfigPath == "" {
		return errors.New("empty AddOnKubeflow.KfctlConfigPath")
	}
	ts.cfg.Logger.Info("mkdir", zap.String("kfdir", ts.cfg.EKSConfig.AddOnKubeflow.KfDir))
	if err := os.MkdirAll(ts.cfg.EKSConfig.AddOnKubeflow.KfDir, 0700); err != nil {
		return fmt.Errorf("could not create %q (%v)", ts.cfg.EKSConfig.AddOnKubeflow.KfDir, err)
	}
	ts.cfg.Logger.Info("writing kfctl config", zap.String("kfctl-config-path", ts.cfg.EKSConfig.AddOnKubeflow.KfctlConfigPath))

	nodeInstanceRoleName := ""
	if ts.cfg.EKSConfig.IsEnabledAddOnNodeGroups() {
		nodeInstanceRoleName = ts.cfg.EKSConfig.AddOnNodeGroups.RoleName
	}
	if ts.cfg.EKSConfig.IsEnabledAddOnManagedNodeGroups() {
		nodeInstanceRoleName = ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleName
	}

	tpl := template.Must(template.New("kfctlConfigTmpl").Parse(kfctlConfigTmpl))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, struct {
		Region               string
		NodeInstanceRoleName string
	}{
		ts.cfg.EKSConfig.Region,
		nodeInstanceRoleName,
	}); err != nil {
		return nil
	}
	cfgTxt := buf.String()

	f, err := os.OpenFile(ts.cfg.EKSConfig.AddOnKubeflow.KfctlConfigPath, os.O_RDWR|os.O_TRUNC, 0777)
	if err != nil {
		f, err = os.Create(ts.cfg.EKSConfig.AddOnKubeflow.KfctlConfigPath)
		if err != nil {
			return err
		}
	}
	defer f.Close()
	if _, err := f.Write([]byte(cfgTxt)); err != nil {
		return err
	}
	ts.cfg.Logger.Info("wrote kfctl config", zap.String("kfctl-config-path", ts.cfg.EKSConfig.AddOnKubeflow.KfctlConfigPath))

	fmt.Printf("\n\n'kfctl configuration' %q:\n\n%s\n\n", ts.cfg.EKSConfig.AddOnKubeflow.KfctlConfigPath, cfgTxt)
	return nil
}

// https://www.kubeflow.org/docs/aws/deploy/install-kubeflow/
// https://github.com/kubeflow/manifests/blob/master/kfdef/kfctl_aws.v1.0.2.yaml
const kfctlConfigTmpl = `
apiVersion: kfdef.apps.kubeflow.org/v1
kind: KfDef
metadata:
  namespace: kubeflow
spec:
  applications:
  - kustomizeConfig:
      parameters:
      - name: namespace
        value: istio-system
      repoRef:
        name: manifests
        path: istio/istio-crds
    name: istio-crds
  - kustomizeConfig:
      parameters:
      - name: namespace
        value: istio-system
      repoRef:
        name: manifests
        path: istio/istio-install
    name: istio-install
  - kustomizeConfig:
      parameters:
      - name: namespace
        value: istio-system
      repoRef:
        name: manifests
        path: istio/cluster-local-gateway
    name: cluster-local-gateway
  - kustomizeConfig:
      parameters:
      - name: clusterRbacConfig
        value: 'OFF'
      repoRef:
        name: manifests
        path: istio/istio
    name: istio
  - kustomizeConfig:
      parameters:
      - name: namespace
        value: istio-system
      repoRef:
        name: manifests
        path: istio/add-anonymous-user-filter
    name: add-anonymous-user-filter
  - kustomizeConfig:
      repoRef:
        name: manifests
        path: application/application-crds
    name: application-crds
  - kustomizeConfig:
      overlays:
      - application
      repoRef:
        name: manifests
        path: application/application
    name: application
  - kustomizeConfig:
      parameters:
      - name: namespace
        value: cert-manager
      repoRef:
        name: manifests
        path: cert-manager/cert-manager-crds
    name: cert-manager-crds
  - kustomizeConfig:
      parameters:
      - name: namespace
        value: kube-system
      repoRef:
        name: manifests
        path: cert-manager/cert-manager-kube-system-resources
    name: cert-manager-kube-system-resources
  - kustomizeConfig:
      overlays:
      - self-signed
      - application
      parameters:
      - name: namespace
        value: cert-manager
      repoRef:
        name: manifests
        path: cert-manager/cert-manager
    name: cert-manager
  - kustomizeConfig:
      repoRef:
        name: manifests
        path: metacontroller
    name: metacontroller
  - kustomizeConfig:
      overlays:
      - istio
      - application
      repoRef:
        name: manifests
        path: argo
    name: argo
  - kustomizeConfig:
      repoRef:
        name: manifests
        path: kubeflow-roles
    name: kubeflow-roles
  - kustomizeConfig:
      overlays:
      - istio
      - application
      repoRef:
        name: manifests
        path: common/centraldashboard
    name: centraldashboard
  - kustomizeConfig:
      overlays:
      - application
      repoRef:
        name: manifests
        path: admission-webhook/webhook
    name: webhook
  - kustomizeConfig:
      overlays:
      - application
      parameters:
      - name: webhookNamePrefix
        value: admission-webhook-
      repoRef:
        name: manifests
        path: admission-webhook/bootstrap
    name: bootstrap
  - kustomizeConfig:
      overlays:
      - istio
      - application
      parameters:
      - name: userid-header
        value: kubeflow-userid
      repoRef:
        name: manifests
        path: jupyter/jupyter-web-app
    name: jupyter-web-app
  - kustomizeConfig:
      overlays:
      - application
      repoRef:
        name: manifests
        path: spark/spark-operator
    name: spark-operator
  - kustomizeConfig:
      overlays:
      - istio
      - application
      - db
      repoRef:
        name: manifests
        path: metadata
    name: metadata
  - kustomizeConfig:
      overlays:
      - istio
      - application
      repoRef:
        name: manifests
        path: jupyter/notebook-controller
    name: notebook-controller
  - kustomizeConfig:
      overlays:
      - application
      repoRef:
        name: manifests
        path: pytorch-job/pytorch-job-crds
    name: pytorch-job-crds
  - kustomizeConfig:
      overlays:
      - application
      repoRef:
        name: manifests
        path: pytorch-job/pytorch-operator
    name: pytorch-operator
  - kustomizeConfig:
      overlays:
      - application
      parameters:
      - name: namespace
        value: knative-serving
      repoRef:
        name: manifests
        path: knative/knative-serving-crds
    name: knative-crds
  - kustomizeConfig:
      overlays:
      - application
      parameters:
      - name: namespace
        value: knative-serving
      repoRef:
        name: manifests
        path: knative/knative-serving-install
    name: knative-install
  - kustomizeConfig:
      overlays:
      - application
      repoRef:
        name: manifests
        path: kfserving/kfserving-crds
    name: kfserving-crds
  - kustomizeConfig:
      overlays:
      - application
      repoRef:
        name: manifests
        path: kfserving/kfserving-install
    name: kfserving-install
  - kustomizeConfig:
      overlays:
      - application
      parameters:
      - name: usageId
        value: <randomly-generated-id>
      - name: reportUsage
        value: 'true'
      repoRef:
        name: manifests
        path: common/spartakus
    name: spartakus
  - kustomizeConfig:
      overlays:
      - istio
      repoRef:
        name: manifests
        path: tensorboard
    name: tensorboard
  - kustomizeConfig:
      overlays:
      - application
      repoRef:
        name: manifests
        path: tf-training/tf-job-crds
    name: tf-job-crds
  - kustomizeConfig:
      overlays:
      - application
      repoRef:
        name: manifests
        path: tf-training/tf-job-operator
    name: tf-job-operator
  - kustomizeConfig:
      overlays:
      - application
      repoRef:
        name: manifests
        path: katib/katib-crds
    name: katib-crds
  - kustomizeConfig:
      overlays:
      - application
      - istio
      repoRef:
        name: manifests
        path: katib/katib-controller
    name: katib-controller
  - kustomizeConfig:
      overlays:
      - application
      repoRef:
        name: manifests
        path: pipeline/api-service
    name: api-service
  - kustomizeConfig:
      overlays:
      - application
      parameters:
      - name: minioPvcName
        value: minio-pv-claim
      repoRef:
        name: manifests
        path: pipeline/minio
    name: minio
  - kustomizeConfig:
      overlays:
      - application
      parameters:
      - name: mysqlPvcName
        value: mysql-pv-claim
      repoRef:
        name: manifests
        path: pipeline/mysql
    name: mysql
  - kustomizeConfig:
      overlays:
      - application
      repoRef:
        name: manifests
        path: pipeline/persistent-agent
    name: persistent-agent
  - kustomizeConfig:
      overlays:
      - application
      repoRef:
        name: manifests
        path: pipeline/pipelines-runner
    name: pipelines-runner
  - kustomizeConfig:
      overlays:
      - istio
      - application
      repoRef:
        name: manifests
        path: pipeline/pipelines-ui
    name: pipelines-ui
  - kustomizeConfig:
      overlays:
      - application
      repoRef:
        name: manifests
        path: pipeline/pipelines-viewer
    name: pipelines-viewer
  - kustomizeConfig:
      overlays:
      - application
      repoRef:
        name: manifests
        path: pipeline/scheduledworkflow
    name: scheduledworkflow
  - kustomizeConfig:
      overlays:
      - application
      repoRef:
        name: manifests
        path: pipeline/pipeline-visualization-service
    name: pipeline-visualization-service
  - kustomizeConfig:
      overlays:
      - application
      - istio
      repoRef:
        name: manifests
        path: profiles
    name: profiles
  - kustomizeConfig:
      overlays:
      - application
      repoRef:
        name: manifests
        path: seldon/seldon-core-operator
    name: seldon-core
  - kustomizeConfig:
      overlays:
      - application
      repoRef:
        name: manifests
        path: mpi-job/mpi-operator
    name: mpi-operator
  - kustomizeConfig:
      overlays:
      - application
      parameters:
      - name: clusterName
        value: kubeflow-aws
      repoRef:
        name: manifests
        path: aws/aws-alb-ingress-controller
    name: aws-alb-ingress-controller
  - kustomizeConfig:
      overlays:
      - application
      repoRef:
        name: manifests
        path: aws/nvidia-device-plugin
    name: nvidia-device-plugin
  plugins:
  - kind: KfAwsPlugin
    metadata:
      name: aws
    spec:
      auth:
        basicAuth:
          password:
            name: password
          username: admin
      region: {{ .Region }}
      roles:
      - {{ .NodeInstanceRoleName }}
  repos:
  - name: manifests
    uri: https://github.com/kubeflow/manifests/archive/v1.0.2.tar.gz
  version: v1.0.2

`

// https://www.kubeflow.org/docs/aws/deploy/install-kubeflow/
func (ts *tester) installKfConfig() error {
	args := []string{
		ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath,
		"apply",
		"--verbose",
		"--file=" + ts.cfg.EKSConfig.AddOnKubeflow.KfctlConfigPath,
	}
	cmdTxt := strings.Join(args, " ")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	cmd := osexec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = ts.cfg.EKSConfig.AddOnKubeflow.KfDir
	cmd.Env = []string{
		"KUBECONFIG=" + ts.cfg.EKSConfig.KubeConfigPath,
		"AWS_CLUSTER_NAME=" + ts.cfg.EKSConfig.Name,
		"KF_NAME=" + ts.cfg.EKSConfig.Name,
		"BASE_DIR=" + ts.cfg.EKSConfig.AddOnKubeflow.BaseDir,
		"KF_DIR=" + ts.cfg.EKSConfig.AddOnKubeflow.KfDir,
		"CONFIG_FILE=" + ts.cfg.EKSConfig.AddOnKubeflow.KfctlConfigPath,
	}
	for _, ev := range cmd.Env {
		ss := strings.Split(ev, "=")
		os.Setenv(ss[0], ss[1])
		defer os.Unsetenv(ss[0])
	}

	pwd, _ := os.Getwd()
	defer func() {
		err := os.Chdir(pwd)
		ts.cfg.Logger.Info("chdir", zap.String("dir", pwd), zap.Error(err))
	}()
	if err := os.Chdir(ts.cfg.EKSConfig.AddOnKubeflow.KfDir); err != nil {
		ts.cfg.Logger.Warn("chdir failed", zap.String("dir", ts.cfg.EKSConfig.AddOnKubeflow.KfDir), zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("chdir", zap.String("dir", ts.cfg.EKSConfig.AddOnKubeflow.KfDir))

	ts.cfg.Logger.Info("kfctl applying", zap.String("command", strings.Join(args, " ")))
	output, err := cmd.Output()
	if err != nil {
		// not working...
		// e.g.  Definitions:apiextensions.JSONSchemaDefinitions(nil), ExternalDocs:(*apiextensions.ExternalDocumentation)(nil), Example:(*apiextensions.JSON)(nil)}: must only have "properties", "required" or "description" at the root if the status subresource is enabled]  filename="kustomize/kustomize.go:202"
		ts.cfg.Logger.Warn("kfctl apply failed", zap.String("command", strings.Join(args, " ")), zap.Error(err))
	}
	out := string(output)
	fmt.Printf("\n'%s' (env %q) output:\n\n%s\n\n", cmdTxt, cmd.Env, out)

	if err != nil {
		// TODO: fix
		fmt.Printf("kfctl apply failed... try yourself...")
		fmt.Println("1. install aws-iam-authenticator")
		fmt.Println("2. install eksctl")
		fmt.Println("3. run following")
		fmt.Printf("\n\n%s\n\n%s\n\n", strings.Join(cmd.Env, "\n"), cmdTxt)
	}
	return nil
}
