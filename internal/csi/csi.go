// Package csi implements csi test operations.
package csi

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/internal/ec2"
	"github.com/aws/aws-k8s-tester/internal/ssh"
	"go.uber.org/zap"
)

type Tester struct {
	cfg *ec2config.Config
	iam *iamResources
	ec  ec2.Deployer

	terminateOnExit bool
	journalctlLogs  bool
}

func NewTester(cfg *ec2config.Config, terminateOnExit, journalctlLogs bool) (tester *Tester, err error) {
	// TODO: some sort of validations check
	tester = &Tester{
		terminateOnExit: terminateOnExit,
		journalctlLogs:  journalctlLogs,
		cfg:             cfg,
	}

	if tester.iam, err = createIAMResources(cfg.AWSRegion); err != nil {
		return nil, fmt.Errorf("failed to create iamResources (%v)", err)
	}
	cfg.InstanceProfileName = tester.iam.instanceProfile.name

	defer func() {
		if err != nil {
			if deleteErr := tester.iam.deleteIAMResources(); deleteErr != nil {
				tester.iam.lg.Error("failed to delete all IAM resources", zap.Error(deleteErr))
			}
		}
	}()

	if tester.ec, err = ec2.NewDeployer(tester.cfg); err != nil {
		return nil, fmt.Errorf("failed to create EC2 deployer (%v)", err)
	}
	if err = tester.ec.Create(); err != nil {
		return nil, fmt.Errorf("failed to create EC2 instance (%v)", err)
	}
	return tester, nil
}

func CreateConfig(vpcID, prNum, githubAccount, githubBranch string) (cfg *ec2config.Config, err error) {
	cfg = ec2config.NewDefault()
	cfg.UploadTesterLogs = false
	cfg.VPCID = vpcID
	cfg.IngressRulesTCP = map[string]string{"22": "0.0.0.0/0"}
	cfg.Plugins = []string{
		"update-amazon-linux-2",
		"install-go-1.11.4",
	}
	// If prNum is set to an non-empty string, the master branch of kubernetes-sigs/aws-ebs-csi-driver is used,
	// regardless of whether or not githubAccount and githubBranch have different values.
	if prNum != "" {
		if githubAccount != "kubernetes-sigs" || githubBranch != "master" {
			fmt.Printf("WARNING: PR number %s takes precedence over non-default GitHub account and/or branch.\n", prNum)
		}
		cfg.Plugins = append(cfg.Plugins, "install-csi-"+prNum)
	} else {
		cfg.CustomScript, err = createCustomScript(gitAccountAndBranch{
			Account: githubAccount,
			Branch:  githubBranch,
		})
		if err != nil {
			return nil, err
		}
	}
	cfg.Wait = true
	return cfg, nil
}

func (tester *Tester) RunCSIIntegrationTest() error {
	lg := tester.ec.Logger()

	downOrPrintCommands := func() {
		if tester.terminateOnExit {
			err := tester.Down()
			if err != nil {
				lg.Warn("failed to take down all resources", zap.Error(err))
			}
		} else {
			fmt.Println(tester.cfg.SSHCommands())
			fmt.Println(tester.iam.getManualDeleteCommands())
		}
	}

	fmt.Println(tester.cfg.SSHCommands())
	fmt.Println(tester.iam.getManualDeleteCommands())

	var iv ec2config.Instance
	for _, v := range tester.cfg.Instances {
		iv = v
		break
	}

	sh, serr := ssh.New(ssh.Config{
		Logger:        lg,
		KeyPath:       tester.cfg.KeyPath,
		PublicIP:      iv.PublicIP,
		PublicDNSName: iv.PublicDNSName,
		UserName:      tester.cfg.UserName})

	if serr != nil {
		downOrPrintCommands()
		return fmt.Errorf("failed to create SSH (%v)", serr)
	}
	defer sh.Close()

	if err := sh.Connect(); err != nil {
		downOrPrintCommands()
		return fmt.Errorf("failed to connect SSH (%v)", err)
	}

	testCmd := fmt.Sprintf(`cd /home/%s/go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver && sudo sh -c 'source /home/%s/.bashrc && ./hack/test-integration.sh'`, tester.cfg.UserName, tester.cfg.UserName)
	out, err := sh.Run(
		testCmd,
		ssh.WithTimeout(10*time.Minute),
	)
	if err != nil {
		// handle "Process exited with status 2" error
		downOrPrintCommands()
		return fmt.Errorf("CSI integration test FAILED (%v, %v)", err, reflect.TypeOf(err))
	}

	testOutput := string(out)
	fmt.Printf("CSI test output:\n\n%s\n\n", testOutput)

	/*
	   expects

	   Ran 1 of 1 Specs in 25.028 seconds
	   SUCCESS! -- 1 Passed | 0 Failed | 0 Pending | 0 Skipped
	*/
	if !strings.Contains(testOutput, "1 Passed") {
		downOrPrintCommands()
		return fmt.Errorf("CSI integration test FAILED")
	}

	if tester.journalctlLogs {
		// full journal logs (e.g. disk mounts)
		lg.Info("fetching journal logs")
		journalCmd := "sudo journalctl --no-pager --output=short-precise"
		out, err = sh.Run(journalCmd)
		if err != nil {
			lg.Warn(
				"failed to run journalctl",
				zap.String("cmd", journalCmd),
				zap.Error(err),
			)
		} else {
			fmt.Printf("journalctl logs:\n\n%s\n\n", string(out))
		}
	}

	if tester.terminateOnExit {
		err = tester.Down()
		if err != nil {
			lg.Warn("failed to take down all resources", zap.Error(err))
		}
	}

	return nil
}

func (tester *Tester) Down() error {
	tester.ec.Terminate()
	return tester.iam.deleteIAMResources()
}

type gitAccountAndBranch struct {
	Account string
	Branch  string
}

func createCustomScript(g gitAccountAndBranch) (string, error) {
	tpl := template.Must(template.New("installGitAccountAndBranchTemplate").Parse(installGitAccountAndBranchTemplate))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, g); err != nil {
		return "", err
	}
	return buf.String(), nil
}

const installGitAccountAndBranchTemplate = `

################################## install kubernetes-sigs from account {{ .Account }}, branch {{ .Branch }}

mkdir -p ${GOPATH}/src/github.com/kubernetes-sigs/
cd ${GOPATH}/src/github.com/kubernetes-sigs/

RETRIES=10
DELAY=10
COUNT=1
while [[ ${COUNT} -lt ${RETRIES} ]]; do
  rm -rf ./aws-ebs-csi-driver
  git clone "https://github.com/{{ .Account }}/aws-ebs-csi-driver.git"
  if [[ $? -eq 0 ]]; then
    RETRIES=0
    echo "Successfully git cloned!"
    break
  fi
  let COUNT=${COUNT}+1
  sleep ${DELAY}
done

cd ${GOPATH}/src/github.com/kubernetes-sigs/aws-ebs-csi-driver

git checkout origin/{{ .Branch }}
git checkout -B {{ .Branch }}

git remote -v
git branch
git log --pretty=oneline -5

pwd
make aws-ebs-csi-driver && sudo cp ./bin/aws-ebs-csi-driver /usr/local/bin/aws-ebs-csi-driver

##################################

`
