// Package nodewaiter implements node waiter.
package nodewaiter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/eksconfig"
	aws_ec2 "github.com/aws/aws-k8s-tester/pkg/aws/ec2"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	k8s_object "github.com/aws/aws-k8s-tester/pkg/k8s-object"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/eks"
	aws_eks "github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	v1 "k8s.io/api/core/v1"
)

// NodeWaiter defines node waiter operation.
type NodeWaiter interface {
	// Wait waits until all MNG and Kubernetes nodes are ready.
	Wait(mngName string, retries int) error
}

// Config defines version upgrade configuration.
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	EC2API    ec2iface.EC2API
	ASGAPI    autoscalingiface.AutoScalingAPI
	EKSAPI    eksiface.EKSAPI
}

// New creates a new node waiter.
func New(cfg Config) NodeWaiter {
	cfg.Logger.Info("creating NodeWaiter", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	return &tester{cfg: cfg}
}

type tester struct {
	cfg Config
}

func (ts *tester) Wait(mngName string, retries int) error {
	return ts.waitForNodes(mngName, retries)
}

func (ts *tester) waitForNodes(mngName string, retriesLeft int) error {
	cur, ok := ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
	if !ok {
		return fmt.Errorf("MNGs[%q] not found", mngName)
	}

	ts.cfg.Logger.Info("checking MNG", zap.String("mng-name", cur.Name))
	dout, err := ts.cfg.EKSAPI.DescribeNodegroup(&eks.DescribeNodegroupInput{
		ClusterName:   aws.String(ts.cfg.EKSConfig.Name),
		NodegroupName: aws.String(cur.Name),
	})
	if err != nil {
		return err
	}
	if dout.Nodegroup == nil {
		return fmt.Errorf("MNG %q not found", cur.Name)
	}
	if dout.Nodegroup.Resources == nil {
		return fmt.Errorf("MNG %q Resources not found", cur.Name)
	}
	if len(dout.Nodegroup.Resources.AutoScalingGroups) != 1 {
		return fmt.Errorf("expected 1 ASG for %q, got %d", mngName, len(dout.Nodegroup.Resources.AutoScalingGroups))
	}
	if cur.RemoteAccessSecurityGroupID == "" {
		cur.RemoteAccessSecurityGroupID = aws.StringValue(dout.Nodegroup.Resources.RemoteAccessSecurityGroup)
		ts.cfg.Logger.Info("checking MNG security group", zap.String("mng-name", cur.Name), zap.String("security-group-id", cur.RemoteAccessSecurityGroupID))
	}
	if cur.RemoteAccessSecurityGroupID == "" {
		if retriesLeft > 0 {
			ts.cfg.Logger.Warn("remote access security group ID not found; retrying", zap.String("mng-name", mngName), zap.Int("retries-left", retriesLeft))
			time.Sleep(5 * time.Second)
			return ts.waitForNodes(mngName, retriesLeft-1)
		}
		return fmt.Errorf("remote access security group ID not found for mng %q", mngName)
	}
	ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
	ts.cfg.EKSConfig.Sync()

	cur, ok = ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
	if !ok {
		return fmt.Errorf("Managed Node Group %q not found", mngName)
	}
	asg := dout.Nodegroup.Resources.AutoScalingGroups[0]
	cur.ASGName = aws.StringValue(asg.Name)
	ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
	ts.cfg.EKSConfig.Sync()
	ts.cfg.Logger.Info("checking MNG ASG", zap.String("mng-name", cur.Name), zap.String("asg-name", cur.ASGName))

	var aout *autoscaling.DescribeAutoScalingGroupsOutput
	aout, err = ts.cfg.ASGAPI.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: aws.StringSlice([]string{cur.ASGName}),
	})
	if err != nil {
		return fmt.Errorf("ASG %q not found (%v)", cur.ASGName, err)
	}
	if len(aout.AutoScalingGroups) != 1 {
		return fmt.Errorf("%q expected only 1 ASG, got %+v", cur.ASGName, aout.AutoScalingGroups)
	}

	av := aout.AutoScalingGroups[0]
	instanceIDs := make([]string, 0, len(av.Instances))
	for _, iv := range av.Instances {
		instanceIDs = append(instanceIDs, aws.StringValue(iv.InstanceId))
	}

	waitDur := 3*time.Minute + time.Duration(5*cur.ASGDesiredCapacity)*time.Second
	ts.cfg.Logger.Info(
		"describing EC2 instances in ASG",
		zap.String("asg-name", cur.ASGName),
		zap.Strings("instance-ids", instanceIDs),
		zap.Duration("wait", waitDur),
	)
	ec2Instances, err := aws_ec2.PollUntilRunning(
		waitDur,
		ts.cfg.Logger,
		ts.cfg.EC2API,
		instanceIDs...,
	)
	if err != nil {
		return err
	}
	cur, ok = ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
	if !ok {
		return fmt.Errorf("Managed Node Group %q not found", mngName)
	}
	cur.Instances = make(map[string]ec2config.Instance)
	for id, vv := range ec2Instances {
		ivv := ec2config.ConvertInstance(vv)
		ivv.RemoteAccessUserName = cur.RemoteAccessUserName
		cur.Instances[id] = ivv
	}
	ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
	ts.cfg.EKSConfig.Sync()

	cur, ok = ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
	if !ok {
		return fmt.Errorf("Managed Node Group %q not found", mngName)
	}

	// Hostname/InternalDNS == EC2 private DNS
	// TODO: handle DHCP option domain name
	ec2PrivateDNS := make(map[string]struct{})
	for _, v := range cur.Instances {
		ts.cfg.Logger.Info("found private DNS for an EC2 instance", zap.String("instance-id", v.InstanceID), zap.String("private-dns-name", v.PrivateDNSName))
		ec2PrivateDNS[v.PrivateDNSName] = struct{}{}
		// "ip-192-168-81-186" from "ip-192-168-81-186.my-private-dns"
		ec2PrivateDNS[strings.Split(v.PrivateDNSName, ".")[0]] = struct{}{}
	}

	ts.cfg.Logger.Info("checking nodes readiness")
	retryStart := time.Now()
	ready := false
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("checking node aborted")
		case <-time.After(5 * time.Second):
		}

		nodes, err := ts.cfg.K8SClient.ListNodes(150, 5*time.Second)
		if err != nil {
			ts.cfg.Logger.Warn("get nodes failed", zap.Error(err))
			continue
		}

		readies := 0
		for _, node := range nodes {
			labels := node.GetLabels()
			if labels["NGName"] != mngName {
				continue
			}
			nodeName := node.GetName()
			nodeInfo, _ := json.Marshal(k8s_object.ParseNodeInfo(node.Status.NodeInfo))

			// e.g. given node name ip-192-168-81-186.us-west-2.compute.internal + DHCP option my-private-dns
			// InternalIP == 192.168.81.186
			// ExternalIP == 52.38.118.149
			// Hostname == my-private-dns (without DHCP option, it's "ip-192-168-81-186.my-private-dns", private DNS, InternalDNS)
			// InternalDNS == ip-192-168-81-186.my-private-dns
			// ExternalDNS == ec2-52-38-118-149.us-west-2.compute.amazonaws.com
			ts.cfg.Logger.Info("checking node address with EC2 Private DNS",
				zap.String("node-name", nodeName),
				zap.String("node-info", string(nodeInfo)),
				zap.String("labels", fmt.Sprintf("%v", labels)),
			)

			hostName := ""
			for _, av := range node.Status.Addresses {
				ts.cfg.Logger.Info("node status address",
					zap.String("node-name", nodeName),
					zap.String("type", string(av.Type)),
					zap.String("address", string(av.Address)),
				)
				if av.Type != v1.NodeHostName && av.Type != v1.NodeInternalDNS {
					continue
				}
				// handle when node is configured DHCP
				hostName = av.Address
				_, ok := ec2PrivateDNS[hostName]
				if !ok {
					// "ip-192-168-81-186" from "ip-192-168-81-186.my-private-dns"
					_, ok = ec2PrivateDNS[strings.Split(hostName, ".")[0]]
				}
				if ok {
					break
				}
			}
			if hostName == "" {
				return fmt.Errorf("%q not found for node %q", v1.NodeHostName, nodeName)
			}
			_, ok := ec2PrivateDNS[hostName]
			if !ok {
				// "ip-192-168-81-186" from "ip-192-168-81-186.my-private-dns"
				_, ok = ec2PrivateDNS[strings.Split(hostName, ".")[0]]
			}
			if !ok {
				ts.cfg.Logger.Warn("node may not belong to this ASG", zap.String("host-name", hostName), zap.Int("ec2-private-dnss", len(ec2PrivateDNS)))
				continue
			}
			ts.cfg.Logger.Debug("checked node host name with EC2 Private DNS", zap.String("name", nodeName), zap.String("host-name", hostName))

			for _, cond := range node.Status.Conditions {
				if cond.Status != v1.ConditionTrue {
					continue
				}
				if cond.Type != v1.NodeReady {
					continue
				}
				ts.cfg.Logger.Info("node is ready!",
					zap.String("name", nodeName),
					zap.String("type", fmt.Sprintf("%s", cond.Type)),
					zap.String("status", fmt.Sprintf("%s", cond.Status)),
				)
				readies++
				break
			}
		}
		ts.cfg.Logger.Info("nodes",
			zap.Int("current-ready-nodes", readies),
			zap.Int("desired-ready-nodes", cur.ASGDesiredCapacity),
		)

		/*
			e.g.
			"/tmp/kubectl-test-v1.16.9 --kubeconfig=/tmp/leegyuho-test-eks.kubeconfig.yaml get csr -o=wide":
			NAME        AGE   REQUESTOR                                                   CONDITION
			csr-4msk5   58s   system:node:ip-192-168-65-124.us-west-2.compute.internal    Approved,Issued
			csr-9dbs8   57s   system:node:ip-192-168-208-6.us-west-2.compute.internal     Approved,Issued
		*/
		output, err := ts.cfg.K8SClient.ListCSRs(150, 5*time.Second)
		if err != nil {
			ts.cfg.Logger.Warn("list CSRs failed", zap.Error(err))
		} else {
			for _, cv := range output {
				ts.cfg.Logger.Info("current CSR",
					zap.String("name", cv.GetName()),
					zap.String("requester", cv.Spec.Username),
					zap.String("status", extractCSRStatus(cv)),
				)
			}
		}

		if readies >= cur.ASGDesiredCapacity {
			ready = true
			break
		}
	}
	if !ready {
		return fmt.Errorf("MNG %q not ready", mngName)
	}

	return ts.cfg.EKSConfig.Sync()
}

