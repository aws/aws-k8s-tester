/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package spdx

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"sigs.k8s.io/bom/pkg/license"
	"sigs.k8s.io/release-utils/http"
	"sigs.k8s.io/release-utils/util"
)

const (
	goRunnerVersionURL = "https://raw.githubusercontent.com/kubernetes/release/master/images/build/go-runner/VERSION"
	goRunnerLicenseURL = "https://raw.githubusercontent.com/kubernetes/release/master/images/build/go-runner/Dockerfile"
)

type goRunnerHandler struct {
	reader  *license.Reader
	Options *ContainerLayerAnalyzerOptions
}

func (h *goRunnerHandler) ReadPackageData(layerPath string, pkg *Package) error { //nolint: revive
	pkg.Supplier.Person = "Kubernetes Release Managers (release-managers@kubernetes.io)"
	pkg.Name = "go-runner"

	// Get the go-runner version
	// TODO: Add http retries
	versionb, err := http.NewAgent().Get(goRunnerVersionURL)
	if err != nil {
		return fmt.Errorf("fetching go-runner VERSION file: %w", err)
	}
	logrus.Infof("go-runner image is at version %s", string(versionb))
	pkg.Version = string(versionb)

	// Read the docker file to scan for license
	lic, err := http.NewAgent().Get(goRunnerLicenseURL)
	if err != nil {
		return fmt.Errorf("fetching go-runner VERSION file: %w", err)
	}

	df, err := os.CreateTemp(os.TempDir(), "gorunner-dockerfile-")
	if err != nil {
		return fmt.Errorf("creating temporary file to read go-runner license: %w", err)
	}
	defer df.Close()
	defer os.Remove(df.Name())

	if err := os.WriteFile(df.Name(), lic, os.FileMode(0o644)); err != nil {
		return fmt.Errorf("writing go-runner license to temp file: %w", err)
	}

	// Let's extract the license for the layer:
	var grlic *license.License
	licenseReader, err := h.licenseReader(h.Options)
	if err != nil {
		return fmt.Errorf("getting license reader: %w", err)
	}
	// First, check if the file has our boiler plate
	hasbp, err := license.HasKubernetesBoilerPlate(df.Name())
	if err != nil {
		return fmt.Errorf("checking for k8s boilerplate in go-runner: %w", err)
	}
	// If the boilerplate was found, we know it is apache2
	if hasbp {
		grlic = licenseReader.LicenseFromLabel("Apache-2.0")
		// Otherwise, as a fallback, try to classify the file
	} else {
		grlic, err = licenseReader.LicenseFromFile(df.Name())
		if err != nil {
			return fmt.Errorf("attempting to read go-runner license: %w", err)
		}
	}
	pkg.LicenseDeclared = grlic.LicenseID
	logrus.Infof("Found license %s in go-runner image", grlic.LicenseID)
	return nil
}

// licenseReader returns a reusable license reader
func (h *goRunnerHandler) licenseReader(o *ContainerLayerAnalyzerOptions) (*license.Reader, error) {
	if h.reader == nil {
		logrus.Info("Initializing licence reader with default options")
		// We use a default license cache
		opts := license.DefaultReaderOptions
		ldir := filepath.Join(os.TempDir(), spdxLicenseDlCache)
		// ... unless overridden by the options
		if o.LicenseCacheDir != "" {
			ldir = o.LicenseCacheDir
		}

		// If the license cache does not exist, create it
		if !util.Exists(ldir) {
			if err := os.MkdirAll(ldir, os.FileMode(0o0755)); err != nil {
				return nil, fmt.Errorf("creating license cache directory: %w", err)
			}
		}
		opts.CacheDir = ldir
		opts.LicenseDir = filepath.Join(os.TempDir(), spdxLicenseData)
		// Create the new reader
		reader, err := license.NewReaderWithOptions(opts)
		if err != nil {
			return nil, fmt.Errorf("creating reusable license reader: %w", err)
		}
		h.reader = reader
	}
	return h.reader, nil
}

// CanHandle returns a bools indicating if this handle can supply more
// data about the specified tarball
func (h *goRunnerHandler) CanHandle(layerPath string) (can bool, err error) {
	// Open the tar file
	f, err := os.Open(layerPath)
	if err != nil {
		return can, fmt.Errorf("opening tarball: %w", err)
	}
	defer f.Close()
	var tr *tar.Reader
	if filepath.Ext(layerPath) == ".gz" {
		gzf, err := gzip.NewReader(f)
		if err != nil {
			return can, fmt.Errorf("creating gzip reader: %w", err)
		}
		tr = tar.NewReader(gzf)
	} else {
		tr = tar.NewReader(f)
	}

	binaryFound := false
	// Search for the os-file in the tar contents
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return can, fmt.Errorf("reading the image tarfile at %s: %w", layerPath, err)
		}

		if hdr.FileInfo().IsDir() {
			continue
		}

		// Scan for the os-release file in the tarball
		if hdr.Name == "go-runner" {
			binaryFound = true
			logrus.Infof("👍 Tarball %s identified as a go-runner layer", layerPath)
			break
		}
	}
	return binaryFound, nil
}
