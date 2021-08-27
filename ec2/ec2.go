// Package ec2 implements testing utilities using EC2.
package ec2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	pkg_aws "github.com/aws/aws-k8s-tester/pkg/aws"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-k8s-tester/version"
	aws_v2 "github.com/aws/aws-sdk-go-v2/aws"
	aws_asg_v2 "github.com/aws/aws-sdk-go-v2/service/autoscaling"
	aws_ec2_v2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	aws_ec2_v2_types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	aws_elbv2_v2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	aws_iam_v2 "github.com/aws/aws-sdk-go-v2/service/iam"
	aws_kms_v2 "github.com/aws/aws-sdk-go-v2/service/kms"
	aws_ssm_v2 "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

// Tester implements "kubetest2" Deployer.
// ref. https://github.com/kubernetes/test-infra/blob/master/kubetest2/pkg/types/types.go
type Tester struct {
	color func(string) string

	stopCreationCh     chan struct{}
	stopCreationChOnce *sync.Once

	osSig chan os.Signal

	downMu *sync.Mutex
	logsMu *sync.RWMutex

	lg        *zap.Logger
	logWriter io.Writer
	logFile   *os.File

	cfg *ec2config.Config

	awsSession *session.Session

	iamAPIV2   *aws_iam_v2.Client
	kmsAPIV2   *aws_kms_v2.Client
	ssmAPIV2   *aws_ssm_v2.Client
	ec2APIV2   *aws_ec2_v2.Client
	s3API      s3iface.S3API
	asgAPIV2   *aws_asg_v2.Client
	elbv2APIV2 *aws_elbv2_v2.Client
}

