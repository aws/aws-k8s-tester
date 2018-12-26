// Package ec2 implements testing utilities using EC2.
package ec2

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/ec2config/plugins"
	"github.com/aws/aws-k8s-tester/internal/ssh"
	"github.com/aws/aws-k8s-tester/pkg/awsapi"
	"github.com/aws/aws-k8s-tester/pkg/zaputil"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

// Deployer defines EC2 deployer.
type Deployer interface {
	// Create creates a cluster of EC2 instances.
	Create() error
	// Add creates one more instance to the cluster.
	Add() error
	// Stop stops create operation.å
	Stop()
	// Delete deletes one instance.
	Delete(id string) error
	// Terminate terminates all EC2 instances in the cluster.
	Terminate() error
	// Logger returns the logger.
	Logger() *zap.Logger
	// UploadToBucketForTests uploads a local file to aws-k8s-tester S3 bucket.
	UploadToBucketForTests(localPath, remotePath string) error
}

type embedded struct {
	stopc chan struct{}

	mu  sync.RWMutex
	lg  *zap.Logger
	cfg *ec2config.Config

	ss  *session.Session
	sts stsiface.STSAPI
	cf  cloudformationiface.CloudFormationAPI
	ec2 ec2iface.EC2API

	s3        s3iface.S3API
	s3Buckets map[string]struct{}
}

// TODO: use cloudformation, ASG

// NewDeployer creates a new EKS deployer.
func NewDeployer(cfg *ec2config.Config) (Deployer, error) {
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	lg, err := zaputil.New(cfg.LogDebug, cfg.LogOutputs)
	if err != nil {
		return nil, err
	}

	md := &embedded{
		stopc:     make(chan struct{}),
		lg:        lg,
		cfg:       cfg,
		s3Buckets: make(map[string]struct{}),
	}

	awsCfg := &awsapi.Config{
		Logger:         md.lg,
		DebugAPICalls:  cfg.LogDebug,
		Region:         cfg.AWSRegion,
		CustomEndpoint: "",
	}
	md.ss, err = awsapi.New(awsCfg)
	if err != nil {
		return nil, err
	}

	md.sts = sts.New(md.ss)
	md.cf = cloudformation.New(md.ss)
	md.ec2 = ec2.New(md.ss)
	md.s3 = s3.New(md.ss)

	output, oerr := md.sts.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if oerr != nil {
		return nil, oerr
	}
	md.cfg.AWSAccountID = *output.Account

	// up to 63 characters
	// https://docs.aws.amazon.com/awscloudtrail/latest/userguide/cloudtrail-s3-bucket-naming-requirements.html
	md.cfg.Tag += "-" + strings.ToLower(*output.UserId)
	h, _ := os.Hostname()
	if len(h) > 5 {
		h = strings.ToLower(h)
		h = strings.Replace(h, ".", "", -1)
		h = strings.Replace(h, "-", "", -1)
		h = strings.Replace(h, "_", "", -1)
		md.cfg.Tag += h
	}
	if len(md.cfg.Tag) > 42 {
		md.cfg.Tag = md.cfg.Tag[:42]
	}
	md.cfg.ConfigPathURL = genS3URL(md.cfg.AWSRegion, md.cfg.Tag, md.cfg.ConfigPathBucket)
	md.cfg.LogOutputToUploadPathURL = genS3URL(md.cfg.AWSRegion, md.cfg.Tag, md.cfg.LogOutputToUploadPathBucket)
	md.cfg.KeyPathURL = genS3URL(md.cfg.AWSRegion, md.cfg.Tag, md.cfg.KeyPathBucket)

	lg.Info(
		"created EC2 deployer",
		zap.String("id", cfg.ClusterName),
		zap.String("aws-k8s-tester-ec2config-path", cfg.ConfigPath),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)
	return md, md.cfg.Sync()
}

