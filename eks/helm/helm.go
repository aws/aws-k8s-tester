// Package helm implements helm utilities.
package helm

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

/*
helm repo add stable https://kubernetes-charts.storage.googleapis.com
helm repo update
helm search repo stable

helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update
helm search repo bitnami

helm repo add eks https://aws.github.io/eks-charts
helm repo update
helm search repo eks

helm repo add jupyterhub https://jupyterhub.github.io/helm-chart/
helm repo update
helm search repo jupyterhub

https://github.com/jupyterhub/zero-to-jupyterhub-k8s/blob/master/jupyterhub/values.yaml
*/

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

// InstallConfig defines helm installation configuration.
type InstallConfig struct {
	Logger *zap.Logger

	Stopc   chan struct{}
	Timeout time.Duration

	KubeConfigPath string
	Namespace      string
	ChartRepoURL   string
	ChartName      string
	ReleaseName    string
	Values         map[string]interface{}

	QueryFunc     func()
	QueryInterval time.Duration
}

const defaultQueryInterval = 30 * time.Second

// Install installs a helm chart.
func Install(cfg InstallConfig) (err error) {
	if cfg.QueryInterval == 0 {
		cfg.QueryInterval = defaultQueryInterval
	}

	cfg.Logger.Info("installing chart",
		zap.String("namespace", cfg.Namespace),
		zap.String("chart-repo-url", cfg.ChartRepoURL),
		zap.String("chart-name", cfg.ChartName),
		zap.String("release-name", cfg.ReleaseName),
	)

	cfgFlags := genericclioptions.NewConfigFlags(false)
	cfgFlags.KubeConfig = &cfg.KubeConfigPath
	cfgFlags.Namespace = &cfg.Namespace

	act := new(action.Configuration)
	if err := act.Init(
		cfgFlags,
		cfg.Namespace,
		"secrets",
		func(format string, v ...interface{}) {
			cfg.Logger.Info(fmt.Sprintf("[install] "+format, v...))
		},
	); err != nil {
		return err
	}

	install := action.NewInstall(act)
	install.Namespace = cfg.Namespace
	install.ReleaseName = cfg.ReleaseName
	install.Wait = true
	install.Timeout = cfg.Timeout

	var chart *chart.Chart
	switch {
	case strings.HasSuffix(cfg.ChartRepoURL, ".tgz"):
		// https://github.com/kubernetes-sigs/aws-ebs-csi-driver#deploy-driver
		var rd io.ReadCloser
		retryStart, waitDur := time.Now(), 3*time.Minute
		for time.Now().Sub(retryStart) < waitDur {
			var resp *http.Response
			resp, err = http.Get(cfg.ChartRepoURL)
			if err != nil {
				cfg.Logger.Warn("failed to download tar", zap.Error(err))
				time.Sleep(5 * time.Second)
				continue
			}
			rd = resp.Body
			break
		}
		if err != nil {
			return err
		}
		defer rd.Close()
		cfg.Logger.Info("downloading chart .tgz", zap.String("url", cfg.ChartRepoURL))
		chart, err = loader.LoadArchive(rd)
		if err != nil {
			return err
		}
		cfg.Logger.Info("loaded chart via .tgz",
			zap.String("namespace", cfg.Namespace),
			zap.String("chart-repo", cfg.ChartRepoURL),
			zap.String("chart-name", cfg.ChartName),
			zap.String("release-name", cfg.ReleaseName),
			zap.String("chart-full-path", chart.ChartFullPath()),
			zap.String("chart-name", chart.Name()),
			zap.String("chart-app-version", chart.AppVersion()),
		)

	default:
		cfg.Logger.Info("locating chart",
			zap.String("namespace", cfg.Namespace),
			zap.String("chart-repo", cfg.ChartRepoURL),
			zap.String("chart-name", cfg.ChartName),
			zap.String("release-name", cfg.ReleaseName),
		)
		install.ChartPathOptions.RepoURL = cfg.ChartRepoURL
		chartPath, err := install.ChartPathOptions.LocateChart(cfg.ChartName, cli.New())
		if err != nil {
			cfg.Logger.Warn("failed to locate chart",
				zap.String("chart-repo", cfg.ChartRepoURL),
				zap.String("chart-name", cfg.ChartName),
				zap.Error(err),
			)
			return err
		}
		cfg.Logger.Info("located chart",
			zap.String("namespace", cfg.Namespace),
			zap.String("chart-repo", cfg.ChartRepoURL),
			zap.String("chart-name", cfg.ChartName),
			zap.String("release-name", cfg.ReleaseName),
			zap.String("chart-path", chartPath),
		)

		cfg.Logger.Info("loading chart",
			zap.String("namespace", cfg.Namespace),
			zap.String("chart-repo", cfg.ChartRepoURL),
			zap.String("chart-name", cfg.ChartName),
			zap.String("release-name", cfg.ReleaseName),
			zap.String("chart-path", chartPath),
		)
		chart, err = loader.Load(chartPath)
		if err != nil {
			cfg.Logger.Warn("failed to load chart",
				zap.String("chart-repo", cfg.ChartRepoURL),
				zap.String("chart-name", cfg.ChartName),
				zap.String("chart-path", chartPath),
				zap.Error(err),
			)
			return err
		}
		cfg.Logger.Info("loaded chart via remote repo",
			zap.String("namespace", cfg.Namespace),
			zap.String("chart-repo", cfg.ChartRepoURL),
			zap.String("chart-name", cfg.ChartName),
			zap.String("release-name", cfg.ReleaseName),
			zap.String("chart-path", chartPath),
			zap.String("chart-full-path", chart.ChartFullPath()),
			zap.String("chart-name", chart.Name()),
			zap.String("chart-app-version", chart.AppVersion()),
		)
	}

	donec1, donec2 := make(chan struct{}), make(chan struct{})
	if cfg.QueryFunc != nil {
		go func() {
			cfg.Logger.Info("starting query function for-loop", zap.Duration("interval", cfg.QueryInterval))
			for {
				select {
				case <-donec1:
					cfg.Logger.Warn("closing goroutine")
					close(donec2)
					return
				case <-cfg.Stopc:
					cfg.Logger.Warn("stopping goroutine")
					return
				case <-time.After(cfg.QueryInterval):
				}

				cfg.Logger.Info("executing query function")
				cfg.QueryFunc()
				cfg.Logger.Info("executed query function")
			}
		}()
	}

	rs, err := install.Run(chart, cfg.Values)
	if err != nil {
		cfg.Logger.Warn("failed to install chart", zap.String("release-name", cfg.ReleaseName), zap.Error(err))
	} else {
		cfg.Logger.Info("installed chart",
			zap.String("namespace", rs.Namespace),
			zap.String("name", rs.Name),
			zap.String("version", fmt.Sprintf("%v", rs.Version)),
		)
	}

	if cfg.QueryFunc != nil {
		close(donec1)
		select {
		case <-donec2:
		case <-cfg.Stopc:
		}
	}

	return err
}

