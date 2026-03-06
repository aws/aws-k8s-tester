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

	"github.com/aws/aws-k8s-tester/internal"
	"github.com/jstemmer/go-junit-report/v2/junit"
	"github.com/jstemmer/go-junit-report/v2/parser/gotest"
	"github.com/octago/sflags/gen/gpflag"
	"k8s.io/klog/v2"
	"sigs.k8s.io/kubetest2/pkg/artifacts"
	"sigs.k8s.io/kubetest2/pkg/testers"
)

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

	help := fs.BoolP("help", "h", false, "")
	_ = fs.Parse(os.Args[1:2])

	if *help {
		fs.Usage()
		return nil
	}

	t.argv = os.Args[1:]
	if err := testers.WriteVersionToMetadata(internal.Version, ""); err != nil {
		return err
	}
	return t.Test()
}

func expandEnv(args []string) []string {
	expanded := make([]string, len(args))
	for i, arg := range args {
		if strings.Contains(arg, `\$`) {
			expanded[i] = strings.ReplaceAll(arg, `\$`, `$`)
		} else {
			expanded[i] = os.ExpandEnv(arg)
		}
	}
	return expanded
}

func (t *Tester) Test() error {
	args := expandEnv(t.argv)
	return run(args[0], args[1:])
}

func run(binary string, args []string) error {
	var buf bytes.Buffer

	cmd := exec.Command(binary, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = io.MultiWriter(os.Stdout, &buf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &buf)

	signals := make(chan os.Signal, 5)
	signal.Notify(signals)
	defer signal.Stop(signals)

	if err := cmd.Start(); err != nil {
		return err
	}

	wait := make(chan error, 1)
	go func() {
		wait <- cmd.Wait()
		close(wait)
	}()

	var testErr error
	for {
		select {
		case sig := <-signals:
			_ = cmd.Process.Signal(sig)
		case testErr = <-wait:
			goto done
		}
	}
done:

	if err := writeJUnit(binary, &buf); err != nil {
		klog.Errorf("failed to write junit: %v", err)
	}

	return testErr
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