func (md *embedded) Create() (err error) {
	md.mu.Lock()
	defer md.mu.Unlock()

	now := time.Now().UTC()
	md.lg.Info("creating", zap.String("cluster-name", md.cfg.ClusterName))

	defer func() {
		if err != nil {
			md.lg.Warn("reverting EC2 creation", zap.Error(err))
			if derr := md.deleteInstances(); derr != nil {
				md.lg.Warn("failed to revert instance creation", zap.Error(derr))
			}
			if derr := md.deleteSecurityGroup(); derr != nil {
				md.lg.Warn("failed to revert security group creation", zap.Error(derr))
			}
			if md.cfg.VPCCreated {
				if derr := md.deleteSubnet(); derr != nil {
					md.lg.Warn("failed to revert subnet creation", zap.Error(derr))
				}
				if derr := md.deleteVPC(); derr != nil {
					md.lg.Warn("failed to revert VPC creation", zap.Error(derr))
				}
			}
			if derr := md.deleteKeyPair(); derr != nil {
				md.lg.Warn("failed to revert key pair creation", zap.Error(derr))
			}
		}
	}()
	defer md.cfg.Sync()

	if err = catchStopc(md.lg, md.stopc, md.createKeyPair); err != nil {
		return err
	}
	if md.cfg.VPCID != "" { // use existing VPC
		if len(md.cfg.SubnetIDs) == 0 {
			if err = catchStopc(md.lg, md.stopc, md.getSubnets); err != nil {
				return err
			}
		} else {
			// otherwise, use specified subnet ID
			var output *ec2.DescribeSubnetsOutput
			output, err = md.ec2.DescribeSubnets(&ec2.DescribeSubnetsInput{
				SubnetIds: aws.StringSlice(md.cfg.SubnetIDs),
			})
			if err != nil {
				return err
			}
			if md.cfg.SubnetIDToAvailabilityZone == nil {
				md.cfg.SubnetIDToAvailabilityZone = make(map[string]string)
			}
			for _, sv := range output.Subnets {
				md.cfg.SubnetIDToAvailabilityZone[*sv.SubnetId] = *sv.AvailabilityZone
			}
		}

		if err = md.associatePublicIP(); err != nil {
			return err
		}

		var do *ec2.DescribeVpcsOutput
		do, err = md.ec2.DescribeVpcs(&ec2.DescribeVpcsInput{
			VpcIds: aws.StringSlice([]string{md.cfg.VPCID}),
		})
		if err != nil {
			md.lg.Warn("failed to describe VPC", zap.String("vpc-id", md.cfg.VPCID), zap.Error(err))
		} else {
			md.cfg.VPCCIDR = *do.Vpcs[0].CidrBlock
		}
		md.lg.Info(
			"found subnets",
			zap.String("vpc-id", md.cfg.VPCID),
			zap.String("vpc-cidr", md.cfg.VPCCIDR),
			zap.Strings("subnet-ids", md.cfg.SubnetIDs),
			zap.String("availability-zones", fmt.Sprintf("%v", md.cfg.SubnetIDToAvailabilityZone)),
		)
	} else {
		if err = catchStopc(md.lg, md.stopc, md.createVPC); err != nil {
			return err
		}
		md.cfg.VPCCreated = true
		md.cfg.Sync()
		if err = catchStopc(md.lg, md.stopc, md.createSubnets); err != nil {
			return err
		}
	}
	if err = catchStopc(md.lg, md.stopc, md.createSecurityGroup); err != nil {
		return err
	}
	if err = catchStopc(md.lg, md.stopc, md.createInstances); err != nil {
		return err
	}

	md.lg.Info("created",
		zap.String("cluster-name", md.cfg.ClusterName),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)

	if err = md.cfg.Sync(); err != nil {
		return err
	}
	if md.cfg.UploadTesterLogs {
		if err = md.uploadTesterLogs(); err != nil {
			md.lg.Warn("failed to upload", zap.Error(err))
		}
	}

	return nil
}