// Uninstall uninstalls a helm chart.
func Uninstall(cfg InstallConfig) error {
	cfg.Logger.Info("uninstalling chart",
		zap.String("namespace", cfg.Namespace),
		zap.String("release-name", cfg.ReleaseName),
	)

	cfgFlags := genericclioptions.NewConfigFlags(false)
	cfgFlags.KubeConfig = &cfg.KubeConfigPath
	cfgFlags.Namespace = &cfg.Namespace

	act := new(action.Configuration)
	if err := act.Init(
		cfgFlags,
		cfg.Namespace,
		"secrets",
		func(format string, v ...interface{}) {
			cfg.Logger.Info(fmt.Sprintf("[uninstall] "+format, v...))
		},
	); err != nil {
		return err
	}

	uninstall := action.NewUninstall(act)
	uninstall.Timeout = cfg.Timeout

	rs, err := uninstall.Run(cfg.ReleaseName)
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			cfg.Logger.Warn("failed to uninstall chart", zap.String("release-name", cfg.ReleaseName), zap.Error(err))
			return err
		}
		cfg.Logger.Info("uninstalled chart", zap.Error(err))
		return nil
	}
	cfg.Logger.Info("uninstalled chart",
		zap.String("namespace", rs.Release.Namespace),
		zap.String("name", rs.Release.Name),
		zap.String("version", fmt.Sprintf("%v", rs.Release.Version)),
		zap.Error(err),
	)
	return nil
}
