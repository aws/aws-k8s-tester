// Package csi implements csi test operations.
package csi

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/internal/ec2"
	"github.com/aws/aws-k8s-tester/internal/ssh"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"go.uber.org/zap"
)

const policyDocument = `{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Action": "ec2:*",
			"Effect": "Allow",
			"Resource": "*"
		},
		{
			"Effect": "Allow",
			"Action": "elasticloadbalancing:*",
			"Resource": "*"
		},
		{
			"Effect": "Allow",
			"Action": "cloudwatch:*",
			"Resource": "*"
		},
		{
			"Effect": "Allow",
			"Action": "autoscaling:*",
			"Resource": "*"
		},
		{
			"Effect": "Allow",
			"Action": "iam:CreateServiceLinkedRole",
			"Resource": "*",
			"Condition": {
				"StringEquals": {
					"iam:AWSServiceName": [
						"autoscaling.amazonaws.com",
						"ec2scheduled.amazonaws.com",
						"elasticloadbalancing.amazonaws.com",
						"spot.amazonaws.com",
						"spotfleet.amazonaws.com"
					]
				}
			}
		}
	]
}
`

type csiTester struct {
	cfg *ec2config.Config
	ec  ec2.Deployer

	terminateOnExit bool
	journalctlLogs  bool
}

// Tester defines CSI test interface.
type Tester interface {
	RunIntegration() error
	Down() error
}

// NewTester creates a new tester.
func NewTester(cfg *ec2config.Config, terminateOnExit, journalctlLogs bool) (ct *csiTester, err error) {
	// TODO: some sort of validations check
	ct = &csiTester{
		terminateOnExit: terminateOnExit,
		journalctlLogs:  journalctlLogs,
		cfg:             cfg,
	}

	if ct.ec, err = ec2.NewDeployer(ct.cfg); err != nil {
		return nil, fmt.Errorf("failed to create EC2 deployer (%v)", err)
	}
	if err = ct.ec.Create(); err != nil {
		return nil, fmt.Errorf("failed to create EC2 instance (%v)", err)
	}
	return ct, nil
}

// CreateConfig creates a new configuration.
func CreateConfig(vpcID, prNum, githubAccount, githubBranch string) (cfg *ec2config.Config, err error) {
	cfg = ec2config.NewDefault()
	cfg.UploadTesterLogs = false
	cfg.VPCID = vpcID
	cfg.IngressRulesTCP = map[string]string{"22": "0.0.0.0/0"}
	cfg.Plugins = []string{
		"update-amazon-linux-2",
		"install-go-1.11.4",
	}
	cfg.InstanceProfileFilePath, err = fileutil.WriteTempFile([]byte(policyDocument))
	if err != nil {
		return nil, err
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

func (ct *csiTester) RunIntegration() error {
	lg := ct.ec.Logger()

	downOrPrintCommands := func() {
		if ct.terminateOnExit {
			err := ct.Down()
			if err != nil {
				lg.Warn("failed to take down all resources", zap.Error(err))
			}
		} else {
			fmt.Println(ct.cfg.SSHCommands())
		}
	}

	fmt.Println(ct.cfg.SSHCommands())

	var iv ec2config.Instance
	for _, v := range ct.cfg.Instances {
		iv = v
		break
	}

	sh, serr := ssh.New(ssh.Config{
		Logger:        lg,
		KeyPath:       ct.cfg.KeyPath,
		PublicIP:      iv.PublicIP,
		PublicDNSName: iv.PublicDNSName,
		UserName:      ct.cfg.UserName})

	if serr != nil {
		downOrPrintCommands()
		return fmt.Errorf("failed to create SSH (%v)", serr)
	}
	defer sh.Close()

	if err := sh.Connect(); err != nil {
		downOrPrintCommands()
		return fmt.Errorf("failed to connect SSH (%v)", err)
	}

	testCmd := fmt.Sprintf(`cd /home/%s/go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver && sudo sh -c 'source /home/%s/.bashrc && ./hack/test-integration.sh'`, ct.cfg.UserName, ct.cfg.UserName)
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

	if ct.journalctlLogs {
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

	if ct.terminateOnExit {
		err = ct.Down()
		if err != nil {
			lg.Warn("failed to take down all resources", zap.Error(err))
		}
		os.RemoveAll(ct.cfg.InstanceProfileFilePath)
	}

	return nil
}

func (ct *csiTester) Down() error {
	return ct.ec.Terminate()
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
