package mng

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/eks/mng/wait"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-k8s-tester/pkg/user"
	"github.com/aws/aws-k8s-tester/version"
	aws_v2 "github.com/aws/aws-sdk-go-v2/aws"
	aws_eks "github.com/aws/aws-sdk-go/service/eks"
	smithy "github.com/aws/smithy-go"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

func (ts *tester) createMNGs() error {
	now := time.Now()
	tss, err := ts._createMNGs()
	if err != nil {
		return err
	}
	if err = ts.waitForMNGs(now, tss); err != nil {
		return err
	}
	return nil
}

func (ts *tester) deleteMNG(mngName string) error {
	cur, ok := ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
	if !ok {
		return fmt.Errorf("MNGs[%q] not found; cannot delete managed node group", mngName)
	}
	if cur.RemoteAccessSecurityGroupID == "" {
		return fmt.Errorf("MNG[%q] security group ID not found; cannot delete managed node group", mngName)
	}
	ts.cfg.Logger.Info("deleting MNG/ASG", zap.String("mng-name", mngName))

	ts.cfg.Logger.Info("deleting managed node group using EKS API", zap.String("name", mngName))
	_, err := ts.cfg.EKSAPI.DeleteNodegroup(&aws_eks.DeleteNodegroupInput{
		ClusterName:   aws_v2.String(ts.cfg.EKSConfig.Name),
		NodegroupName: aws_v2.String(mngName),
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				ts.cfg.EKSConfig.Status.DeletedResources[mngName] = "AddOnManagedNodeGroups.Name"
				ts.cfg.EKSConfig.Sync()
				return nil
			}
		}
		return err
	}

	timeStart := time.Now()

	ts.cfg.Logger.Info("waiting for delete managed node group using EKS API", zap.String("name", mngName))
	initialWait, timeout := 2*time.Minute, 15*time.Minute
	if len(cur.Instances) > 50 {
		initialWait, timeout = 3*time.Minute, 20*time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	ch := wait.Poll(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Logger,
		ts.cfg.LogWriter,
		ts.cfg.EKSAPI,
		ts.cfg.EKSConfig.Name,
		mngName,
		wait.ManagedNodeGroupStatusDELETEDORNOTEXIST,
		initialWait,
		20*time.Second,
	)
	for sv := range ch {
		serr := ts.setStatus(sv)
		if serr != nil {
			cancel()
			return serr
		}
		err = sv.Error
	}
	cancel()
	if err != nil {
		return err
	}

	timeEnd := time.Now()
	cur.TimeFrameDelete = timeutil.NewTimeFrame(timeStart, timeEnd)
	if err != nil {
		cur.Status = fmt.Sprintf("MNGs[%q] failed to delete %v", mngName, err)
		ts.cfg.Logger.Warn("failed to delete managed node group", zap.String("name", mngName), zap.Error(err))
	} else {
		cur.Status = wait.ManagedNodeGroupStatusDELETEDORNOTEXIST
		ts.cfg.Logger.Info("deleted managed node group", zap.String("name", mngName))
	}
	ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
	ts.cfg.EKSConfig.Sync()

	return nil
}

func (ts *tester) _createMNGs() (tss tupleTimes, err error) {
	ts.cfg.Logger.Info("creating MNGs")

	for mngName, cur := range ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs {
		ts.cfg.Logger.Info("requesting MNG creation", zap.String("mng-name", mngName))

		createInput := aws_eks.CreateNodegroupInput{
			ClusterName:   aws_v2.String(ts.cfg.EKSConfig.Name),
			NodegroupName: aws_v2.String(cur.Name),
			NodeRole:      aws_v2.String(ts.cfg.EKSConfig.AddOnManagedNodeGroups.Role.ARN),
			AmiType:       aws_v2.String(cur.AMIType),
			DiskSize:      aws_v2.Int64(int64(cur.VolumeSize)),
			InstanceTypes: aws_v2.StringSlice(cur.InstanceTypes),
			RemoteAccess: &aws_eks.RemoteAccessConfig{
				Ec2SshKey: aws_v2.String(ts.cfg.EKSConfig.RemoteAccessKeyName),
			},
			ScalingConfig: &aws_eks.NodegroupScalingConfig{
				MinSize:     aws_v2.Int64(int64(cur.ASGMinSize)),
				DesiredSize: aws_v2.Int64(int64(cur.ASGDesiredCapacity)),
				MaxSize:     aws_v2.Int64(int64(cur.ASGMaxSize)),
			},
			Subnets: aws_v2.StringSlice(ts.cfg.EKSConfig.VPC.PublicSubnetIDs),
			Tags: map[string]*string{
				"Kind":                   aws_v2.String("aws-k8s-tester"),
				"aws-k8s-tester-version": aws_v2.String(version.ReleaseVersion),
				"User":                   aws_v2.String(user.Get()),
			},
			Labels: map[string]*string{
				"NodeType": aws_v2.String("regular"),
				"AMIType":  aws_v2.String(cur.AMIType),
				"NGType":   aws_v2.String("managed"),
				"NGName":   aws_v2.String(cur.Name),
			},
		}
		for k, v := range cur.Tags {
			createInput.Tags[k] = aws_v2.String(v)
			ts.cfg.Logger.Info("added EKS tag", zap.String("key", k), zap.String("value", v))
		}
		if cur.ReleaseVersion != "" {
			createInput.ReleaseVersion = aws_v2.String(cur.ReleaseVersion)
			ts.cfg.Logger.Info("added EKS release version", zap.String("version", cur.ReleaseVersion))
		}
		timeStart := time.Now()
		req, _ := ts.cfg.EKSAPI.CreateNodegroupRequest(&createInput)
		if ts.cfg.EKSConfig.AddOnManagedNodeGroups.RequestHeaderKey != "" && ts.cfg.EKSConfig.AddOnManagedNodeGroups.RequestHeaderValue != "" {
			req.HTTPRequest.Header[ts.cfg.EKSConfig.AddOnManagedNodeGroups.RequestHeaderKey] = []string{ts.cfg.EKSConfig.AddOnManagedNodeGroups.RequestHeaderValue}
			ts.cfg.Logger.Info("set request header for EKS managed node group create request",
				zap.String("key", ts.cfg.EKSConfig.AddOnManagedNodeGroups.RequestHeaderKey),
				zap.String("value", ts.cfg.EKSConfig.AddOnManagedNodeGroups.RequestHeaderValue),
			)
		}

		err := req.Send()
		if err != nil {
			ts.cfg.Logger.Warn("failed to created MNG", zap.Error(err))
			return nil, fmt.Errorf("MNGs[%q] create request failed (%v)", cur.Name, err)
		}

		cur.TimeFrameCreate = timeutil.NewTimeFrame(timeStart, time.Now())
		cur.CreateRequested = true
		cur.Status = aws_eks.NodegroupStatusCreating
		cur.Instances = make(map[string]ec2config.Instance)
		cur.Logs = make(map[string][]string)
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.Created = true
		ts.cfg.EKSConfig.Sync()
		ts.cfg.Logger.Info("sent create managed node group request")

		tss = append(tss, tupleTime{ts: time.Now(), name: mngName})
	}

	sort.Sort(sort.Reverse(tss))
	ts.cfg.Logger.Info("created MNGs")
	return tss, nil
}

