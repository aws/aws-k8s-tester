package prow

import (
	"crypto/tls"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aws/aws-k8s-tester/pkg/fileutil"

	"go.uber.org/zap"
)

// DownloadJobsUpstream downloads all Prow configurations from upstream "kubernetes/test-infra".
func DownloadJobsUpstream(lg *zap.Logger) (dir string, paths []string, err error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	u := "https://github.com/kubernetes/test-infra/archive/master.zip"
	lg.Info("downloading", zap.String("url", u))

	var resp *http.Response
	resp, err = http.Get(u)
	if err != nil {
		return "", nil, err
	}
	var d []byte
	d, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}
	resp.Body.Close()
	lg.Info("downloaded", zap.String("url", u))

	var f *os.File
	f, err = ioutil.TempFile(os.TempDir(), "git-test-ekstest")
	if err != nil {
		return "", nil, err
	}
	p := f.Name()
	os.RemoveAll(p)
	defer os.RemoveAll(p)
	if err = ioutil.WriteFile(p, d, os.ModePerm); err != nil {
		return "", nil, err
	}

	dir, err = ioutil.TempDir(os.TempDir(), "git-test-dir-ekstest")
	if err != nil {
		return "", nil, err
	}
	os.RemoveAll(dir)

	if err = unzip(lg, p, dir); err != nil {
		return "", nil, err
	}
	dir = filepath.Join(dir, "test-infra-master")

	// https://github.com/kubernetes/test-infra/blob/master/prow/config.yaml
	// but this is being moved to "config/jobs/*"
	// see https://github.com/kubernetes/test-infra/issues/8485
	cp := filepath.Join(dir, "prow/config.yaml")
	if fileutil.Exist(cp) {
		paths = []string{cp}
	}

	// https://github.com/kubernetes/test-infra/tree/master/config/jobs
	// ref. https://github.com/kubernetes/test-infra/issues/8485
	visit := func(path string, f os.FileInfo, err error) error {
		if f == nil {
			return nil
		}
		if f.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".yaml" {
			paths = append(paths, path)
		}
		return nil
	}
	if err := filepath.Walk(filepath.Join(dir, "config/jobs"), visit); err != nil {
		return "", nil, err
	}

	lg.Info("fetched all prow config files", zap.Int("jobs", len(paths)), zap.String("dir", dir))
	return dir, paths, nil
}