func (md *embedded) Add() (err error) {
	md.mu.Lock()
	defer md.mu.Unlock()

	if md.cfg.KeyName == "" {
		return errors.New("cannot add without key name")
	}
	if len(md.cfg.SubnetIDs) == 0 {
		return errors.New("cannot add without SubnetIDs")
	}
	if len(md.cfg.SecurityGroupIDs) == 0 {
		return errors.New("cannot add without SecurityGroupIDs")
	}

	instanceProfileSpecifics := &ec2.IamInstanceProfileSpecification{Name: &md.cfg.InstanceProfileName}

	now := time.Now().UTC()
	md.lg.Info("creating one EC2 instance", zap.String("cluster-name", md.cfg.ClusterName))

	tkn := md.cfg.ClusterName + fmt.Sprintf("%X", time.Now().Nanosecond())
	_, err = md.ec2.RunInstances(&ec2.RunInstancesInput{
		ClientToken:                       aws.String(tkn),
		ImageId:                           aws.String(md.cfg.ImageID),
		MinCount:                          aws.Int64(1),
		MaxCount:                          aws.Int64(1),
		InstanceType:                      aws.String(md.cfg.InstanceType),
		KeyName:                           aws.String(md.cfg.KeyName),
		SubnetId:                          aws.String(md.cfg.SubnetIDs[0]),
		SecurityGroupIds:                  aws.StringSlice(md.cfg.SecurityGroupIDs),
		InstanceInitiatedShutdownBehavior: aws.String("terminate"),
		UserData:                          aws.String(base64.StdEncoding.EncodeToString([]byte(md.cfg.InitScript))),
		IamInstanceProfile:                instanceProfileSpecifics,
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String("instance"),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(md.cfg.ClusterName),
					},
				},
			},
		},
	})
	if err != nil {
		return err
	}

	ready, iv := false, ec2config.Instance{}
	retryStart := time.Now().UTC()
done:
	for time.Now().UTC().Sub(retryStart) < 5*time.Minute {
		var output *ec2.DescribeInstancesOutput
		output, err = md.ec2.DescribeInstances(&ec2.DescribeInstancesInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("client-token"),
					Values: aws.StringSlice([]string{tkn}),
				},
			},
		})
		if err != nil {
			md.lg.Warn("failed to describe instances", zap.Error(err))
			time.Sleep(3 * time.Second)
			continue
		}

		for _, rv := range output.Reservations {
			for _, inst := range rv.Instances {
				id := *inst.InstanceId
				if *inst.State.Name == "running" {
					_, ok := md.cfg.Instances[id]
					if !ok {
						iv = ConvertEC2Instance(inst)
						md.cfg.Instances[id] = iv
						md.cfg.ClusterSize++
						md.lg.Info("instance is ready",
							zap.String("instance-id", iv.InstanceID),
							zap.String("instance-public-ip", iv.PublicIP),
							zap.String("instance-private-ip", iv.PrivateIP),
							zap.String("instance-public-dns", iv.PublicDNSName),
							zap.String("instance-private-dns", iv.PrivateDNSName),
						)
						ready = true
						break done
					}
				}
			}
		}
		time.Sleep(5 * time.Second)
	}
	if ready {
		md.lg.Info("created one EC2 instance",
			zap.String("cluster-name", md.cfg.ClusterName),
			zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
		)
	} else {
		md.lg.Warn("created one EC2 instance but not ready",
			zap.String("cluster-name", md.cfg.ClusterName),
			zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
		)
	}

	mm := make(map[string]ec2config.Instance, 1)
	mm[iv.InstanceID] = iv
	md.wait(mm)

	return md.cfg.Sync()
}

func (md *embedded) Stop() { close(md.stopc) }

func (md *embedded) Delete(id string) (err error) {
	md.mu.Lock()
	defer md.mu.Unlock()

	if len(md.cfg.Instances) != md.cfg.ClusterSize {
		return fmt.Errorf("len(Instances) %d != ClusterSize %d", len(md.cfg.Instances), md.cfg.ClusterSize)
	}

	now := time.Now().UTC()
	md.lg.Info("deleting an instance", zap.String("cluster-name", md.cfg.ClusterName), zap.String("instance-id", id))

	_, ok := md.cfg.Instances[id]
	if !ok {
		return fmt.Errorf("failed to delete an instance, id %q not found", id)
	}
	_, err = md.ec2.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: aws.StringSlice([]string{id}),
	})
	if err != nil {
		return fmt.Errorf("failed to delete an instance (%v)", id)
	}

	retryStart := time.Now().UTC()