func (ts *tester) waitForMNGs(now time.Time, tss tupleTimes) (err error) {
	ts.cfg.Logger.Info("waiting for MNGs")

	for _, tv := range tss {
		mngName := tv.name
		cur, ok := ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
		if !ok {
			return fmt.Errorf("MNG name %q not found after creation", mngName)
		}

		select {
		case <-time.After(10 * time.Second):
		case <-ts.cfg.Stopc:
			return errors.New("stopped")
		}

		ts.cfg.Logger.Info("waiting for MNG", zap.String("mng-name", mngName))

		timeStart := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		ch := wait.Poll(
			ctx,
			ts.cfg.Stopc,
			ts.cfg.Logger,
			ts.cfg.LogWriter,
			ts.cfg.EKSAPI,
			ts.cfg.EKSConfig.Name,
			mngName,
			aws_eks.NodegroupStatusActive,
			time.Minute,
			20*time.Second,
		)
		for sv := range ch {
			serr := ts.setStatus(sv)
			if serr != nil {
				cancel()
				return serr
			}
			err = sv.Error
		}
		cancel()
		if err != nil {
			return err
		}

		cur, ok = ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
		if !ok {
			return fmt.Errorf("MNGs[%q] not found after creation", mngName)
		}
		timeEnd := time.Now()
		cur.TimeFrameCreate = timeutil.NewTimeFrame(cur.TimeFrameCreate.StartUTC, cur.TimeFrameCreate.EndUTC.Add(timeEnd.Sub(timeStart)))
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
		ts.cfg.EKSConfig.Sync()

		timeStart = time.Now()
		if err := ts.nodeWaiter.Wait(mngName, 10); err != nil {
			return err
		}
		timeEnd = time.Now()

		cur.TimeFrameCreate = timeutil.NewTimeFrame(timeStart, timeEnd)
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
		ts.cfg.EKSConfig.Sync()

		ts.cfg.Logger.Info("created a managed node group",
			zap.String("mng-name", cur.Name),
			zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
		)
	}

	ts.cfg.Logger.Info("waited for MNGs")
	return nil
}

func (ts *tester) setStatus(sv wait.ManagedNodeGroupStatus) (err error) {
	name := sv.NodeGroupName
	if name == "" {
		return errors.New("EKS Managed Node Group empty name")
	}
	cur, ok := ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[name]
	if !ok {
		return fmt.Errorf("EKS MNGs[%q] not found", name)
	}

	if sv.NodeGroup == nil {
		if sv.Error != nil {
			cur.Status = fmt.Sprintf("%q failed with error %v", sv.NodeGroupName, sv.Error)
		} else {
			cur.Status = wait.ManagedNodeGroupStatusDELETEDORNOTEXIST
		}
	} else {
		cur.Status = aws_v2.ToString(sv.NodeGroup.Status)
		if sv.NodeGroup.Resources != nil && cur.RemoteAccessSecurityGroupID == "" {
			cur.RemoteAccessSecurityGroupID = aws_v2.ToString(sv.NodeGroup.Resources.RemoteAccessSecurityGroup)
		}
	}

	ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[name] = cur
	ts.cfg.EKSConfig.Sync()
	return nil
}
