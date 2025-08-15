package multi

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-k8s-tester/internal"
	"github.com/urfave/sflags/gen/gpflag"
	"k8s.io/klog"
	"sigs.k8s.io/kubetest2/pkg/app/shim"
	"sigs.k8s.io/kubetest2/pkg/artifacts"
	"sigs.k8s.io/kubetest2/pkg/process"
	"sigs.k8s.io/kubetest2/pkg/testers"
)

const TesterName = "multi"

const usage = `kubetest2 --test=multi -- [MultiTesterDriverArgs] -- [TesterName] [TesterArgs] -- ...

  MultiTesterDriverArgs: arguments passed to the multi-tester driver

  TesterName: the name of the tester to run
  TesterArgs: arguments passed to tester

  Each tester clause is separated by "--".
`

func Main() {
	if err := execute(); err != nil {
		klog.Fatalf("failed to run multi tester: %v", err)
	}
}

type multiTesterDriver struct {
	argv []string
}

type tester struct {
	name string
	path string
	args []string
}

func execute() error {
	driverArgs, testerClauses := splitArguments(os.Args)
	driver := multiTesterDriver{
		argv: driverArgs,
	}
	fs, err := gpflag.Parse(&driver)
	if err != nil {
		return fmt.Errorf("failed to initialize tester: %v", err)
	}

	fs.Usage = func() {
		fmt.Print(usage)
	}

	if len(testerClauses) == 0 {
		fs.Usage()
		return nil
	}

	// gracefully handle -h or --help if it is the only argument
	help := fs.BoolP("help", "h", false, "")

	failFast := fs.Bool("fail-fast", false, "Exit immediately if any tester fails")

	// we don't care about errors, only if -h / --help was set
	err = fs.Parse(driver.argv)
	if err != nil {
		fs.Usage()
		return err
	}

	if *help {
		fs.Usage()
		return nil
	}

	if err := testers.WriteVersionToMetadata(internal.Version); err != nil {
		return err
	}

	if testers, err := prepareTesters(testerClauses); err != nil {
		return err
	} else {
		return test(testers, *failFast)
	}
}

func test(testers []tester, failFast bool) error {
	metadataPath := filepath.Join(artifacts.BaseDir(), "metadata.json")
	backupMetdataPath := metadataPath + ".bak"
	if err := os.Rename(metadataPath, backupMetdataPath); err != nil {
		klog.Errorf("failed to backup driver metadata: %v", err)
	}
	var testerErrs []error
	for _, tester := range testers {
		if err := tester.run(); err != nil {
			klog.Errorf("tester failed: %+v: %v", tester, err)
			testerErrs = append(testerErrs, fmt.Errorf("%+v: %v", tester, err))
			if failFast {
				break
			}
		}
		// reset the metadata.json file
		// testers will try to set the tester-version key and cause conflicts
		if err := os.Remove(metadataPath); err != nil {
			return fmt.Errorf("failed to delete tester metadata: %v", err)
		}
	}
	if err := os.Rename(backupMetdataPath, metadataPath); err != nil {
		return fmt.Errorf("failed to restore driver metadata: %v", err)
	}
	if len(testerErrs) > 0 {
		return errors.Join(testerErrs...)
	}
	return nil
}

// splitArguments splits arguments into driver arguments and tester clauses, separated by "--".
func splitArguments(argv []string) ([]string, [][]string) {
	var clauses [][]string
	var last int
	for i, arg := range argv {
		if arg == "--" {
			clauses = append(clauses, argv[last:i])
			last = i + 1
		}
	}
	clauses = append(clauses, argv[last:])
	return clauses[0], clauses[1:]
}

func prepareTesters(testerClauses [][]string) ([]tester, error) {
	var testers []tester
	for _, clause := range testerClauses {
		testerName := clause[0]
		if testerName == TesterName {
			return nil, fmt.Errorf("nesting isn't possible with the %s tester", TesterName)
		}
		path, err := shim.FindTester(testerName)
		if err != nil {
			return nil, err
		}
		tester := tester{
			name: testerName,
			path: path,
			args: expandEnv(clause[1:]),
		}
		testers = append(testers, tester)
	}
	return testers, nil
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

func (t *tester) run() error {
	klog.Infof("running tester: %+v", t)
	return process.ExecJUnit(t.path, t.args, os.Environ())
}
