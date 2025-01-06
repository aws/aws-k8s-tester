/*
Copyright 2019 The Kubernetes Authors.

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

// Package types defines the common types / interfaces for kubetest2 deployer
// and tester implementations
package types

import (
	"github.com/spf13/pflag"
)

// IncorrectUsage is an error with an addition HelpText() method
// NewDeployer and NewTester implementations should return a type meeting this
// interface if they want to display usage to the user when incorrect arguments
// or flags are supplied
type IncorrectUsage interface {
	error
	HelpText() string
}

// NewDeployer should return a new instance of a Deployer along with a flagset
// bound to the deployer with any additional Deployer specific CLI flags
//
// kubetest2 will call this once at startup for the injected deployer
//
// opts will provide access to options defined by common flags and kubetest2 logic
type NewDeployer func(opts Options) (deployer Deployer, flags *pflag.FlagSet)

// Options is an interface to get common options supplied by kubetest2
// to all implementations
type Options interface {
	// TODO(BenTheElder): provide getters to more common options
	// if this returns true, help text will be shown to the user after instancing
	// the deployer and tester
	HelpRequested() bool
	// if this is true, kubetest2 will be calling deployer.Build
	ShouldBuild() bool
	// if this is true, kubetest2 will be calling deployer.Up
	ShouldUp() bool
	// if this is true, kubetest2 will be calling deployer.Down
	ShouldDown() bool
	// if this is true, kubetest2 will be calling tester.Test
	ShouldTest() bool
	// if this is true, kubetest2 will be skipping reporting the test result as a JUnit test case.
	SkipTestJUnitReport() bool
	// RunID returns a unique identifier for a kubetest2 run.
	RunID() string
	// RunDir returns the directory to put run-specific output files.
	RunDir() string
	// if this is true, kubetest2 will copy the RunDIR to ARTIFACTS
	RundirInArtifacts() bool
}

// Deployer defines the interface between kubetest and a deployer
//
// If any returned error meets the:
// sigs.k8s.io/kubetest2/pkg/metadata.JUnitError
// interface, then this metadata will be pulled out when writing out the results
type Deployer interface {
	// Up should provision a new cluster for testing
	Up() error
	// Down should tear down the test cluster if any
	Down() error
	// IsUp should return true if a test cluster is successfully provisioned
	IsUp() (up bool, err error)
	// DumpClusterLogs should export logs from the cluster. It may be called
	// multiple times. Options for this should come from New(...)
	DumpClusterLogs() error
	// Build should build kubernetes and package it in whatever format
	// the deployer consumes
	Build() error
}

// Some testers may use information about the deployer not available
// in the standard deployer interface.

// DeployerWithKubeconfig adds the ability to return a path to kubeconfig file.
type DeployerWithKubeconfig interface {
	Deployer

	// Kubeconfig returns a path to a kubeconfig file for the cluster.
	Kubeconfig() (string, error)
}

// DeployerWithProvider adds the ability to return a specific provider string.
// This is reuired for some legacy deployers, which need a specific string to be
// passed through to e2e.test.
type DeployerWithProvider interface {
	Deployer

	// Provider returns the kubernetes provider for legacy deployers.
	Provider() string
}

// DeployerWithPostTester adds the ability to define after-test behavior
// based on the results of the test.
type DeployerWithPostTester interface {
	Deployer

	// PostTest runs after the tester completes.
	// testErr is the error returned from the tester's Run()
	PostTest(testErr error) error
}

// DeployerWithVersion allows the deployer to specify it's version
type DeployerWithVersion interface {
	Deployer

	// Version determines the version of the deployer binary
	Version() string
}

// DeployerWithInit adds the ability to define initialization behavior
type DeployerWithInit interface {
	Deployer

	// Init initializes the deployer. This will be called prior to any other lifecycle action.
	Init() error
}

// DeployerWithFinish adds the ability to define finalizer behavior
type DeployerWithFinish interface {
	Deployer

	// Finish finalizes the deployer. This will be called after any other deployer action and immediately before exit.
	Finish() error
}

// Tester defines the "interface" between kubetest2 and a tester
// The tester is executed as a separate binary during the Test() phase
type Tester struct {
	TesterPath string
	TesterArgs []string
}
