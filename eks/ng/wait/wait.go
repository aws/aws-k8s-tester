// Package wait implements node waiter.
package wait

import (
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
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"go.uber.org/zap"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	v1 "k8s.io/api/core/v1"
)

// NodeWaiter defines node waiter operation.
type NodeWaiter interface {
	// Wait waits until all NG and Kubernetes nodes are ready.
	Wait(asgName string, retries int) error
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

func (ts *tester) Wait(asgName string, retries int) error {
	return ts.waitForNodes(asgName, retries)
}

func (ts *tester) waitForNodes(asgName string, retriesLeft int) error {
	cur, ok := ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName]
	if !ok {
		return fmt.Errorf("ASGs[%q] not found", asgName)
	}

	ts.cfg.Logger.Info("checking ASG", zap.String("asg-name", cur.Name))
	aout, err := ts.cfg.ASGAPI.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: aws.StringSlice([]string{cur.Name}),
	})
	if err != nil {
		return fmt.Errorf("ASGs[%q] not found (%v)", cur.Name, err)
	}
	if len(aout.AutoScalingGroups) != 1 {
		if retriesLeft > 0 {
			ts.cfg.Logger.Warn("expected only 1 ASG; retrying", zap.String("asg-name", cur.Name), zap.Int("retries-left", retriesLeft))
			time.Sleep(5 * time.Second)
			return ts.waitForNodes(asgName, retriesLeft-1)
		}
		return fmt.Errorf("%q expected only 1 ASG, got %+v", cur.Name, aout.AutoScalingGroups)
	}

	av := aout.AutoScalingGroups[0]
	instanceIDs := make([]string, 0, len(av.Instances))
	for _, iv := range av.Instances {
		instanceIDs = append(instanceIDs, aws.StringValue(iv.InstanceId))
	}

	waitDur := 3*time.Minute + time.Duration(5*cur.ASGDesiredCapacity)*time.Second
	ts.cfg.Logger.Info(
		"waiting for EC2 instances in ASG",
		zap.String("asg-name", cur.Name),
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
	cur, ok = ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName]
	if !ok {
		return fmt.Errorf("ASG %q not found", asgName)
	}
	cur.Instances = make(map[string]ec2config.Instance)
	for id, vv := range ec2Instances {
		ivv := ec2config.ConvertInstance(vv)
		ivv.RemoteAccessUserName = cur.RemoteAccessUserName
		cur.Instances[id] = ivv
	}
	ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = cur
	ts.cfg.EKSConfig.Sync()

	cur, ok = ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName]
	if !ok {
		return fmt.Errorf("ASG %q not found", asgName)
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

	ts.cfg.Logger.Info("checking nodes readiness", zap.Duration("wait", waitDur))
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
			if labels["NGName"] != asgName {
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
			zap.Int64("desired-ready-nodes", cur.ASGDesiredCapacity),
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

		if int64(readies) >= cur.ASGDesiredCapacity {
			ready = true
			break
		}
	}
	if !ready {
		if retriesLeft > 0 {
			ts.cfg.Logger.Warn("nodes in ASG are not ready yet; retrying", zap.String("asg-name", cur.Name), zap.Int("retries-left", retriesLeft))
			time.Sleep(5 * time.Second)
			return ts.waitForNodes(asgName, retriesLeft-1)
		}
		return fmt.Errorf("ASG %q not ready", asgName)
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
