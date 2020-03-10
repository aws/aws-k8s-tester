// Package ec2 implements testing utilities using EC2.
package ec2

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	pkgaws "github.com/aws/aws-k8s-tester/pkg/aws"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-k8s-tester/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/kms/kmsiface"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/dustin/go-humanize"
	"github.com/mitchellh/colorstring"
	"go.uber.org/zap"
)

// Tester implements "kubetest2" Deployer.
// ref. https://github.com/kubernetes/test-infra/blob/master/kubetest2/pkg/types/types.go
type Tester struct {
	stopCreationCh     chan struct{}
	stopCreationChOnce *sync.Once

	interruptSig chan os.Signal

	downMu *sync.Mutex
	logsMu *sync.RWMutex

	lg  *zap.Logger
	cfg *ec2config.Config

	awsSession *session.Session
	iamAPI     iamiface.IAMAPI
	kmsAPI     kmsiface.KMSAPI
	ssmAPI     ssmiface.SSMAPI
	cfnAPI     cloudformationiface.CloudFormationAPI
	ec2API     ec2iface.EC2API
	s3API      s3iface.S3API
	asgAPI     autoscalingiface.AutoScalingAPI
	elbv2API   elbv2iface.ELBV2API
}

// New creates a new EC2 tester.
func New(cfg *ec2config.Config) (*Tester, error) {
	fmt.Println("😎 🙏")
	fmt.Println(version.Version())
	colorstring.Printf("\n\n\n[light_green]New [default](%q)\n", cfg.ConfigPath)
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		return nil, err
	}

	lcfg := logutil.AddOutputPaths(logutil.DefaultZapLoggerConfig, cfg.LogOutputs, cfg.LogOutputs)
	lcfg.Level = zap.NewAtomicLevelAt(logutil.ConvertToZapLevel(cfg.LogLevel))
	lg, err := lcfg.Build()
	if err != nil {
		return nil, err
	}

	ts := &Tester{
		stopCreationCh:     make(chan struct{}),
		stopCreationChOnce: new(sync.Once),
		interruptSig:       make(chan os.Signal),
		downMu:             new(sync.Mutex),
		logsMu:             new(sync.RWMutex),
		lg:                 lg,
		cfg:                cfg,
	}
	signal.Notify(ts.interruptSig, syscall.SIGTERM, syscall.SIGINT)

	defer ts.cfg.Sync()

	awsCfg := &pkgaws.Config{
		Logger:        ts.lg,
		DebugAPICalls: ts.cfg.LogLevel == "debug",
		Region:        ts.cfg.Region,
	}
	var stsOutput *sts.GetCallerIdentityOutput
	ts.awsSession, stsOutput, ts.cfg.AWSCredentialPath, err = pkgaws.New(awsCfg)
	if err != nil {
		return nil, err
	}
	ts.cfg.AWSAccountID = aws.StringValue(stsOutput.Account)
	ts.cfg.AWSUserID = aws.StringValue(stsOutput.UserId)
	ts.cfg.AWSIAMRoleARN = aws.StringValue(stsOutput.Arn)
	ts.cfg.Sync()

	ts.iamAPI = iam.New(ts.awsSession)
	ts.kmsAPI = kms.New(ts.awsSession)
	ts.ssmAPI = ssm.New(ts.awsSession)
	ts.cfnAPI = cloudformation.New(ts.awsSession)

	ts.ec2API = ec2.New(ts.awsSession)
	if _, err := ts.ec2API.DescribeInstances(&ec2.DescribeInstancesInput{MaxResults: aws.Int64(5)}); err != nil {
		return nil, fmt.Errorf("failed to describe instances using EC2 API (%v)", err)
	}
	colorstring.Println("[green]EC2 API available!")

	ts.s3API = s3.New(ts.awsSession)
	ts.asgAPI = autoscaling.New(ts.awsSession)
	ts.elbv2API = elbv2.New(ts.awsSession)

	return ts, nil
}

