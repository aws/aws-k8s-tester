// Package helm implements helm utilities.
package helm

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var settings *cli.EnvSettings

func init() {
	settings = cli.New()
}

// RepoAdd adds repo with given name and url
func RepoAdd(lg *zap.Logger, name, url string) error {
	repoFile := settings.RepositoryConfig

	err := os.MkdirAll(filepath.Dir(repoFile), os.ModePerm)
	if err != nil && !os.IsExist(err) {
		return err
	}

	fck := flock.New(strings.Replace(repoFile, filepath.Ext(repoFile), ".lock", 1))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	locked, err := fck.TryLockContext(ctx, time.Second)
	defer cancel()
	if err == nil && locked {
		defer fck.Unlock()
	}
	if err != nil {
		return err
	}

	lg.Info("acquired flock; adding repo", zap.String("repo-file", repoFile), zap.String("name", name))
	b, err := ioutil.ReadFile(repoFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var f repo.File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return err
	}
	if f.Has(name) {
		lg.Info("repository name already exists", zap.String("name", name))
		return nil
	}

	c := repo.Entry{
		Name: name,
		URL:  url,
	}
	r, err := repo.NewChartRepository(&c, getter.All(settings))
	if err != nil {
		return err
	}

	if _, err := r.DownloadIndexFile(); err != nil {
		lg.Warn("failed to download index file", zap.String("url", url), zap.Error(err))
		return err
	}

	f.Update(&c)

	if err := f.WriteFile(repoFile, 0644); err != nil {
		return err
	}

	lg.Info("added repo", zap.String("name", name))
	return nil
}

// Install installs a helm chart.
func Install(
	lg *zap.Logger,
	timeout time.Duration,
	kubeconfig string,
	namespace string,
	chartRepoURL string,
	chartName string,
	releaseName string,
	values map[string]interface{},
) error {
	copied := make(map[string]string)
	for k, v := range values {
		if strings.Contains(strings.ToLower(k), "password") {
			v = "[redacted]"
		}
		copied[k] = fmt.Sprintf("%v", v)
	}
	lg.Info("installing chart",
		zap.String("namespace", namespace),
		zap.String("chart-repo-url", chartRepoURL),
		zap.String("chart-name", chartName),
		zap.String("release-name", releaseName),
		zap.String("values", fmt.Sprintf("%+v", copied)),
	)

	cfgFlags := genericclioptions.NewConfigFlags(false)
	cfgFlags.KubeConfig = &kubeconfig
	cfgFlags.Namespace = &namespace

	act := new(action.Configuration)
	if err := act.Init(
		cfgFlags,
		namespace,
		"secrets",
		func(format string, v ...interface{}) {
			lg.Info(fmt.Sprintf("[helm-install-log] "+format, v...))
		},
	); err != nil {
		return err
	}

	install := action.NewInstall(act)
	install.ChartPathOptions.RepoURL = chartRepoURL
	install.Namespace = namespace
	install.ReleaseName = releaseName
	install.Wait = true
	install.Timeout = timeout

	lg.Info("locating chart",
		zap.String("namespace", namespace),
		zap.String("chart-repo", chartRepoURL),
		zap.String("chart-name", chartName),
		zap.String("release-name", releaseName),
	)
	chartPath, err := install.ChartPathOptions.LocateChart(chartName, cli.New())
	if err != nil {
		lg.Warn("failed to locate chart",
			zap.String("chart-repo", chartRepoURL),
			zap.String("chart-name", chartName),
			zap.Error(err),
		)
		return err
	}
	lg.Info("located chart",
		zap.String("namespace", namespace),
		zap.String("chart-repo", chartRepoURL),
		zap.String("chart-name", chartName),
		zap.String("release-name", releaseName),
		zap.String("chart-path", chartPath),
	)

	lg.Info("loading chart",
		zap.String("namespace", namespace),
		zap.String("chart-repo", chartRepoURL),
		zap.String("chart-name", chartName),
		zap.String("release-name", releaseName),
		zap.String("chart-path", chartPath),
	)
	chart, err := loader.Load(chartPath)
	if err != nil {
		lg.Warn("failed to load chart",
			zap.String("chart-repo", chartRepoURL),
			zap.String("chart-name", chartName),
			zap.String("chart-path", chartPath),
			zap.Error(err),
		)
		return err
	}
	lg.Info("loaded chart",
		zap.String("namespace", namespace),
		zap.String("chart-repo", chartRepoURL),
		zap.String("chart-name", chartName),
		zap.String("release-name", releaseName),
		zap.String("chart-path", chartPath),
		zap.String("chart-full-path", chart.ChartFullPath()),
		zap.String("chart-name", chart.Name()),
		zap.String("chart-app-version", chart.AppVersion()),
	)

	rs, err := install.Run(chart, values)
	if err != nil {
		lg.Warn("failed to install chart", zap.String("release-name", releaseName), zap.Error(err))
		return err
	}
	lg.Info("installed chart",
		zap.String("namespace", rs.Namespace),
		zap.String("name", rs.Name),
		zap.String("version", fmt.Sprintf("%v", rs.Version)),
	)
	return nil
}

// Uninstall uninstalls a helm chart.
func Uninstall(
	lg *zap.Logger,
	timeout time.Duration,
	kubeconfig string,
	namespace string,
	releaseName string,
) error {
	lg.Info("uninstalling chart",
		zap.String("namespace", namespace),
		zap.String("release-name", releaseName),
	)

	cfgFlags := genericclioptions.NewConfigFlags(false)
	cfgFlags.KubeConfig = &kubeconfig
	cfgFlags.Namespace = &namespace

	act := new(action.Configuration)
	if err := act.Init(
		cfgFlags,
		namespace,
		"secrets",
		func(format string, v ...interface{}) {
			lg.Info(fmt.Sprintf("[helm-uninstall-log] "+format, v...))
		},
	); err != nil {
		return err
	}

	uninstall := action.NewUninstall(act)
	uninstall.Timeout = timeout

	rs, err := uninstall.Run(releaseName)
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			lg.Warn("failed to uninstall chart", zap.String("release-name", releaseName), zap.Error(err))
			return err
		}
		lg.Info("uninstalled chart", zap.Error(err))
		return nil
	}
	lg.Info("uninstalled chart",
		zap.String("namespace", rs.Release.Namespace),
		zap.String("name", rs.Release.Name),
		zap.String("version", fmt.Sprintf("%v", rs.Release.Version)),
		zap.Error(err),
	)
	return nil
}
