package cluster

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-k8s-tester/eks/cluster/wait"
	wait_v2 "github.com/aws/aws-k8s-tester/eks/cluster/wait-v2"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-k8s-tester/pkg/user"
	"github.com/aws/aws-k8s-tester/version"
	aws_v2 "github.com/aws/aws-sdk-go-v2/aws"
	aws_eks_v2 "github.com/aws/aws-sdk-go-v2/service/eks"
	aws_eks_v2_types "github.com/aws/aws-sdk-go-v2/service/eks/types"
	aws_eks "github.com/aws/aws-sdk-go/service/eks"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

// see https://github.com/aws/aws-k8s-tester/blob/v1.6.0/eks/cluster/cluster.go for CloudFormation based workflow

const (
	ClusterCreateTimeout = time.Hour
	ClusterDeleteTimeout = time.Hour
)

func (ts *tester) createEKS() (err error) {
	fmt.Print(ts.cfg.EKSConfig.Colorize("\n\n[yellow]*********************************\n"))
	fmt.Printf(ts.cfg.EKSConfig.Colorize("[light_green]createEKS [default](%q)\n"), ts.cfg.EKSConfig.ConfigPath)

	if ts.cfg.EKSConfig.Status.ClusterARN != "" ||
		ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint != "" ||
		ts.cfg.EKSConfig.Status.ClusterCA != "" ||
		ts.cfg.EKSConfig.Status.ClusterCADecoded != "" {
		ts.cfg.Logger.Info("non-empty cluster given; no need to create a new one", zap.String("status", ts.cfg.EKSConfig.Status.ClusterStatusCurrent))
		return nil
	}
	if ts.cfg.EKSConfig.Status.Up {
		ts.cfg.Logger.Info("cluster is up; no need to create cluster")
		return nil
	}

	ts.describeCluster()
	if ts.cfg.EKSConfig.Status.ClusterStatusCurrent == fmt.Sprint(aws_eks_v2_types.ClusterStatusActive) {
		ts.cfg.Logger.Info("cluster status is active; no need to create cluster", zap.String("status", ts.cfg.EKSConfig.Status.ClusterStatusCurrent))
		return nil
	}

	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.Status.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()
	initialWait := 9 * time.Minute

	subnets := make([]string, len(ts.cfg.EKSConfig.VPC.PublicSubnetIDs))
	copy(subnets, ts.cfg.EKSConfig.VPC.PublicSubnetIDs)
	if len(ts.cfg.EKSConfig.VPC.PrivateSubnetIDs) > 0 {
		subnets = append(subnets, ts.cfg.EKSConfig.VPC.PrivateSubnetIDs...)
	}

	ts.cfg.Logger.Info("creating a cluster using EKS API",
		zap.String("name", ts.cfg.EKSConfig.Name),
		zap.Bool("eks-v2-sdk", ts.useV2SDK),
		zap.String("resolver-url", ts.cfg.EKSConfig.ResolverURL),
		zap.String("signing-name", ts.cfg.EKSConfig.SigningName),
		zap.String("request-header-key", ts.cfg.EKSConfig.RequestHeaderKey),
		zap.String("request-header-value", ts.cfg.EKSConfig.RequestHeaderValue),
	)

	if ts.useV2SDK {
		createInput := &aws_eks_v2.CreateClusterInput{
			Name:    aws_v2.String(ts.cfg.EKSConfig.Name),
			Version: aws_v2.String(ts.cfg.EKSConfig.Version),
			RoleArn: aws_v2.String(ts.cfg.EKSConfig.Role.ARN),
			ResourcesVpcConfig: &aws_eks_v2_types.VpcConfigRequest{
				SubnetIds:        subnets,
				SecurityGroupIds: []string{ts.cfg.EKSConfig.VPC.SecurityGroupID},
			},
			Tags: map[string]string{
				"Kind":                   "aws-k8s-tester",
				"aws-k8s-tester-version": version.ReleaseVersion,
				"User":                   user.Get(),
			},
		}
		for k, v := range ts.cfg.EKSConfig.Tags {
			createInput.Tags[k] = v
			ts.cfg.Logger.Info("added EKS tag to EKS API request",
				zap.String("key", k),
				zap.String("value", v),
			)
		}
		if ts.cfg.EKSConfig.Encryption.CMKARN != "" {
			ts.cfg.Logger.Info("added encryption to EKS API request",
				zap.String("cmk-arn", ts.cfg.EKSConfig.Encryption.CMKARN),
			)
			createInput.EncryptionConfig = []aws_eks_v2_types.EncryptionConfig{
				{
					Resources: []string{"secrets"},
					Provider: &aws_eks_v2_types.Provider{
						KeyArn: aws_v2.String(ts.cfg.EKSConfig.Encryption.CMKARN),
					},
				},
			}
		}
		opts := make([]func(*aws_eks_v2.Options), 0)
		if ts.cfg.EKSConfig.RequestHeaderKey != "" && ts.cfg.EKSConfig.RequestHeaderValue != "" {
			ts.cfg.Logger.Info("set request header for EKS create request",
				zap.String("key", ts.cfg.EKSConfig.RequestHeaderKey),
				zap.String("value", ts.cfg.EKSConfig.RequestHeaderValue),
			)
			opts = append(opts, func(op *aws_eks_v2.Options) {
				op.HTTPClient = &httpClientWithRequestHeader{
					cli:            op.HTTPClient,
					reqHeaderKey:   ts.cfg.EKSConfig.RequestHeaderKey,
					reqHeaderValue: ts.cfg.EKSConfig.RequestHeaderValue,
				}
			})
		}
		_, err = ts.cfg.EKSAPIV2.CreateCluster(context.Background(), createInput, opts...)
		if err != nil {
			return err
		}
	} else {
		createInput := &aws_eks.CreateClusterInput{
			Name:    aws_v2.String(ts.cfg.EKSConfig.Name),
			Version: aws_v2.String(ts.cfg.EKSConfig.Version),
			RoleArn: aws_v2.String(ts.cfg.EKSConfig.Role.ARN),
			ResourcesVpcConfig: &aws_eks.VpcConfigRequest{
				SubnetIds:        aws_v2.StringSlice(subnets),
				SecurityGroupIds: aws_v2.StringSlice([]string{ts.cfg.EKSConfig.VPC.SecurityGroupID}),
			},
			Tags: map[string]*string{
				"Kind":                   aws_v2.String("aws-k8s-tester"),
				"aws-k8s-tester-version": aws_v2.String(version.ReleaseVersion),
				"User":                   aws_v2.String(user.Get()),
			},
		}
		for k, v := range ts.cfg.EKSConfig.Tags {
			createInput.Tags[k] = aws_v2.String(v)
			ts.cfg.Logger.Info("added EKS tag to EKS API request",
				zap.String("key", k),
				zap.String("value", v),
			)
		}
		if ts.cfg.EKSConfig.Encryption.CMKARN != "" {
			ts.cfg.Logger.Info("added encryption to EKS API request",
				zap.String("cmk-arn", ts.cfg.EKSConfig.Encryption.CMKARN),
			)
			createInput.EncryptionConfig = []*aws_eks.EncryptionConfig{
				{
					Resources: aws_v2.StringSlice([]string{"secrets"}),
					Provider: &aws_eks.Provider{
						KeyArn: aws_v2.String(ts.cfg.EKSConfig.Encryption.CMKARN),
					},
				},
			}
		}
		req, _ := ts.cfg.EKSAPI.CreateClusterRequest(createInput)
		if ts.cfg.EKSConfig.RequestHeaderKey != "" && ts.cfg.EKSConfig.RequestHeaderValue != "" {
			req.HTTPRequest.Header[ts.cfg.EKSConfig.RequestHeaderKey] = []string{ts.cfg.EKSConfig.RequestHeaderValue}
			ts.cfg.Logger.Info("set request header for EKS create request",
				zap.String("key", ts.cfg.EKSConfig.RequestHeaderKey),
				zap.String("value", ts.cfg.EKSConfig.RequestHeaderValue),
			)
		}
		err = req.Send()
		if err != nil {
			return err
		}
	}

	ts.cfg.Logger.Info("sent create cluster request")
	ctx, cancel := context.WithTimeout(context.Background(), ClusterCreateTimeout)
	if ts.useV2SDK {
		ch := wait_v2.Poll(
			ctx,
			ts.cfg.Stopc,
			ts.cfg.Logger,
			ts.cfg.LogWriter,
			ts.cfg.EKSAPIV2,
			ts.cfg.EKSConfig.Name,
			aws_eks.ClusterStatusActive,
			initialWait,
			30*time.Second,
		)
		for sv := range ch {
			ts.updateClusterStatusV2(sv, aws_eks.ClusterStatusActive)
			err = sv.Error
		}
	} else {
		ch := wait.Poll(
			ctx,
			ts.cfg.Stopc,
			ts.cfg.Logger,
			ts.cfg.LogWriter,
			ts.cfg.EKSAPI,
			ts.cfg.EKSConfig.Name,
			aws_eks.ClusterStatusActive,
			initialWait,
			30*time.Second,
		)
		for sv := range ch {
			ts.updateClusterStatusV1(sv, aws_eks.ClusterStatusActive)
			err = sv.Error
		}
	}
	cancel()

	switch err {
	case nil:
		ts.cfg.Logger.Info("created a cluster",
			zap.String("cluster-arn", ts.cfg.EKSConfig.Status.ClusterARN),
			zap.String("cluster-api-server-endpoint", ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint),
			zap.Int("cluster-ca-bytes", len(ts.cfg.EKSConfig.Status.ClusterCA)),
			zap.String("config-path", ts.cfg.EKSConfig.ConfigPath),
			zap.String("started", humanize.RelTime(createStart, time.Now(), "ago", "from now")),
		)

	case context.DeadlineExceeded:
		ts.cfg.Logger.Warn("cluster creation took too long",
			zap.String("cluster-arn", ts.cfg.EKSConfig.Status.ClusterARN),
			zap.String("cluster-api-server-endpoint", ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint),
			zap.String("config-path", ts.cfg.EKSConfig.ConfigPath),
			zap.String("started", humanize.RelTime(createStart, time.Now(), "ago", "from now")),
			zap.Error(err),
		)
		return err

	default:
		ts.cfg.Logger.Warn("failed to create cluster",
			zap.String("cluster-arn", ts.cfg.EKSConfig.Status.ClusterARN),
			zap.String("cluster-api-server-endpoint", ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint),
			zap.String("config-path", ts.cfg.EKSConfig.ConfigPath),
			zap.String("started", humanize.RelTime(createStart, time.Now(), "ago", "from now")),
			zap.Error(err),
		)
		return err
	}

	ts.cfg.EKSConfig.Sync()
	return nil
}

