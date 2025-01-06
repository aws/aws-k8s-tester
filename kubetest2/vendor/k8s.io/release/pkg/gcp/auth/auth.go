/*
Copyright 2020 The Kubernetes Authors.

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

package auth

import (
	"errors"
	"fmt"
	"strings"

	"sigs.k8s.io/release-sdk/gcli"
)

func GetCurrentGCPUser() (string, error) {
	gcpUser, err := gcli.GCloudOutput(
		"auth",
		"list",
		"--filter=status:ACTIVE",
		"--format=value(account)",
		"--verbosity=debug",
	)
	if err != nil {
		return "", err
	}

	if gcpUser == "" {
		return "", errors.New("the GCP user name should not be empty")
	}

	gcpUser = NormalizeGCPUser(gcpUser)

	return gcpUser, nil
}

func ConfigureDocker() error {
	err := gcli.GCloud(
		"auth",
		"configure-docker",
	)
	if err != nil {
		return fmt.Errorf("running 'gcloud auth configure-docker': %w", err)
	}

	return nil
}

func NormalizeGCPUser(gcpUser string) string {
	gcpUser = strings.TrimSpace(gcpUser)
	gcpUser = strings.ReplaceAll(gcpUser, "@", "-at-")
	gcpUser = strings.ReplaceAll(gcpUser, ".", "-")
	gcpUser = strings.ToLower(gcpUser)

	return gcpUser
}
