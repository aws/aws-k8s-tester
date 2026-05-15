// This file is derived from the following files in kubetest2:
// - pkg/testers/exec/exec.go (Tester, Execute, expandEnv, Main)
// - pkg/process/exec.go (execCmdWithSignals)
// - pkg/process/mutexwriter.go (mutexWriter)
// - pkg/process/junitexec.go (output capture via MultiWriter)
//
// Modifications:
// 1. Instead of returning a JUnitError, output is parsed with go-junit-report
//    to produce per-test-case JUnit XML files.
// 2. The package has been renamed from `exec` to `gotestjunit`.

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

package gotestjunit

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jstemmer/go-junit-report/v2/junit"
	"github.com/jstemmer/go-junit-report/v2/parser/gotest"
	"github.com/urfave/sflags/gen/gpflag"
	"k8s.io/klog/v2"
	"sigs.k8s.io/kubetest2/pkg/artifacts"
	"sigs.k8s.io/kubetest2/pkg/testers"
)

var GitTag string

type Tester struct {
	argv []string
}

const usage = `kubetest2 --test=gotest-junit -- [TestCommand] [TestArgs]
  TestCommand: the Go test binary to invoke
  TestArgs:    arguments passed to test command

This tester executes a Go test binary and generates JUnit XML output.
`

func (t *Tester) Execute() error {
	fs, err := gpflag.Parse(t)
	if err != nil {
		return fmt.Errorf("failed to initialize tester: %v", err)
	}

	fs.Usage = func() {
		fmt.Print(usage)
	}

	if len(os.Args) < 2 {
		fs.Usage()
		return nil
	}

	// gracefully handle -h or --help if it is the only argument
	help := fs.BoolP("help", "h", false, "")
	// we don't care about errors, only if -h / --help was set
	_ = fs.Parse(os.Args[1:2])

	if *help {
		fs.Usage()
		return nil
	}

	t.argv = os.Args[1:]
	if err := testers.WriteVersionToMetadata(GitTag, ""); err != nil {
		return err
	}
	return t.Test()
}

func expandEnv(args []string) []string {
	expandedArgs := make([]string, len(args))
	for i, arg := range args {
		// best effort handle literal dollar for backward compatibility
		// this is not an all-purpose shell special character handler
		if strings.Contains(arg, `\$`) {
			expandedArgs[i] = strings.ReplaceAll(arg, `\$`, `$`)
		} else {
			expandedArgs[i] = os.ExpandEnv(arg)
		}
	}
	return expandedArgs
}

func (t *Tester) Test() error {
	expandedArgs := expandEnv(t.argv)
	return run(expandedArgs[0], expandedArgs[1:])
}

// Without -test.v, Go test binaries only emit output for failing tests
// and nothing for passing tests, so go-junit-report cannot produce
// complete JUnit XML.
func ensureVerbose(args []string) []string {
	for i, arg := range args {
		if strings.HasPrefix(arg, "-test.v") {
			args[i] = "-test.v=true"
			return args
		}
	}
	return append(args, "-test.v=true")
}

func run(binary string, args []string) error {
	args = ensureVerbose(args)

	var buf bytes.Buffer
	syncBuf := &mutexWriter{writer: &buf}

	cmd := exec.Command(binary, args...)
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	cmd.Stdout = io.MultiWriter(os.Stdout, syncBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, syncBuf)

	testErr := execCmdWithSignals(cmd)

	if err := writeJUnit(binary, &buf); err != nil {
		klog.Errorf("failed to write junit: %v", err)
	}

	return testErr
}

// mutexWriter is a simple synchronized wrapper around an io.Writer
type mutexWriter struct {
	writer io.Writer
	mu     sync.Mutex
}

func (m *mutexWriter) Write(b []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writer.Write(b)
}

func execCmdWithSignals(cmd *exec.Cmd) error {
	// setup listener to forward all signals
	signals := make(chan os.Signal, 5)
	signal.Notify(signals)
	defer signal.Stop(signals)

	// start the process
	if err := cmd.Start(); err != nil {
		return err
	}

	// set up a channel to monitor for when it exits
	wait := make(chan error, 1)
	go func() {
		wait <- cmd.Wait()
		close(wait)
	}()

	// pass all signals to the subcommand until it exits, return the result
	for {
		select {
		case sig := <-signals:
			_ = cmd.Process.Signal(sig)
		case err := <-wait:
			return err
		}
	}
}

func writeJUnit(binary string, output *bytes.Buffer) error {
	parser := gotest.NewParser()
	report, err := parser.Parse(output)
	if err != nil {
		return err
	}

	name := filepath.Base(binary)
	name = strings.TrimSuffix(name, ".test")

	if err := os.MkdirAll(artifacts.BaseDir(), 0755); err != nil {
		return err
	}

	hostname, _ := os.Hostname()
	testsuites := junit.CreateFromReport(report, hostname)

	for i, suite := range testsuites.Suites {
		if suite.Name == "" {
			testsuites.Suites[i].Name = name
		}
		filename := fmt.Sprintf("junit_%s.xml", name)
		if len(testsuites.Suites) > 1 {
			filename = fmt.Sprintf("junit_%s_%02d.xml", name, i)
		}
		f, err := os.Create(filepath.Join(artifacts.BaseDir(), filename))
		if err != nil {
			return err
		}
		single := junit.Testsuites{Suites: []junit.Testsuite{testsuites.Suites[i]}}
		err = single.WriteXML(f)
		f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func NewDefaultTester() *Tester {
	return &Tester{}
}

func Main() {
	t := NewDefaultTester()
	if err := t.Execute(); err != nil {
		klog.Fatalf("failed to run gotest-junit tester: %v", err)
	}
}