done:
	for time.Now().UTC().Sub(retryStart) < 3*time.Minute {
		var output *ec2.DescribeInstancesOutput
		output, err = md.ec2.DescribeInstances(&ec2.DescribeInstancesInput{
			InstanceIds: aws.StringSlice([]string{id}),
		})
		if err != nil {
			return err
		}

		for _, rv := range output.Reservations {
			for _, inst := range rv.Instances {
				if *inst.InstanceId != id {
					return fmt.Errorf("unexpected instance id %q, expected %q", *inst.InstanceId, id)
				}
				if *inst.State.Name == "terminated" {
					break done
				}
				md.lg.Info("deleting an instance", zap.String("instance-id", id), zap.String("state", *inst.State.Name))
			}
		}

		time.Sleep(5 * time.Second)
	}

	prev := len(md.cfg.Instances)
	delete(md.cfg.Instances, id)
	md.cfg.ClusterSize--
	md.lg.Info(
		"deleted an instance",
		zap.String("cluster-name", md.cfg.ClusterName),
		zap.String("instance-id", id),
		zap.Int("previous-cluster-size", prev),
		zap.Int("current-cluster-size", len(md.cfg.Instances)),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)
	return nil
}

func (md *embedded) deleteInstances() (err error) {
	now := time.Now().UTC()

	ids := make([]string, 0, len(md.cfg.Instances))
	for id := range md.cfg.Instances {
		ids = append(ids, id)
	}
	_, err = md.ec2.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: aws.StringSlice(ids),
	})
	md.lg.Info("terminating", zap.Strings("instance-ids", ids), zap.Error(err))

	sleepDur := 5 * time.Second * time.Duration(md.cfg.ClusterSize)
	if sleepDur > 3*time.Minute {
		sleepDur = 3 * time.Minute
	}
	time.Sleep(sleepDur)

	retryStart := time.Now().UTC()
	terminated := make(map[string]struct{})
	for len(terminated) != md.cfg.ClusterSize &&
		time.Now().UTC().Sub(retryStart) < time.Duration(md.cfg.ClusterSize)*2*time.Minute {
		var output *ec2.DescribeInstancesOutput
		output, err = md.ec2.DescribeInstances(&ec2.DescribeInstancesInput{
			InstanceIds: aws.StringSlice(ids),
		})
		if err != nil {
			return err
		}
		for _, rv := range output.Reservations {
			for _, inst := range rv.Instances {
				id := *inst.InstanceId
				if _, ok := terminated[id]; ok {
					continue
				}
				if *inst.State.Name == "terminated" {
					terminated[id] = struct{}{}
					md.lg.Info("terminated", zap.String("instance-id", id))
				}
			}
		}
		time.Sleep(5 * time.Second)
	}

	md.lg.Info("terminated",
		zap.Strings("instance-ids", ids),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)
	return nil
}

