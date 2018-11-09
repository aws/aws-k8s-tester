package eks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	awseks "github.com/aws/aws-sdk-go/service/eks"
	humanize "github.com/dustin/go-humanize"
	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

func (md *embedded) createCluster() error {
	if md.cfg.ClusterName == "" {
		return errors.New("cannot create empty cluster")
	}
	if md.cfg.ClusterState.ServiceRoleWithPolicyARN == "" {
		return errors.New("can't create cluster without service role ARN")
	}
	if len(md.cfg.SubnetIDs) == 0 {
		return errors.New("can't create cluster without subnet IDs")
	}
	if md.cfg.SecurityGroupID == "" {
		return errors.New("can't create cluster without security group ID")
	}

	now := time.Now().UTC()

	_, err := md.eks.CreateCluster(&awseks.CreateClusterInput{
		Name:    aws.String(md.cfg.ClusterName),
		Version: aws.String(md.cfg.KubernetesVersion),
		RoleArn: aws.String(md.cfg.ClusterState.ServiceRoleWithPolicyARN),
		ResourcesVpcConfig: &awseks.VpcConfigRequest{
			SubnetIds:        aws.StringSlice(md.cfg.SubnetIDs),
			SecurityGroupIds: aws.StringSlice([]string{md.cfg.SecurityGroupID}),
		},
	})
	if err != nil {
		return err
	}
	md.cfg.ClusterState.StatusClusterCreated = true
	md.cfg.ClusterState.Status = "CREATING"
	md.cfg.Sync()

	if md.cfg.UploadTesterLogs {
		if err = md.uploadTesterLogs(); err != nil {
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
		md.cfg.PlatformVersion = *do.Cluster.PlatformVersion
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

		if md.cfg.UploadTesterLogs {
			if err = md.uploadTesterLogs(); err != nil {
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
		md.awsIAMAuthenticatorPath,
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

	time.Sleep(5 * time.Second)

	retryStart = time.Now().UTC()
	txt, done := "", false
	for time.Now().UTC().Sub(retryStart) < 5*time.Minute {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		var out1 []byte
		out1, err = exec.New().CommandContext(ctx,
			md.kubectlPath,
			"--kubeconfig="+md.cfg.KubeConfigPath,
			"version",
		).CombinedOutput()
		cancel()
		md.lg.Info("ran kubectl version",
			zap.String("kubectl-path", md.kubectlPath),
			zap.String("aws-iam-authenticator-path", md.awsIAMAuthenticatorPath),
			zap.String("output", string(out1)),
			zap.Error(err),
		)

		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		var out2 []byte
		out2, err = exec.New().CommandContext(ctx,
			md.kubectlPath,
			"--kubeconfig="+md.cfg.KubeConfigPath,
			"cluster-info",
		).CombinedOutput()
		cancel()
		md.lg.Info("ran kubectl cluster-info",
			zap.String("kubectl-path", md.kubectlPath),
			zap.String("aws-iam-authenticator-path", md.awsIAMAuthenticatorPath),
			zap.String("output", string(out2)),
			zap.Error(err),
		)

		if err == nil &&
			strings.Contains(string(out1), "-eks") &&
			strings.Contains(string(out2), "is running") {
			done = true
			break
		}

		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		var out3 []byte
		out3, err = exec.New().CommandContext(ctx,
			md.kubectlPath,
			"--kubeconfig="+md.cfg.KubeConfigPath,
			"cluster-info",
			"dump",
		).CombinedOutput()
		cancel()
		md.lg.Info("ran kubectl cluster-info dump",
			zap.String("kubectl-path", md.kubectlPath),
			zap.String("aws-iam-authenticator-path", md.awsIAMAuthenticatorPath),
			zap.String("output", string(out3)),
			zap.Error(err),
		)

		time.Sleep(10 * time.Second)
	}
	if err != nil || !done {
		return fmt.Errorf("'kubectl get all' output unexpected: %s (%v)", txt, err)
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
			md.cfg.PlatformVersion = *do.Cluster.PlatformVersion
			md.cfg.Sync()

			md.lg.Info("deleting cluster",
				zap.String("status", md.cfg.ClusterState.Status),
				zap.String("created-ago", humanize.RelTime(md.cfg.ClusterState.Created, time.Now().UTC(), "ago", "from now")),
				zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
			)

			if md.cfg.UploadTesterLogs {
				if err = md.uploadTesterLogs(); err != nil {
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