// New creates a new EC2 tester.
func New(cfg *ec2config.Config) (*Tester, error) {
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		return nil, err
	}

	lg, logWriter, logFile, err := logutil.NewWithStderrWriter(cfg.LogLevel, cfg.LogOutputs)
	if err != nil {
		return nil, err
	}
	lg.Info("set up log writer and file", zap.Strings("outputs", cfg.LogOutputs), zap.Bool("is-color", cfg.LogColor))
	cfg.Sync()

	fmt.Fprint(logWriter, cfg.Colorize("\n\n[yellow]*********************************\n"))
	fmt.Fprintln(logWriter, "😎 🙏 🚶 ✔️ 👍")
	fmt.Fprintf(logWriter, cfg.Colorize("[light_green]New %q [default](%q)\n"), cfg.ConfigPath, version.Version())

	ts := &Tester{
		color:              cfg.Colorize,
		stopCreationCh:     make(chan struct{}),
		stopCreationChOnce: new(sync.Once),
		osSig:              make(chan os.Signal),
		downMu:             new(sync.Mutex),
		logsMu:             new(sync.RWMutex),
		lg:                 lg,
		logWriter:          logWriter,
		logFile:            logFile,
		cfg:                cfg,
	}
	signal.Notify(ts.osSig, syscall.SIGTERM, syscall.SIGINT)

	defer ts.cfg.Sync()

	awsCfg := &pkg_aws.Config{
		Logger:        ts.lg,
		DebugAPICalls: ts.cfg.LogLevel == "debug",
		Partition:     ts.cfg.Partition,
		Region:        ts.cfg.Region,
	}
	var stsOutput *sts.GetCallerIdentityOutput
	ts.awsSession, stsOutput, ts.cfg.AWSCredentialPath, err = pkg_aws.New(awsCfg)
	if err != nil {
		return nil, err
	}
	ts.cfg.AWSAccountID = aws.StringValue(stsOutput.Account)
	ts.cfg.AWSUserID = aws.StringValue(stsOutput.UserId)
	ts.cfg.AWSIAMRoleARN = aws.StringValue(stsOutput.Arn)
	ts.cfg.Sync()

	ts.s3API = s3.New(ts.awsSession)

	awsCfgV2, err := pkg_aws.NewV2(awsCfg)
	if err != nil {
		return nil, err
	}

	ts.iamAPIV2 = aws_iam_v2.NewFromConfig(awsCfgV2)
	ts.kmsAPIV2 = aws_kms_v2.NewFromConfig(awsCfgV2)
	ts.ssmAPIV2 = aws_ssm_v2.NewFromConfig(awsCfgV2)

	ts.ec2APIV2 = aws_ec2_v2.NewFromConfig(awsCfgV2)
	if _, err := ts.ec2APIV2.DescribeInstances(
		context.Background(),
		&aws_ec2_v2.DescribeInstancesInput{MaxResults: aws_v2.Int32(5)}); err != nil {
		return nil, fmt.Errorf("failed to describe instances using EC2 API (%v)", err)
	}
	fmt.Fprintln(ts.logWriter, "EC2 API available!")

	ts.asgAPIV2 = aws_asg_v2.NewFromConfig(awsCfgV2)
	ts.elbv2APIV2 = aws_elbv2_v2.NewFromConfig(awsCfgV2)

	// endpoints package no longer exists in the AWS SDK for Go V2
	// "github.com/aws/aws-sdk-go/aws/endpoints" is deprecated...
	// the check will be done in "eks" with AWS API call
	// ref. https://aws.github.io/aws-sdk-go-v2/docs/migrating/
	fmt.Fprintln(ts.logWriter, "checking region...")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	rout, err := ts.ec2APIV2.DescribeRegions(
		ctx,
		&aws_ec2_v2.DescribeRegionsInput{
			RegionNames: []string{ts.cfg.Region},
			AllRegions:  aws_v2.Bool(false),
		},
	)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("failed to describe region using EC2 API v2 (%v)", err)
	}
	if len(rout.Regions) != 1 {
		return nil, fmt.Errorf("failed to describe region using EC2 API v2 (expected 1, but got %v)", rout.Regions)
	}
	ts.lg.Info("found region",
		zap.String("region-name", aws_v2.ToString(rout.Regions[0].RegionName)),
		zap.String("endpoint", aws_v2.ToString(rout.Regions[0].Endpoint)),
		zap.String("opt-in-status", aws_v2.ToString(rout.Regions[0].OptInStatus)),
	)

	fmt.Fprintln(ts.logWriter, "checking availability zones...")
	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	dout, err := ts.ec2APIV2.DescribeAvailabilityZones(
		ctx,
		&aws_ec2_v2.DescribeAvailabilityZonesInput{
			// TODO: include opt-in zones?
			AllAvailabilityZones: aws_v2.Bool(false),
			Filters: []aws_ec2_v2_types.Filter{
				{
					Name:   aws_v2.String("zone-type"),
					Values: []string{"availability-zone"},
				},
			},
		},
	)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("failed to describe availability zones using EC2 API v2 (%v)", err)
	}
	for _, z := range dout.AvailabilityZones {
		ts.lg.Info("availability zone",
			zap.String("zone-name", aws_v2.ToString(z.ZoneName)),
			zap.String("zone-id", aws_v2.ToString(z.ZoneId)),
			zap.String("zone-type", aws_v2.ToString(z.ZoneType)),
			zap.String("zone-opt-in-status", fmt.Sprintf("%+v", z.OptInStatus)),
		)
		ts.cfg.AvailabilityZoneNames = append(ts.cfg.AvailabilityZoneNames, aws_v2.ToString(z.ZoneName))
	}
	sort.Strings(ts.cfg.AvailabilityZoneNames)
	if len(ts.cfg.AvailabilityZoneNames) > len(ts.cfg.VPC.PublicSubnetCIDRs) {
		ts.cfg.AvailabilityZoneNames = ts.cfg.AvailabilityZoneNames[:len(ts.cfg.VPC.PublicSubnetCIDRs)]
	}
	ts.cfg.Sync()
	if len(ts.cfg.AvailabilityZoneNames) < 2 {
		return nil, fmt.Errorf("too few availability zone %v (expected at least two)", ts.cfg.AvailabilityZoneNames)
	}

	return ts, nil
}

