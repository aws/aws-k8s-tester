package appmesh

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const (
	eksHelmChartsRepo          = "https://aws.github.io/eks-charts"
	appMeshControllerHelmChart = "appmesh-controller"
	appMeshInjectorHelmChart   = "appmesh-inject"

	appMeshControllerHelmReleaseName = "appmesh-controller"
	appMeshInjectorHelmReleaseName   = "appmesh-inject"
)

func (ts *tester) installController() error {
	vals := make(map[string]interface{})
	if ts.cfg.EKSConfig.AddOnAppMesh.ControllerImage != "" {
		imageRepo, imageTag, err := splitImageRepoAndTag(ts.cfg.EKSConfig.AddOnAppMesh.ControllerImage)
		if err != nil {
			return err
		}
		vals["image"] = map[string]interface{}{
			"repository": imageRepo,
			"tag":        imageTag,
		}
	}

	ts.cfg.Logger.Info("installing AppMesh controller",
		zap.String("controllerImage", ts.cfg.EKSConfig.AddOnAppMesh.ControllerImage),
	)
	release, err := ts.installHelmChart(eksHelmChartsRepo, appMeshControllerHelmChart, ts.cfg.EKSConfig.AddOnAppMesh.Namespace,
		appMeshControllerHelmReleaseName, vals)
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info(
		"installed AppMesh controller",
		zap.String("namespace", release.Namespace),
		zap.String("name", release.Name),
		zap.String("version", fmt.Sprintf("%v", release.Version)),
	)
	return nil
}

func (ts *tester) uninstallController() error {
	ts.cfg.Logger.Info("uninstalling AppMesh controller",
		zap.String("controllerImage", ts.cfg.EKSConfig.AddOnAppMesh.ControllerImage),
	)
	resp, err := ts.uninstallHelmChart(ts.cfg.EKSConfig.AddOnAppMesh.Namespace, appMeshControllerHelmReleaseName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return err
	}
	ts.cfg.Logger.Info(
		"uninstalled AppMesh controller",
		zap.String("namespace", resp.Release.Namespace),
		zap.String("name", resp.Release.Name),
		zap.String("version", fmt.Sprintf("%v", resp.Release.Version)),
	)
	return nil
}

func (ts *tester) installInjector() error {
	vals := make(map[string]interface{})
	if ts.cfg.EKSConfig.AddOnAppMesh.InjectorImage != "" {
		imageRepo, imageTag, err := splitImageRepoAndTag(ts.cfg.EKSConfig.AddOnAppMesh.InjectorImage)
		if err != nil {
			return err
		}
		vals["image"] = map[string]interface{}{
			"repository": imageRepo,
			"tag":        imageTag,
		}
	}
	ts.cfg.Logger.Info("installing AppMesh injector",
		zap.String("controllerImage", ts.cfg.EKSConfig.AddOnAppMesh.ControllerImage),
	)
	release, err := ts.installHelmChart(eksHelmChartsRepo, appMeshInjectorHelmChart, ts.cfg.EKSConfig.AddOnAppMesh.Namespace,
		appMeshInjectorHelmReleaseName, vals)
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info(
		"installed AppMesh injector",
		zap.String("namespace", release.Namespace),
		zap.String("name", release.Name),
		zap.String("version", fmt.Sprintf("%v", release.Version)),
	)
	return nil
}

func (ts *tester) uninstallInjector() error {
	ts.cfg.Logger.Info("uninstalling AppMesh injector",
		zap.String("controllerImage", ts.cfg.EKSConfig.AddOnAppMesh.InjectorImage),
	)
	resp, err := ts.uninstallHelmChart(ts.cfg.EKSConfig.AddOnAppMesh.Namespace, appMeshInjectorHelmReleaseName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return err
	}
	ts.cfg.Logger.Info(
		"uninstalled AppMesh injector",
		zap.String("namespace", resp.Release.Namespace),
		zap.String("name", resp.Release.Name),
		zap.String("version", fmt.Sprintf("%v", resp.Release.Version)),
	)
	return nil
}

// installHelmChart installs a helm chart into tester cluster.
func (ts *tester) installHelmChart(chartRepo string, chartName string, namespace string, releaseName string, vals map[string]interface{}) (*release.Release, error) {
	cfgFlags := genericclioptions.NewConfigFlags(false)
	cfgFlags.KubeConfig = &ts.cfg.EKSConfig.KubeConfigPath
	cfgFlags.Namespace = &namespace

	actionConfig := new(action.Configuration)
	actionConfig.Init(cfgFlags, namespace, "secrets", func(format string, v ...interface{}) {
		ts.cfg.Logger.Info(fmt.Sprintf(format, v...))
	})
	installAction := action.NewInstall(actionConfig)
	installAction.ChartPathOptions.RepoURL = chartRepo
	installAction.Namespace = namespace
	installAction.ReleaseName = releaseName
	installAction.Wait = true

	cp, err := installAction.ChartPathOptions.LocateChart(chartName, cli.New())
	chartRequested, err := loader.Load(cp)
	if err != nil {
		return nil, err
	}
	return installAction.Run(chartRequested, vals)
}

// uninstallHelmChart uninstalls a helm chart into tester cluster.
func (ts *tester) uninstallHelmChart(namespace string, releaseName string) (*release.UninstallReleaseResponse, error) {
	cfgFlags := genericclioptions.NewConfigFlags(false)
	cfgFlags.KubeConfig = &ts.cfg.EKSConfig.KubeConfigPath
	cfgFlags.Namespace = &namespace

	actionConfig := new(action.Configuration)
	actionConfig.Init(cfgFlags, namespace, "secrets", func(format string, v ...interface{}) {
		ts.cfg.Logger.Info(fmt.Sprintf(format, v...))
	})
	uninstallAction := action.NewUninstall(actionConfig)
	return uninstallAction.Run(releaseName)
}

// splitImageRepoAndTag parses a docker image in format <imageRepo>:<imageTag> into `imageRepo` and `imageTag`
func splitImageRepoAndTag(dockerImage string) (string, string, error) {
	parts := strings.Split(dockerImage, ":")
	if len(parts) != 2 {
		return "", "", errors.Errorf("dockerImage expects <imageRepo>:<imageTag>, got: %s", dockerImage)
	}
	return parts[0], parts[1], nil
}