type httpClientWithRequestHeader struct {
	cli            aws_eks_v2.HTTPClient
	reqHeaderKey   string
	reqHeaderValue string
}

func (h *httpClientWithRequestHeader) Do(req *http.Request) (*http.Response, error) {
	if req == nil {
		req.Header[h.reqHeaderKey] = []string{h.reqHeaderValue}
	}
	return h.cli.Do(req)
}

// deleteEKS returns error if EKS cluster delete fails.
// It returns nil if the cluster has already been deleted.
func (ts *tester) deleteEKS() error {
	fmt.Print(ts.cfg.EKSConfig.Colorize("\n\n\n[yellow]*********************************\n"))
	fmt.Printf(ts.cfg.EKSConfig.Colorize("[light_blue]deleteEKS [default](%q)\n"), ts.cfg.EKSConfig.ConfigPath)

	ts.describeCluster()
	if ts.cfg.EKSConfig.Status.ClusterStatusCurrent == "" || ts.cfg.EKSConfig.Status.ClusterStatusCurrent == eksconfig.ClusterStatusDELETEDORNOTEXIST {
		ts.cfg.Logger.Info("cluster already deleted; no need to delete cluster")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.Status.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	ts.cfg.Logger.Info("deleting cluster", zap.String("cluster-name", ts.cfg.EKSConfig.Name))

	_, err := ts.cfg.EKSAPI.DeleteCluster(&aws_eks.DeleteClusterInput{
		Name: aws_v2.String(ts.cfg.EKSConfig.Name),
	})
	if err != nil {
		if wait_v2.IsDeleted(err) {
			ts.cfg.Logger.Warn("cluster is already deleted", zap.Error(err))
			ts.cfg.EKSConfig.RecordStatus(eksconfig.ClusterStatusDELETEDORNOTEXIST)
			ts.cfg.EKSConfig.Status.Up = false
			ts.cfg.EKSConfig.Sync()
			return nil
		}

		ts.cfg.Logger.Warn("failed to delete cluster", zap.Error(err))
		ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to delete cluster (%v)", err))
		return err
	}

	ts.cfg.EKSConfig.Status.Up = false
	ts.cfg.EKSConfig.Sync()

	ctx, cancel := context.WithTimeout(context.Background(), ClusterDeleteTimeout)
	if ts.useV2SDK {
		csCh := wait_v2.Poll(
			ctx,
			make(chan struct{}), // do not exit on stop
			ts.cfg.Logger,
			ts.cfg.LogWriter,
			ts.cfg.EKSAPIV2,
			ts.cfg.EKSConfig.Name,
			eksconfig.ClusterStatusDELETEDORNOTEXIST,
			5*time.Minute,
			20*time.Second,
		)
		for v := range csCh {
			ts.updateClusterStatusV2(v, eksconfig.ClusterStatusDELETEDORNOTEXIST)
		}
	} else {
		csCh := wait.Poll(
			ctx,
			make(chan struct{}), // do not exit on stop
			ts.cfg.Logger,
			ts.cfg.LogWriter,
			ts.cfg.EKSAPI,
			ts.cfg.EKSConfig.Name,
			eksconfig.ClusterStatusDELETEDORNOTEXIST,
			5*time.Minute,
			20*time.Second,
		)
		for v := range csCh {
			ts.updateClusterStatusV1(v, eksconfig.ClusterStatusDELETEDORNOTEXIST)
		}
	}
	cancel()

	ts.cfg.Logger.Info("deleted a cluster",
		zap.String("cluster-name", ts.cfg.EKSConfig.Name),
	)
	ts.cfg.EKSConfig.Sync()
	return nil
}

func (ts *tester) describeCluster() {
	emptyCluster, status := false, ""
	var err error

	if ts.useV2SDK {
		dout, derr := ts.cfg.EKSAPIV2.DescribeCluster(
			context.Background(),
			&aws_eks_v2.DescribeClusterInput{
				Name: aws_v2.String(ts.cfg.EKSConfig.Name),
			},
		)
		err = derr

		emptyCluster = dout == nil || (dout != nil && dout.Cluster == nil)
		if dout != nil && dout.Cluster != nil {
			status = fmt.Sprint(dout.Cluster.Status)
		}
	} else {
		dout, derr := ts.cfg.EKSAPI.DescribeCluster(
			&aws_eks.DescribeClusterInput{
				Name: aws_v2.String(ts.cfg.EKSConfig.Name),
			},
		)
		err = derr

		emptyCluster = dout == nil || (dout != nil && dout.Cluster == nil)
		if dout != nil && dout.Cluster != nil {
			status = aws_v2.ToString(dout.Cluster.Status)
		}
	}

	if err != nil {
		if wait.IsDeleted(err) || wait_v2.IsDeleted(err) {
			ts.cfg.EKSConfig.RecordStatus(eksconfig.ClusterStatusDELETEDORNOTEXIST)
			return
		}

		ts.cfg.Logger.Warn("failed to describe cluster", zap.Error(err))
		ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to describe cluster (%v)", err))
		return
	}

	if emptyCluster {
		ts.cfg.EKSConfig.RecordStatus(eksconfig.ClusterStatusDELETEDORNOTEXIST)
	} else {
		ts.cfg.EKSConfig.RecordStatus(status)
	}

	ts.cfg.Logger.Info("described cluster",
		zap.String("name", ts.cfg.EKSConfig.Name),
		zap.String("status", ts.cfg.EKSConfig.Status.ClusterStatusCurrent),
	)
}