// Up should provision a new cluster for testing
func (ts *Tester) Up() (err error) {
	colorstring.Printf("\n\n\n[light_green]Up [default](%q)\n", ts.cfg.ConfigPath)

	now := time.Now()

	defer func() {
		colorstring.Printf("\n\n\n[light_green]Up.defer start [default](%q)\n\n", ts.cfg.ConfigPath)

		if err == nil {
			if ts.cfg.Up {
				colorstring.Printf("\n\n[light_green]SSH [default](%q)\n\n", ts.cfg.ConfigPath)
				fmt.Println(ts.cfg.SSHCommands())

				ts.lg.Info("Up succeeded",
					zap.String("request-started", humanize.RelTime(now, time.Now(), "ago", "from now")),
				)

				colorstring.Printf("\n\n\n[light_green]Up.defer end [default](%q)\n\n", ts.cfg.ConfigPath)
				colorstring.Printf("\n\n😁 😁 [blue]:) [default]Up success\n\n\n")
			} else {
				colorstring.Printf("\n\n😲 😲 [yellow]aborted [default]Up ???\n\n\n")
			}
			return
		}

		if !ts.cfg.OnFailureDelete {
			if ts.cfg.Up {
				colorstring.Printf("\n\n[light_green]SSH [default](%q)\n\n", ts.cfg.ConfigPath)
				fmt.Println(ts.cfg.SSHCommands())
			}

			ts.lg.Warn("Up failed",
				zap.String("request-started", humanize.RelTime(now, time.Now(), "ago", "from now")),
				zap.Error(err),
			)

			colorstring.Printf("\n\n\n[light_red]Up.defer end [default](%q)\n\n", ts.cfg.ConfigPath)
			colorstring.Printf("\n\n😱 ☹ 😡 [light_red](-_-) [default]Up fail\n\n\n")
			return
		}

		if ts.cfg.Up {
			colorstring.Printf("\n\n[light_green]SSH [default](%q)\n\n", ts.cfg.ConfigPath)
			fmt.Println(ts.cfg.SSHCommands())
		}

		ts.lg.Warn("Up failed; reverting resource creation",
			zap.String("request-started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			zap.Error(err),
		)
		waitDur := time.Duration(ts.cfg.OnFailureDeleteWaitSeconds) * time.Second
		if waitDur > 0 {
			ts.lg.Info("waiting before clean up", zap.Duration("wait", waitDur))
			select {
			case <-ts.stopCreationCh:
				ts.lg.Info("wait aborted before clean up")
			case <-ts.interruptSig:
				ts.lg.Info("wait aborted before clean up")
			case <-time.After(waitDur):
			}
		}
		derr := ts.down()
		if derr != nil {
			ts.lg.Warn("failed to revert Up", zap.Error(derr))
		} else {
			ts.lg.Warn("reverted Up")
		}

		colorstring.Printf("\n\n\n[light_red]Up.defer end [default](%q)\n\n", ts.cfg.ConfigPath)
		colorstring.Printf("\n\n😱 ☹ 😡 [light_red](-_-) [default]Up fail\n\n\n")
	}()

	ts.lg.Info("Up started",
		zap.String("version", version.Version()),
		zap.String("name", ts.cfg.Name),
	)
	defer ts.cfg.Sync()

	colorstring.Printf("\n\n\n[light_green]createRole [default](%q)\n", ts.cfg.ConfigPath)
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.interruptSig,
		ts.createRole,
	); err != nil {
		return err
	}

	colorstring.Printf("\n\n\n[light_green]createVPC [default](%q)\n", ts.cfg.ConfigPath)
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.interruptSig,
		ts.createVPC,
	); err != nil {
		return err
	}

	colorstring.Printf("\n\n\n[light_green]createKeyPair [default](%q)\n", ts.cfg.ConfigPath)
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.interruptSig,
		ts.createKeyPair,
	); err != nil {
		return err
	}

	colorstring.Printf("\n\n\n[light_green]createASGs [default](%q, %q)\n", ts.cfg.ConfigPath)
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.interruptSig,
		ts.createASGs,
	); err != nil {
		return err
	}

	colorstring.Printf("\n\n\n[light_green]FetchLogs [default](%q)\n", ts.cfg.ConfigPath)
	waitDur := 20 * time.Second
	ts.lg.Info("sleeping before FetchLogs", zap.Duration("wait", waitDur))
	time.Sleep(waitDur)
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.interruptSig,
		ts.FetchLogs,
	); err != nil {
		return err
	}

	return ts.cfg.Sync()
}

// Down cancels the cluster creation and destroy the test cluster if any.
func (ts *Tester) Down() error {
	ts.downMu.Lock()
	defer ts.downMu.Unlock()
	return ts.down()
}

func (ts *Tester) down() (err error) {
	colorstring.Printf("\n\n\n[light_green]Down start [default](%q)\n\n", ts.cfg.ConfigPath)

	now := time.Now()
	ts.lg.Warn("starting Down",
		zap.String("name", ts.cfg.Name),
	)

	defer func() {
		ts.cfg.Sync()
		if err == nil {
			colorstring.Printf("\n\n\n[light_green]Down.defer end [default](%q)\n\n", ts.cfg.ConfigPath)
			colorstring.Printf("\n\n😁 😁 [blue]:) [default]Down success\n\n\n")

			ts.lg.Info("successfully finished Down",
				zap.String("request-started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			)

		} else {
			colorstring.Printf("\n\n\n[light_red]Down.defer end [default](%q)\n\n", ts.cfg.ConfigPath)
			colorstring.Printf("\n\n😱 ☹ 😡 [light_red](-_-) [default]Down fail\n\n\n")

			ts.lg.Info("failed Down",
				zap.Error(err),
				zap.String("request-started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			)
		}
	}()

	var errs []string

	colorstring.Printf("\n\n\n[light_green]deleteASGs [default](%q)\n", ts.cfg.ConfigPath)
	if err := ts.deleteASGs(); err != nil {
		ts.lg.Warn("deleteASGs failed", zap.Error(err))
		errs = append(errs, err.Error())
	}

	colorstring.Printf("\n\n\n[light_green]deleteKeyPair [default](%q)\n", ts.cfg.ConfigPath)
	if err := ts.deleteKeyPair(); err != nil {
		ts.lg.Warn("deleteKeyPair failed", zap.Error(err))
		errs = append(errs, err.Error())
	}

	colorstring.Printf("\n\n\n[light_green]deleteRole [default](%q)\n", ts.cfg.ConfigPath)
	if err := ts.deleteRole(); err != nil {
		ts.lg.Warn("deleteRole failed", zap.Error(err))
		errs = append(errs, err.Error())
	}

	if ts.cfg.VPCCreate { // VPC was created
		waitDur := 30 * time.Second
		ts.lg.Info("sleeping before VPC deletion", zap.Duration("wait", waitDur))
		time.Sleep(waitDur)
	}

	colorstring.Printf("\n\n\n[light_green]deleteVPC [default](%q)\n", ts.cfg.ConfigPath)
	if err := ts.deleteVPC(); err != nil {
		ts.lg.Warn("deleteVPC failed", zap.Error(err))
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}
	return ts.cfg.Sync()
}
