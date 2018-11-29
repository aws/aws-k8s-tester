// Package csi implements csi test operations.
package csi

import (
	"fmt"
	"reflect"
	"strings"
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

	permissions, permissionsErr := createPermissions(tester.cfg)
	if permissionsErr != nil {
		return nil, fmt.Errorf("failed to create iamResources (%v)", permissionsErr)
	}
	tester.iam = permissions

	ec, err := ec2.NewDeployer(tester.cfg)
	if err != nil {
		tester.iam.deleteIAMResources()
		return nil, fmt.Errorf("failed to create EC2 deployer (%v)", err)
	}
	tester.ec = ec
	if err = ec.Create(); err != nil {
		tester.iam.deleteIAMResources()
		return nil, fmt.Errorf("failed to create EC2 instance (%v)", err)
	}
	return tester, nil
}

func CreateConfig(vpcID, branchOrPR string) *ec2config.Config {
	cfg := ec2config.NewDefault()
	cfg.UploadTesterLogs = false
	cfg.VPCID = vpcID
	cfg.IngressRulesTCP = map[string]string{"22": "0.0.0.0/0"}
	cfg.Plugins = []string{
		"update-amazon-linux-2",
		"install-go-1.11.2",
		"install-csi-" + branchOrPR,
	}
	cfg.Wait = true
	return cfg
}

func createPermissions(cfg *ec2config.Config) (resources *iamResources, err error) {
	resources, err = createIAMResources(cfg.AWSRegion)
	if err != nil {
		resources.deleteIAMResources()
		return nil, err
	} else {
		cfg.InstanceProfileName = resources.instanceProfile.name
	}
	return resources, nil
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
		Logger:        tester.ec.Logger(),
		KeyPath:       tester.cfg.KeyPath,
		PublicIP:      iv.PublicIP,
		PublicDNSName: iv.PublicDNSName,
		UserName:      tester.cfg.UserName,
	})

	if serr != nil {
		downOrPrintCommands()
		return fmt.Errorf("failed to create SSH (%v)", serr)
	}
	defer sh.Close()

	if err := sh.Connect(); err != nil {
		downOrPrintCommands()
		return fmt.Errorf("failed to connect SSH (%v)", err)
	}

	testCmd := fmt.Sprintf(`cd /home/%s/go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver && sudo sh -c 'source /home/%s/.bashrc && make test-integration'`, tester.cfg.UserName, tester.cfg.UserName)
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