func (md *embedded) Terminate() (err error) {
	md.mu.Lock()
	defer md.mu.Unlock()

	now := time.Now().UTC()
	md.lg.Info("deleting", zap.String("cluster-name", md.cfg.ClusterName))

	var errs []string
	if err = md.deleteInstances(); err != nil {
		md.lg.Warn("failed to delete instances", zap.Error(err))
		errs = append(errs, err.Error())
	}
	if err = md.deleteSecurityGroup(); err != nil {
		md.lg.Warn("failed to delete security group", zap.Error(err))
		errs = append(errs, err.Error())
	}
	if md.cfg.VPCCreated {
		if err = md.deleteSubnet(); err != nil {
			md.lg.Warn("failed to delete subnet", zap.Error(err))
			errs = append(errs, err.Error())
		}
		if err = md.deleteVPC(); err != nil {
			md.lg.Warn("failed to delete VPC", zap.Error(err))
			errs = append(errs, err.Error())
		}
	}
	if err = md.deleteKeyPair(); err != nil {
		md.lg.Warn("failed to delete key pair", zap.Error(err))
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	md.lg.Info("deleted",
		zap.String("cluster-name", md.cfg.ClusterName),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)

	if err = md.cfg.Sync(); err != nil {
		return err
	}
	if md.cfg.UploadTesterLogs {
		if err = md.uploadTesterLogs(); err != nil {
			md.lg.Warn("failed to upload", zap.Error(err))
		}
	}

	return nil
}

func (md *embedded) uploadTesterLogs() (err error) {
	if err = md.UploadToBucketForTests(
		md.cfg.ConfigPath,
		md.cfg.ConfigPathBucket,
	); err != nil {
		return err
	}
	if err = md.UploadToBucketForTests(
		md.cfg.LogOutputToUploadPath,
		md.cfg.LogOutputToUploadPathBucket,
	); err != nil {
		return err
	}
	return md.UploadToBucketForTests(
		md.cfg.KeyPath,
		md.cfg.KeyPathBucket,
	)
}

func (md *embedded) createInstances() (err error) {
	now := time.Now().UTC()

	instanceProfileSpecifics := &ec2.IamInstanceProfileSpecification{Name: &md.cfg.InstanceProfileName}

	// evenly distribute per subnet
	left := md.cfg.ClusterSize

	tokens := []string{}
	tknToCnt := make(map[string]int)

	if md.cfg.ClusterSize > len(md.cfg.SubnetIDs) {
		// TODO: configure this per EC2 quota?
		runInstancesBatch := 7
		subnetAllocBatch := md.cfg.ClusterSize / len(md.cfg.SubnetIDs)

		subnetIdx := 0
		for left > 0 {
			n := subnetAllocBatch
			if subnetAllocBatch > left {
				n = left
			}
			md.lg.Info(
				"creating an EC2 instance",
				zap.Int("count", n),
				zap.Int("left", left),
				zap.Int("target-total", md.cfg.ClusterSize),
			)

			subnetID := md.cfg.SubnetIDs[0]
			if len(md.cfg.SubnetIDs) > 1 {
				subnetID = md.cfg.SubnetIDs[subnetIdx%len(md.cfg.SubnetIDs)]
			}

			if n < runInstancesBatch {
				tkn := md.cfg.ClusterName + fmt.Sprintf("%X", time.Now().Nanosecond())
				tokens = append(tokens, tkn)

				_, err = md.ec2.RunInstances(&ec2.RunInstancesInput{
					ClientToken:                       aws.String(tkn),
					ImageId:                           aws.String(md.cfg.ImageID),
					MinCount:                          aws.Int64(int64(n)),
					MaxCount:                          aws.Int64(int64(n)),
					InstanceType:                      aws.String(md.cfg.InstanceType),
					KeyName:                           aws.String(md.cfg.KeyName),
					SubnetId:                          aws.String(subnetID),
					SecurityGroupIds:                  aws.StringSlice(md.cfg.SecurityGroupIDs),
					InstanceInitiatedShutdownBehavior: aws.String("terminate"),
					UserData:                          aws.String(base64.StdEncoding.EncodeToString([]byte(md.cfg.InitScript))),
					IamInstanceProfile:                instanceProfileSpecifics,
					TagSpecifications: []*ec2.TagSpecification{
						{
							ResourceType: aws.String("instance"),
							Tags: []*ec2.Tag{
								{
									Key:   aws.String("Name"),
									Value: aws.String(md.cfg.ClusterName),
								},
							},
						},
					},
				})
				if err != nil {
					return err
				}
				tknToCnt[tkn] = n
			} else {
				nLeft := n
				for nLeft > 0 {
					tkn := md.cfg.ClusterName + fmt.Sprintf("%X", time.Now().UTC().Nanosecond())
					tokens = append(tokens, tkn)

					x := runInstancesBatch
					if nLeft < runInstancesBatch {
						x = nLeft
					}
					nLeft -= x

					_, err = md.ec2.RunInstances(&ec2.RunInstancesInput{
						ClientToken:                       aws.String(tkn),
						ImageId:                           aws.String(md.cfg.ImageID),
						MinCount:                          aws.Int64(int64(x)),
						MaxCount:                          aws.Int64(int64(x)),
						InstanceType:                      aws.String(md.cfg.InstanceType),
						KeyName:                           aws.String(md.cfg.KeyName),
						SubnetId:                          aws.String(subnetID),
						SecurityGroupIds:                  aws.StringSlice(md.cfg.SecurityGroupIDs),
						InstanceInitiatedShutdownBehavior: aws.String("terminate"),
						UserData:                          aws.String(base64.StdEncoding.EncodeToString([]byte(md.cfg.InitScript))),
						IamInstanceProfile:                instanceProfileSpecifics,
						TagSpecifications: []*ec2.TagSpecification{
							{
								ResourceType: aws.String("instance"),
								Tags: []*ec2.Tag{
									{
										Key:   aws.String("Name"),
										Value: aws.String(md.cfg.ClusterName),
									},
								},
							},
						},
					})
					if err != nil {
						return err
					}

					tknToCnt[tkn] = x
					md.lg.Info("launched a batch of instances", zap.Int("instance-count", x))

					time.Sleep(10 * time.Second)
				}
			}

			subnetIdx++
			left -= subnetAllocBatch

			md.lg.Info(
				"created EC2 instance group",
				zap.String("subnet-id", subnetID),
				zap.String("availability-zone", md.cfg.SubnetIDToAvailabilityZone[subnetID]),
				zap.Int("instance-count", n),
			)
		}
	} else {
		// create <1 instance per subnet
		for i := 0; i < md.cfg.ClusterSize; i++ {
			tkn := md.cfg.ClusterName + fmt.Sprintf("%X", time.Now().Nanosecond())
			tokens = append(tokens, tkn)
			tknToCnt[tkn] = 1

			subnetID := md.cfg.SubnetIDs[0]
			if len(md.cfg.SubnetIDs) > 1 {
				subnetID = md.cfg.SubnetIDs[i%len(md.cfg.SubnetIDs)]
			}

			_, err = md.ec2.RunInstances(&ec2.RunInstancesInput{
				ClientToken:                       aws.String(tkn),
				ImageId:                           aws.String(md.cfg.ImageID),
				MinCount:                          aws.Int64(1),
				MaxCount:                          aws.Int64(1),
				InstanceType:                      aws.String(md.cfg.InstanceType),
				KeyName:                           aws.String(md.cfg.KeyName),
				SubnetId:                          aws.String(subnetID),
				SecurityGroupIds:                  aws.StringSlice(md.cfg.SecurityGroupIDs),
				InstanceInitiatedShutdownBehavior: aws.String("terminate"),
				UserData:                          aws.String(base64.StdEncoding.EncodeToString([]byte(md.cfg.InitScript))),
				IamInstanceProfile:                instanceProfileSpecifics,
				TagSpecifications: []*ec2.TagSpecification{
					{
						ResourceType: aws.String("instance"),
						Tags: []*ec2.Tag{
							{
								Key:   aws.String("Name"),
								Value: aws.String(md.cfg.ClusterName),
							},
						},
					},
				},
			})
			if err != nil {
				return err
			}

			md.lg.Info(
				"created EC2 instance group",
				zap.String("subnet-id", subnetID),
				zap.String("availability-zone", md.cfg.SubnetIDToAvailabilityZone[subnetID]),
			)
		}
	}

	md.cfg.Instances = make(map[string]ec2config.Instance)
	tknToCntRunning := make(map[string]int)

	retryStart := time.Now().UTC()
	for len(md.cfg.Instances) != md.cfg.ClusterSize &&
		time.Now().UTC().Sub(retryStart) < time.Duration(md.cfg.ClusterSize)*2*time.Minute {
		for _, tkn := range tokens {
			if v, ok := tknToCntRunning[tkn]; ok {
				if v == tknToCnt[tkn] {
					continue
				}
			}

			var output *ec2.DescribeInstancesOutput
			output, err = md.ec2.DescribeInstances(&ec2.DescribeInstancesInput{
				Filters: []*ec2.Filter{
					{
						Name:   aws.String("client-token"),
						Values: aws.StringSlice([]string{tkn}),
					},
				},
			})
			if err != nil {
				md.lg.Warn("failed to describe instances", zap.Error(err))
				time.Sleep(3 * time.Second)
				continue
			}

			for _, rv := range output.Reservations {
				for _, inst := range rv.Instances {
					id := *inst.InstanceId
					if *inst.State.Name == "running" {
						_, ok := md.cfg.Instances[id]
						if !ok {
							iv := ConvertEC2Instance(inst)
							md.cfg.Instances[id] = iv
							md.lg.Info("instance is ready",
								zap.String("instance-id", iv.InstanceID),
								zap.String("instance-public-ip", iv.PublicIP),
								zap.String("instance-private-ip", iv.PrivateIP),
								zap.String("instance-public-dns", iv.PublicDNSName),
								zap.String("instance-private-dns", iv.PrivateDNSName),
							)
							tknToCntRunning[tkn]++

							if v, ok := tknToCntRunning[tkn]; ok {
								if v == tknToCnt[tkn] {
									md.lg.Info("instance group is ready", zap.String("client-token", tkn), zap.Int("count", v))
								}
							}
						}
					}
				}
			}

			time.Sleep(5 * time.Second)
		}
	}

	md.lg.Info(
		"created EC2 instances",
		zap.Int("cluster-size", md.cfg.ClusterSize),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)

	if md.cfg.Wait {
		md.cfg.Sync()
		md.lg.Info(
			"waiting for EC2 instances",
			zap.Int("cluster-size", md.cfg.ClusterSize),
			zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
		)
		mm := make(map[string]ec2config.Instance, len(md.cfg.Instances))
		for k, v := range md.cfg.Instances {
			mm[k] = v
		}
		md.wait(mm)
	}
	return md.cfg.Sync()
}

func (md *embedded) wait(mm map[string]ec2config.Instance) {
	for {
	done:
		for id, iv := range mm {
			md.lg.Info("waiting for EC2", zap.String("instance-id", id))
			sh, serr := ssh.New(ssh.Config{
				Logger:        md.lg,
				KeyPath:       md.cfg.KeyPath,
				PublicIP:      iv.PublicIP,
				PublicDNSName: iv.PublicDNSName,
				UserName:      md.cfg.UserName,
			})
			if serr != nil {
				fmt.Fprintf(os.Stderr, "failed to create SSH (%v)\n", serr)
				os.Exit(1)
			}

			if err := sh.Connect(); err != nil {
				fmt.Fprintf(os.Stderr, "failed to connect SSH (%v)\n", err)
				os.Exit(1)
			}

			var out []byte
			var err error
			for {
				select {
				case <-time.After(5 * time.Second):
					out, err = sh.Run(
						"tail -15 /var/log/cloud-init-output.log",
						ssh.WithRetry(100, 5*time.Second),
						ssh.WithTimeout(30*time.Second),
					)
					if err != nil {
						md.lg.Warn("failed to fetch cloud-init-output.log", zap.Error(err))
						sh.Close()
						if serr := sh.Connect(); serr != nil {
							fmt.Fprintf(os.Stderr, "failed to connect SSH (%v)\n", serr)
							continue done
						}
						continue
					}

					fmt.Printf("\n\n%s\n\n", string(out))

					if IsReady(string(out)) {
						sh.Close()
						md.lg.Info("cloud-init-output.log READY!", zap.String("instance-id", id))
						delete(mm, id)
						continue done
					}

					md.lg.Info("cloud-init-output NOT READY", zap.String("instance-id", id))
					fmt.Println(md.cfg.SSHCommands())
				}
			}
		}
		if len(mm) == 0 {
			md.lg.Info("all EC2 instances are ready")
			break
		}
	}
}

// IsReady returns true if the instance cloud init logs indicate it's ready.
func IsReady(txt string) bool {
	/*
		to match:

		AWS_K8S_TESTER_EC2_PLUGIN_READY
		Cloud-init v. 18.2 running 'modules:final' at Mon, 29 Oct 2018 22:40:13 +0000. Up 21.89 seconds.
		Cloud-init v. 18.2 finished at Mon, 29 Oct 2018 22:43:59 +0000. Datasource DataSourceEc2Local.  Up 246.57 seconds
	*/
	return strings.Contains(txt, plugins.READY) ||
		(strings.Contains(txt, `Cloud-init v.`) &&
			strings.Contains(txt, `finished at`))
}

func (md *embedded) Logger() *zap.Logger {
	return md.lg
}

func catchStopc(lg *zap.Logger, stopc chan struct{}, run func() error) (err error) {
	errc := make(chan error)
	go func() {
		errc <- run()
	}()
	select {
	case <-stopc:
		lg.Info("interrupting")
		gerr := <-errc
		lg.Info("interrupted", zap.Error(gerr))
		err = fmt.Errorf("interrupted (run function returned %v)", gerr)
	case err = <-errc:
		if err != nil {
			return err
		}
	}
	return err
}

// ConvertEC2Instance converts "aws ec2 describe-instances" to "config.Instance".
func ConvertEC2Instance(iv *ec2.Instance) (instance ec2config.Instance) {
	instance = ec2config.Instance{
		ImageID:      *iv.ImageId,
		InstanceID:   *iv.InstanceId,
		InstanceType: *iv.InstanceType,
		KeyName:      *iv.KeyName,
		Placement: ec2config.Placement{
			AvailabilityZone: *iv.Placement.AvailabilityZone,
			Tenancy:          *iv.Placement.Tenancy,
		},
		PrivateDNSName: *iv.PrivateDnsName,
		PrivateIP:      *iv.PrivateIpAddress,
		State: ec2config.State{
			Code: *iv.State.Code,
			Name: *iv.State.Name,
		},
		SubnetID:            *iv.SubnetId,
		VPCID:               *iv.VpcId,
		BlockDeviceMappings: make([]ec2config.BlockDeviceMapping, len(iv.BlockDeviceMappings)),
		EBSOptimized:        *iv.EbsOptimized,
		RootDeviceName:      *iv.RootDeviceName,
		RootDeviceType:      *iv.RootDeviceType,
		SecurityGroups:      make([]ec2config.SecurityGroup, len(iv.SecurityGroups)),
		LaunchTime:          *iv.LaunchTime,
	}
	if iv.PublicDnsName != nil {
		instance.PublicDNSName = *iv.PublicDnsName
	}
	if iv.PublicIpAddress != nil {
		instance.PublicIP = *iv.PublicIpAddress
	}
	for j := range iv.BlockDeviceMappings {
		instance.BlockDeviceMappings[j] = ec2config.BlockDeviceMapping{
			DeviceName: *iv.BlockDeviceMappings[j].DeviceName,
			EBS: ec2config.EBS{
				DeleteOnTermination: *iv.BlockDeviceMappings[j].Ebs.DeleteOnTermination,
				Status:              *iv.BlockDeviceMappings[j].Ebs.Status,
				VolumeID:            *iv.BlockDeviceMappings[j].Ebs.VolumeId,
			},
		}
	}
	for j := range iv.SecurityGroups {
		instance.SecurityGroups[j] = ec2config.SecurityGroup{
			GroupName: *iv.SecurityGroups[j].GroupName,
			GroupID:   *iv.SecurityGroups[j].GroupId,
		}
	}
	return instance
}

// genS3URL returns S3 URL path.
// e.g. https://s3-us-west-2.amazonaws.com/aws-k8s-tester-20180925/hello-world
func genS3URL(region, bucket, s3Path string) string {
	return fmt.Sprintf("https://s3-%s.amazonaws.com/%s/%s", region, bucket, s3Path)
}