// "pkg/printers/internalversion/printers.go"
func extractCSRStatus(csr certificatesv1beta1.CertificateSigningRequest) string {
	var approved, denied bool
	for _, c := range csr.Status.Conditions {
		switch c.Type {
		case certificatesv1beta1.CertificateApproved:
			approved = true
		case certificatesv1beta1.CertificateDenied:
			denied = true
		default:
			return ""
		}
	}
	var status string
	// must be in order of presidence
	if denied {
		status += "Denied"
	} else if approved {
		status += "Approved"
	} else {
		status += "Pending"
	}
	if len(csr.Status.Certificate) > 0 {
		status += ",Issued"
	}
	return status
}

// ManagedNodeGroupStatusDELETEDORNOTEXIST defines the cluster status when the cluster is not found.
//
// ref. https://docs.aws.amazon.com/eks/latest/APIReference/API_Nodegroup.html
//
//  CREATING
//  ACTIVE
//  DELETING
//  FAILED
//  UPDATING
//
const ManagedNodeGroupStatusDELETEDORNOTEXIST = "DELETED/NOT-EXIST"

// ManagedNodeGroupStatus represents the CloudFormation status.
type ManagedNodeGroupStatus struct {
	NodeGroupName string
	NodeGroup     *aws_eks.Nodegroup
	Error         error
}

