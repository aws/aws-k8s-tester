// Package github implements github utilities.
package github

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

// Download downloads aws-k8s-tester binary from github release.
func Download(lg *zap.Logger, tag string, fpath string) (string, error) {
	if lg == nil {
		lg = zap.NewNop()
	}
	if tag == "" {
		tag = "latest"
	}
	if fpath == "" {
		f, err := ioutil.TempFile(os.TempDir(), "aws-k8s-tester")
		if err != nil {
			return "", err
		}
		fpath = f.Name()
		f.Close()
		os.RemoveAll(fpath)
	}
	fpath, _ = filepath.Abs(fpath)

	now := time.Now()

	r, err := Query(lg, tag)
	if err != nil {
		lg.Warn("failed to query release", zap.Error(err))
		return fpath, err
	}
	downloadURL := ""
	for _, asset := range r.Assets {
		ext := filepath.Ext(asset.Name)
		if ext == ".asc" {
			continue
		}
		if asset.ContentType != "application/octet-stream" {
			continue
		}
		if !strings.HasSuffix(asset.Name, runtime.GOOS+"-"+runtime.GOARCH) {
			continue
		}
		downloadURL = asset.BrowserDownloadURL
		lg.Info("downloading release", zap.String("name", asset.Name), zap.String("updated-ago", asset.UpdatedAgo))
		break
	}
	if downloadURL == "" {
		lg.Warn("failed to find release asset")
		return fpath, errors.New("no release asset found")
	}

	lg.Info("mkdir", zap.String("dir", filepath.Dir(fpath)))
	if err := os.MkdirAll(filepath.Dir(fpath), 0700); err != nil {
		return fpath, fmt.Errorf("could not create %q (%v)", filepath.Dir(fpath), err)
	}
	if err := os.RemoveAll(fpath); err != nil {
		return fpath, err
	}
	f, err := os.Create(fpath)
	if err != nil {
		return fpath, fmt.Errorf("failed to create %q (%v)", fpath, err)
	}
	if err := httpDownloadFile(lg, downloadURL, f); err != nil {
		return fpath, err
	}
	if err := f.Close(); err != nil {
		return fpath, fmt.Errorf("failed to close %v", err)
	}
	if err := fileutil.EnsureExecutable(fpath); err != nil {
		// file may be already executable while the process does not own the file/directory
		// ref. https://github.com/aws/aws-k8s-tester/issues/66
		lg.Warn("failed to ensure executable", zap.Error(err))
		err = nil
	}

	// aws-iam-authenticator version
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	co, err := exec.New().CommandContext(
		ctx,
		fpath,
		"version",
	).CombinedOutput()
	cancel()
	if err != nil {
		return fpath, fmt.Errorf("'aws-k8s-tester version' failed (output %q, error %v)", string(co), err)
	}

	st, err := os.Stat(fpath)
	lg.Info(
		"downloaded release",
		zap.String("file-path", fpath),
		zap.String("version", string(co)),
		zap.String("size", humanize.Bytes(uint64(st.Size()))),
		zap.String("took", humanize.Time(now)),
		zap.Error(err),
	)
	return fpath, nil
}

// Query fetches github release information.
// ref. https://developer.github.com/v3/repos/releases
// e.g.
//  https://api.github.com/repos/aws/aws-k8s-tester/releases/latest
//  https://api.github.com/repos/aws/aws-k8s-tester/releases/tags/v0.8.5
func Query(lg *zap.Logger, tag string) (*Release, error) {
	if lg == nil {
		lg = zap.NewNop()
	}
	if tag == "" {
		tag = "latest"
	}
	url := fmt.Sprintf("https://api.github.com/repos/aws/aws-k8s-tester/releases/tags/%s", tag)
	if tag == "latest" {
		url = "https://api.github.com/repos/aws/aws-k8s-tester/releases/latest"
	}

	lg.Info("querying release", zap.String("url", url))
	buf := bytes.NewBuffer(nil)
	if err := httpReadInsecure(lg, url, buf); err != nil {
		return nil, err
	}
	r := &Release{}
	if err := json.NewDecoder(buf).Decode(r); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	now = time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, now.Location())
	r.PublishedAgo = humanize.RelTime(r.PublishedAt, now, "ago", "from now")
	for i := range r.Assets {
		r.Assets[i].CreatedAgo = humanize.RelTime(r.Assets[i].CreatedAt, now, "ago", "from now")
		r.Assets[i].UpdatedAgo = humanize.RelTime(r.Assets[i].UpdatedAt, now, "ago", "from now")
		r.Assets[i].SizeString = humanize.Bytes(r.Assets[i].Size)
		r.TotalAssetsSize += r.Assets[i].Size
	}
	r.TotalAssetsSizeString = humanize.Bytes(r.TotalAssetsSize)

	lg.Info("queried release",
		zap.String("release-name", r.Name),
		zap.String("tag-name", r.TagName),
		zap.String("published", r.PublishedAgo),
		zap.Int("assets", len(r.Assets)),
		zap.String("total-assets-size", r.TotalAssetsSizeString),
	)
	return r, nil
}

// Release represents github release.
type Release struct {
	HTMLURL               string    `json:"html_url"`
	Name                  string    `json:"name"`
	TagName               string    `json:"tag_name"`
	Assets                []Asset   `json:"assets"`
	PublishedAt           time.Time `json:"published_at"`
	PublishedAgo          string
	TotalAssetsSize       uint64
	TotalAssetsSizeString string
}

// Asset represents github release assets.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	ContentType        string `json:"content_type"`
	Size               uint64 `json:"size"`
	SizeString         string
	DownloadCount      int       `json:"download_count"`
	State              string    `json:"state"`
	CreatedAt          time.Time `json:"created_at"`
	CreatedAgo         string
	UpdatedAt          time.Time `json:"updated_at"`
	UpdatedAgo         string
}

// curl -k [URL]
func httpReadInsecure(lg *zap.Logger, u string, wr io.Writer) error {
	lg.Info("reading", zap.String("url", u))
	cli := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}}
	r, err := cli.Get(u)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode >= 400 {
		return fmt.Errorf("%q returned %d", u, r.StatusCode)
	}

	_, err = io.Copy(wr, r.Body)
	if err != nil {
		lg.Warn("failed to read", zap.String("url", u), zap.Error(err))
	} else {
		lg.Info("read",
			zap.String("url", u),
		)
	}
	return err
}

var httpFileTransport *http.Transport

func init() {
	httpFileTransport = new(http.Transport)
	httpFileTransport.RegisterProtocol("file", http.NewFileTransport(http.Dir("/")))
}

// curl -L [URL] | writer
func httpDownloadFile(lg *zap.Logger, u string, wr io.Writer) error {
	lg.Info("downloading", zap.String("url", u))
	cli := &http.Client{Transport: httpFileTransport}
	r, err := cli.Get(u)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode >= 400 {
		return fmt.Errorf("%q returned %d", u, r.StatusCode)
	}

	_, err = io.Copy(wr, r.Body)
	if err != nil {
		lg.Warn("failed to download", zap.String("url", u), zap.Error(err))
	} else {
		if f, ok := wr.(*os.File); ok {
			lg.Info("downloaded",
				zap.String("url", u),
				zap.String("file-path", f.Name()),
			)
		} else {
			lg.Info("downloaded",
				zap.String("url", u),
				zap.String("value-of", reflect.ValueOf(wr).String()),
			)
		}
	}
	return err
}