// Up should provision a new cluster for testing
func (ts *Tester) Up() (err error) {
	fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
	fmt.Fprintf(ts.logWriter, ts.color("[light_green]UP START [default](%q)\n"), ts.cfg.ConfigPath)

	now := time.Now()

	defer func() {
		fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_green]UP DEFER START [default](%q)\n"), ts.cfg.ConfigPath)
		ts.logFile.Sync()

		if serr := ts.uploadToS3(); serr != nil {
			ts.lg.Warn("failed to upload artifacts to S3", zap.Error(serr))
		}
		fmt.Fprintf(ts.logWriter, "\n\n# to delete instances\nec2-utils delete instances --path %s\n\n", ts.cfg.ConfigPath)

		if err == nil {
			if ts.cfg.Up {
				if ts.cfg.TotalNodes < 10 {
					fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
					fmt.Fprintf(ts.logWriter, ts.color("[light_green]SSH [default](%q)\n"), ts.cfg.ConfigPath)
					fmt.Fprintln(ts.logWriter, ts.cfg.SSHCommands())
				}

				ts.lg.Info("Up succeeded",
					zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
				)
				fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
				fmt.Fprint(ts.logWriter, ts.color("\n\n💯 😁 👍 :) [light_green]Up success\n\n\n"))

				ts.lg.Sugar().Infof("Up.defer end (%s)", ts.cfg.ConfigPath)
				fmt.Fprintf(ts.logWriter, "\n\n💯 😁 👍 :) Up success\n\n\n")
			} else {
				fmt.Fprintf(ts.logWriter, "\n\n😲 😲 aborted Up ???\n\n\n")
			}
			fmt.Fprintf(ts.logWriter, "\n\n# to delete instances\nec2-utils delete instances --path %s\n\n", ts.cfg.ConfigPath)
			ts.logFile.Sync()
			return
		}

		if !ts.cfg.OnFailureDelete {
			if ts.cfg.Up {
				if ts.cfg.TotalNodes < 10 {
					fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
					fmt.Fprintf(ts.logWriter, ts.color("[light_green]SSH [default](%q)\n"), ts.cfg.ConfigPath)
					fmt.Fprintln(ts.logWriter, ts.cfg.SSHCommands())
				}
			}

			ts.lg.Warn("Up failed",
				zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
				zap.Error(err),
			)
			fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
			fmt.Fprint(ts.logWriter, ts.color("🔥 💀 👽 😱 😡 (-_-) [light_magenta]UP FAIL\n"))

			fmt.Fprintf(ts.logWriter, "\n\n# to delete instances\nec2-utils delete instances --path %s\n\n", ts.cfg.ConfigPath)
			ts.logFile.Sync()
			return
		}

		if ts.cfg.Up {
			if ts.cfg.TotalNodes < 10 {
				fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
				fmt.Fprintf(ts.logWriter, ts.color("[light_green]SSH [default](%q)\n"), ts.cfg.ConfigPath)
				fmt.Fprintln(ts.logWriter, ts.cfg.SSHCommands())
			}
		}
		fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprint(ts.logWriter, ts.color("🔥 💀 👽 😱 😡 (-_-) [light_magenta]UP FAIL\n"))

		ts.lg.Warn("Up failed; reverting resource creation",
			zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			zap.Error(err),
		)
		waitDur := time.Duration(ts.cfg.OnFailureDeleteWaitSeconds) * time.Second
		if waitDur > 0 {
			ts.lg.Info("waiting before clean up", zap.Duration("wait", waitDur))
			select {
			case <-ts.stopCreationCh:
				ts.lg.Info("wait aborted before clean up")
			case <-ts.osSig:
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
		fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprint(ts.logWriter, ts.color("🔥 💀 👽 😱 😡 (-_-) [light_magenta]UP FAIL\n"))

		fmt.Fprintf(ts.logWriter, "\n\n# to delete instances\nec2-utils delete instances --path %s\n\n", ts.cfg.ConfigPath)
		ts.logFile.Sync()
	}()

	ts.lg.Info("Up started",
		zap.String("version", version.Version()),
		zap.String("name", ts.cfg.Name),
	)
	defer ts.cfg.Sync()

	fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
	fmt.Fprintf(ts.logWriter, ts.color("[light_green]createS3 [default](%q)\n"), ts.cfg.ConfigPath)
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.osSig,
		ts.createS3,
	); err != nil {
		return err
	}

	fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
	fmt.Fprintf(ts.logWriter, ts.color("[light_green]createRole [default](%q)\n"), ts.cfg.ConfigPath)
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.osSig,
		ts.createRole,
	); err != nil {
		return err
	}

	fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
	fmt.Fprintf(ts.logWriter, ts.color("[light_green]createVPC [default](%q)\n"), ts.cfg.ConfigPath)
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.osSig,
		ts.createVPC,
	); err != nil {
		return err
	}

	fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
	fmt.Fprintf(ts.logWriter, ts.color("[light_green]createKeyPair [default](%q)\n"), ts.cfg.ConfigPath)
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.osSig,
		ts.createKeyPair,
	); err != nil {
		return err
	}

	fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
	fmt.Fprintf(ts.logWriter, ts.color("[light_green]createASGs [default](%q)\n"), ts.cfg.ConfigPath)
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.osSig,
		ts.createASGs,
	); err != nil {
		return err
	}

	fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
	fmt.Fprintf(ts.logWriter, ts.color("[light_green]createSSM [default](%q)\n"), ts.cfg.ConfigPath)
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.osSig,
		ts.createSSM,
	); err != nil {
		return err
	}

	if ts.cfg.ASGsFetchLogs {
		fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_green]FetchLogs [default](%q)\n"), ts.cfg.ConfigPath)

		waitDur := 20 * time.Second
		ts.lg.Info("sleeping before FetchLogs", zap.Duration("wait", waitDur))
		time.Sleep(waitDur)
		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.osSig,
			ts.FetchLogs,
		); err != nil {
			return err
		}
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
	fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
	fmt.Fprintf(ts.logWriter, ts.color("[light_blue]DOWN START [default](%q)\n"), ts.cfg.ConfigPath)

	now := time.Now()
	ts.lg.Warn("starting Down",
		zap.String("name", ts.cfg.Name),
	)
	if serr := ts.uploadToS3(); serr != nil {
		ts.lg.Warn("failed to upload artifacts to S3", zap.Error(serr))
	}

	defer func() {
		ts.cfg.Sync()
		if err == nil {
			fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
			fmt.Fprintf(ts.logWriter, ts.color("[light_blue]DOWN DEFER START [default](%q)\n"), ts.cfg.ConfigPath)
			fmt.Fprint(ts.logWriter, ts.color("\n\n💯 😁 👍 :)  [light_blue]DOWN SUCCESS\n\n\n"))

			ts.lg.Info("successfully finished Down",
				zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			)

		} else {
			fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
			fmt.Fprintf(ts.logWriter, ts.color("[light_blue]DOWN DEFER START [default](%q)\n"), ts.cfg.ConfigPath)
			fmt.Fprint(ts.logWriter, ts.color("🔥 💀 👽 😱 😡 (-_-) [light_magenta]DOWN FAIL\n"))

			ts.lg.Info("failed Down",
				zap.Error(err),
				zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			)
		}
	}()

	var errs []string

	fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
	fmt.Fprintf(ts.logWriter, ts.color("[light_blue]deleteSSM [default](%q)\n"), ts.cfg.ConfigPath)
	if err := ts.deleteSSM(); err != nil {
		ts.lg.Warn("deleteSSM failed", zap.Error(err))
		errs = append(errs, err.Error())
	}

	fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
	fmt.Fprintf(ts.logWriter, ts.color("[light_blue]deleteASGs [default](%q)\n"), ts.cfg.ConfigPath)
	if err := ts.deleteASGs(); err != nil {
		ts.lg.Warn("deleteASGs failed", zap.Error(err))
		errs = append(errs, err.Error())
	}

	fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
	fmt.Fprintf(ts.logWriter, ts.color("[light_blue]deleteKeyPair [default](%q)\n"), ts.cfg.ConfigPath)
	if err := ts.deleteKeyPair(); err != nil {
		ts.lg.Warn("deleteKeyPair failed", zap.Error(err))
		errs = append(errs, err.Error())
	}

	fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
	fmt.Fprintf(ts.logWriter, ts.color("[light_blue]deleteRole [default](%q)\n"), ts.cfg.ConfigPath)
	if err := ts.deleteRole(); err != nil {
		ts.lg.Warn("deleteRole failed", zap.Error(err))
		errs = append(errs, err.Error())
	}

	if ts.cfg.VPC.Create { // VPC was created
		waitDur := 30 * time.Second
		ts.lg.Info("sleeping before VPC deletion", zap.Duration("wait", waitDur))
		time.Sleep(waitDur)
	}

	fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
	fmt.Fprintf(ts.logWriter, ts.color("[light_blue]deleteVPC [default](%q)\n"), ts.cfg.ConfigPath)
	if err := ts.deleteVPC(); err != nil {
		ts.lg.Warn("deleteVPC failed", zap.Error(err))
		errs = append(errs, err.Error())
	}

	fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
	fmt.Fprintf(ts.logWriter, ts.color("[light_blue]deleteS3 [default](%q)\n"), ts.cfg.ConfigPath)
	if err := ts.deleteS3(); err != nil {
		ts.lg.Warn("deleteS3 failed", zap.Error(err))
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}
	return ts.cfg.Sync()
}

func catchInterrupt(lg *zap.Logger, stopc chan struct{}, stopcCloseOnce *sync.Once, osSigCh chan os.Signal, run func() error) (err error) {
	errc := make(chan error)
	go func() {
		errc <- run()
	}()

	select {
	case _, ok := <-stopc:
		rerr := <-errc
		lg.Info("interrupted; stopc received, errc received", zap.Error(rerr))
		err = fmt.Errorf("stopc returned, stopc open %v, run function returned %v", ok, rerr)

	case osSig := <-osSigCh:
		stopcCloseOnce.Do(func() { close(stopc) })
		rerr := <-errc
		lg.Info("OS signal received, errc received", zap.String("signal", osSig.String()), zap.Error(rerr))
		err = fmt.Errorf("received os signal %v, closed stopc, run function returned %v", osSig, rerr)

	case err = <-errc:
		if err != nil {
			err = fmt.Errorf("run function returned %v", err)
		}
	}
	return err
}
