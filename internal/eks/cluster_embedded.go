package eks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	awseks "github.com/aws/aws-sdk-go/service/eks"
	humanize "github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

func (md *embedded) createCluster() error {
	if md.cfg.ClusterName == "" {
		return errors.New("cannot create empty cluster")
	}
	if md.cfg.ClusterState.ServiceRoleWithPolicyARN == "" {
		return errors.New("can't create cluster without service role ARN")
	}
	if len(md.cfg.ClusterState.CFStackVPCSubnetIDs) == 0 {
		return errors.New("can't create cluster without subnet IDs")
	}
	if md.cfg.ClusterState.CFStackVPCSecurityGroupID == "" {
		return errors.New("can't create cluster without security group ID")
	}

	now := time.Now().UTC()

	_, err := md.eks.CreateCluster(&awseks.CreateClusterInput{
		Name:    aws.String(md.cfg.ClusterName),
		Version: aws.String(md.cfg.KubernetesVersion),
		RoleArn: aws.String(md.cfg.ClusterState.ServiceRoleWithPolicyARN),
		ResourcesVpcConfig: &awseks.VpcConfigRequest{
			SubnetIds:        aws.StringSlice(md.cfg.ClusterState.CFStackVPCSubnetIDs),
			SecurityGroupIds: aws.StringSlice([]string{md.cfg.ClusterState.CFStackVPCSecurityGroupID}),
		},
	})
	if err != nil {
		return err
	}
	md.cfg.ClusterState.StatusClusterCreated = true
	md.cfg.ClusterState.Status = "CREATING"
	md.cfg.Sync()

	if md.cfg.LogAutoUpload {
		if err = md.upload(); err != nil {
			md.lg.Warn("failed to upload", zap.Error(err))
		}
	}

	// usually takes 10 minutes
	md.lg.Info("waiting for 7-minute")
	select {
	case <-md.stopc:
		md.lg.Info("interrupted cluster creation")
		return nil
	case <-time.After(7 * time.Minute):
	}

	retryStart := time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 20*time.Minute {
		select {
		case <-md.stopc:
			return nil
		default:
		}

		var do *awseks.DescribeClusterOutput
		do, err = md.eks.DescribeCluster(&awseks.DescribeClusterInput{
			Name: aws.String(md.cfg.ClusterName),
		})
		if err != nil {
			md.lg.Warn("failed to describe cluster", zap.Error(err))
			time.Sleep(10 * time.Second)
			continue
		}

		md.cfg.ClusterState.Status = *do.Cluster.Status
		md.cfg.ClusterState.Created = *do.Cluster.CreatedAt
		md.cfg.ClusterState.PlatformVersion = *do.Cluster.PlatformVersion
		md.cfg.Sync()

		if md.cfg.ClusterState.Status == "FAILED" {
			return fmt.Errorf("failed to create %q", md.cfg.ClusterName)
		}

		if md.cfg.ClusterState.Status == "ACTIVE" {
			if do.Cluster.Endpoint != nil {
				md.cfg.ClusterState.Endpoint = *do.Cluster.Endpoint
			}
			if do.Cluster.CertificateAuthority != nil && do.Cluster.CertificateAuthority.Data != nil {
				md.cfg.ClusterState.CA = *do.Cluster.CertificateAuthority.Data
			}
			md.cfg.Sync()
			break
		}

		md.cfg.Sync()

		md.lg.Info("creating cluster",
			zap.String("status", md.cfg.ClusterState.Status),
			zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
		)

		if md.cfg.LogAutoUpload {
			if err = md.upload(); err != nil {
				md.lg.Warn("failed to upload", zap.Error(err))
			}
		}

		time.Sleep(30 * time.Second)
	}

	if md.cfg.ClusterState.Status != "ACTIVE" {
		return fmt.Errorf("cluster creation took too long (status %q, took %v)", md.cfg.ClusterState.Status, time.Now().UTC().Sub(now))
	}
	if md.cfg.ClusterState.Endpoint == "" || md.cfg.ClusterState.CA == "" {
		return errors.New("cannot find cluster endpoint or cluster CA")
	}

	if err = writeKubeConfig(
		md.cfg.ClusterState.Endpoint,
		md.cfg.ClusterState.CA,
		md.cfg.ClusterName,
		md.cfg.KubeConfigPath,
	); err != nil {
		return err
	}
	if err = md.s3Plugin.UploadToBucketForTests(
		md.cfg.KubeConfigPath,
		md.cfg.KubeConfigPathBucket,
	); err != nil {
		md.lg.Warn("failed to upload KUBECONFIG", zap.Error(err))
	}
	md.lg.Info("wrote KUBECONFIG", zap.String("env", fmt.Sprintf("KUBECONFIG=%s", md.cfg.KubeConfigPath)))

	time.Sleep(3 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	cmd := md.kubectl.CommandContext(ctx,
		md.kubectlPath,
		"--kubeconfig="+md.cfg.KubeConfigPath,
		"get", "all",
	)
	var kubectlOutput []byte
	kubectlOutput, err = cmd.CombinedOutput()
	cancel()
	kubectlOutputTxt := string(kubectlOutput)

	md.lg.Info("kubectl get all", zap.String("output", kubectlOutputTxt), zap.Error(err))

	if err == nil && !isKubernetesControlPlaneReadyKubectl(kubectlOutputTxt) {
		return fmt.Errorf("'kubectl get all' output unexpected: %s", kubectlOutputTxt)
	}

	md.lg.Info("created cluster",
		zap.String("name", md.cfg.ClusterName),
		zap.String("kubernetes-version", md.cfg.KubernetesVersion),
		zap.String("custom-endpoint", md.cfg.AWSCustomEndpoint),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)
	return md.cfg.Sync()
}

func (md *embedded) deleteCluster(deleteKubeconfig bool) error {
	if !md.cfg.ClusterState.StatusClusterCreated {
		return nil
	}
	defer func() {
		md.cfg.ClusterState.StatusClusterCreated = false
		md.cfg.Sync()
	}()

	if md.cfg.ClusterName == "" {
		return errors.New("cannot delete empty cluster")
	}

	now := time.Now().UTC()

	// do not delete kubeconfig on "defer" call
	// only delete on "Down" call
	if deleteKubeconfig && md.cfg.KubeConfigPath != "" {
		rerr := os.RemoveAll(md.cfg.KubeConfigPath)
		md.lg.Info("deleted kubeconfig", zap.Error(rerr))
	}

	_, err := md.eks.DeleteCluster(&awseks.DeleteClusterInput{
		Name: aws.String(md.cfg.ClusterName),
	})
	if err != nil && !isEKSDeletedGoClient(err) {
		md.cfg.ClusterState.Status = err.Error()
		return err
	}

	// usually takes 5-minute
	md.lg.Info("waiting for 4-minute")
	time.Sleep(4 * time.Minute)

	retryStart := time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 15*time.Minute {
		var do *awseks.DescribeClusterOutput
		do, err = md.eks.DescribeCluster(&awseks.DescribeClusterInput{
			Name: aws.String(md.cfg.ClusterName),
		})
		if err == nil {
			md.cfg.ClusterState.Status = *do.Cluster.Status
			md.cfg.ClusterState.Created = *do.Cluster.CreatedAt
			md.cfg.ClusterState.PlatformVersion = *do.Cluster.PlatformVersion
			md.cfg.Sync()

			md.lg.Info("deleting cluster",
				zap.String("status", md.cfg.ClusterState.Status),
				zap.String("created-ago", humanize.Time(md.cfg.ClusterState.Created)),
				zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
			)

			if md.cfg.LogAutoUpload {
				if err = md.upload(); err != nil {
					md.lg.Warn("failed to upload", zap.Error(err))
				}
			}

			time.Sleep(30 * time.Second)
			continue
		}

		if isEKSDeletedGoClient(err) {
			err = nil
			md.cfg.ClusterState.Status = "DELETE_COMPLETE"
			break
		}
		md.cfg.ClusterState.Status = err.Error()
		md.cfg.Sync()

		md.lg.Warn("failed to describe cluster", zap.String("name", md.cfg.ClusterName), zap.Error(err))
		time.Sleep(30 * time.Second)
	}

	if err != nil {
		md.lg.Warn("failed to delete cluster",
			zap.String("name", md.cfg.ClusterName),
			zap.String("status", md.cfg.ClusterState.Status),
			zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
			zap.Error(err),
		)
		return err
	}

	md.lg.Info("deleted cluster",
		zap.String("name", md.cfg.ClusterName),
		zap.String("status", md.cfg.ClusterState.Status),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)
	return md.cfg.Sync()
}