// Poll periodically fetches the managed node group status
// until the node group becomes the desired state.
func Poll(
	ctx context.Context,
	stopc chan struct{},
	lg *zap.Logger,
	eksAPI eksiface.EKSAPI,
	clusterName string,
	mngName string,
	desiredNodeGroupStatus string,
	initialWait time.Duration,
	wait time.Duration,
) <-chan ManagedNodeGroupStatus {
	lg.Info("polling mng",
		zap.String("cluster-name", clusterName),
		zap.String("mng-name", mngName),
		zap.String("desired-status", desiredNodeGroupStatus),
	)

	now := time.Now()

	ch := make(chan ManagedNodeGroupStatus, 10)
	go func() {
		// very first poll should be no-wait
		// in case stack has already reached desired status
		// wait from second interation
		waitDur := time.Duration(0)

		first := true
		for ctx.Err() == nil {
			select {
			case <-ctx.Done():
				lg.Warn("wait aborted", zap.Error(ctx.Err()))
				ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: ctx.Err()}
				close(ch)
				return

			case <-stopc:
				lg.Warn("wait stopped", zap.Error(ctx.Err()))
				ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: errors.New("wait stopped")}
				close(ch)
				return

			case <-time.After(waitDur):
				// very first poll should be no-wait
				// in case stack has already reached desired status
				// wait from second interation
				if waitDur == time.Duration(0) {
					waitDur = wait
				}
			}

			output, err := eksAPI.DescribeNodegroup(&aws_eks.DescribeNodegroupInput{
				ClusterName:   aws.String(clusterName),
				NodegroupName: aws.String(mngName),
			})
			if err != nil {
				if IsDeleted(err) {
					if desiredNodeGroupStatus == ManagedNodeGroupStatusDELETEDORNOTEXIST {
						lg.Info("managed node group is already deleted as desired; exiting", zap.Error(err))
						ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: nil}
						close(ch)
						return
					}

					lg.Warn("managed node group does not exist", zap.Error(err))
					lg.Warn("aborting", zap.Error(ctx.Err()))
					ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: err}
					close(ch)
					return
				}

				lg.Warn("describe managed node group failed; retrying", zap.Error(err))
				ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: err}
				continue
			}

			if output.Nodegroup == nil {
				lg.Warn("expected non-nil managed node group; retrying")
				ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: fmt.Errorf("unexpected empty response %+v", output.GoString())}
				continue
			}

			nodeGroup := output.Nodegroup
			currentStatus := aws.StringValue(nodeGroup.Status)
			lg.Info("poll",
				zap.String("cluster-name", clusterName),
				zap.String("mng-name", mngName),
				zap.String("status", currentStatus),
				zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			)
			switch currentStatus {
			case desiredNodeGroupStatus:
				ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nodeGroup, Error: nil}
				lg.Info("desired managed node group status; done", zap.String("status", currentStatus))
				close(ch)
				return
			case aws_eks.NodegroupStatusCreateFailed,
				aws_eks.NodegroupStatusDeleteFailed,
				aws_eks.NodegroupStatusDegraded:
				ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nodeGroup, Error: fmt.Errorf("unexpected mng status %q", currentStatus)}
				close(ch)
				return
			default:
				ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nodeGroup, Error: nil}
			}

			if first {
				lg.Info("sleeping", zap.Duration("initial-wait", initialWait))
				select {
				case <-ctx.Done():
					lg.Warn("wait aborted", zap.Error(ctx.Err()))
					ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: ctx.Err()}
					close(ch)
					return
				case <-stopc:
					lg.Warn("wait stopped", zap.Error(ctx.Err()))
					ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: errors.New("wait stopped")}
					close(ch)
					return
				case <-time.After(initialWait):
				}
				first = false
			}
		}

		lg.Warn("wait aborted", zap.Error(ctx.Err()))
		ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: ctx.Err()}
		close(ch)
		return
	}()
	return ch
}

// IsDeleted returns true if error from EKS API indicates that
// the EKS managed node group has already been deleted.
func IsDeleted(err error) bool {
	if err == nil {
		return false
	}
	awsErr, ok := err.(awserr.Error)
	if ok && awsErr.Code() == "ResourceNotFoundException" {
		return true
	}

	// ResourceNotFoundException: nodeGroup eks-2019120505-pdx-us-west-2-tqy2d-managed-node-group not found for cluster eks-2019120505-pdx-us-west-2-tqy2d\n\tstatus code: 404, request id: 330998c1-22e9-4a8b-b180-420dadade090
	return strings.Contains(err.Error(), " not found for cluster ")
}
