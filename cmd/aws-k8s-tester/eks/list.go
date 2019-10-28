package eks

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/awsapi"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	awseks "github.com/aws/aws-sdk-go/service/eks"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

var deletePrefix string

func newList() *cobra.Command {
	ac := &cobra.Command{
		Use:   "list <subcommand>",
		Short: "List EKS resources",
	}
	ac.PersistentFlags().StringVar(&region, "region", "us-west-2", "EKS region")
	ac.PersistentFlags().StringVar(&resolverURL, "resolver-url", "", "EKS resolver endpoint URL")
	ac.PersistentFlags().StringVar(&signingName, "signing-name", "", "EKS signing name")
	ac.PersistentFlags().BoolVar(&more, "more", false, "'true' to query all the details")
	ac.PersistentFlags().BoolVar(&deleteFailed, "delete-failed", false, "'true' to clean up failed clusters")
	ac.PersistentFlags().StringVar(&deletePrefix, "delete-prefix", "", "Cluster name prefix to match and delete")
	ac.AddCommand(
		newListClusters(),
	)
	return ac
}

func newListClusters() *cobra.Command {
	return &cobra.Command{
		Use:   "clusters",
		Short: "List EKS clusters",
		Run:   listClustersFunc,
	}
}

func listClustersFunc(cmd *cobra.Command, args []string) {
	lg, _ := logutil.GetDefaultZapLogger()
	awsCfgEKS := &awsapi.Config{
		Logger:      lg,
		Region:      region,
		ResolverURL: resolverURL,
		SigningName: signingName,
	}
	ssEKS, _, _, err := awsapi.New(awsCfgEKS)
	if err != nil {
		panic(err)
	}
	svc := awseks.New(ssEKS)
	clusterNames := make([]string, 0)
	if err = svc.ListClustersPages(&awseks.ListClustersInput{},
		func(output *awseks.ListClustersOutput, lastPage bool) bool {
			for _, name := range output.Clusters {
				clusterNames = append(clusterNames, aws.StringValue(name))
			}
			return true
		}); err != nil {
		panic(err)
	}
	if len(clusterNames) == 0 {
		fmt.Println("'aws-k8s-tester eks list clusters' returned 0 cluster")
		return
	}

	for i, name := range clusterNames {
		if !more {
			fmt.Printf("%03d: %q\n", i, name)
			continue
		}

		out, err := svc.DescribeCluster(&awseks.DescribeClusterInput{
			Name: aws.String(name),
		})
		if err != nil {
			fmt.Printf("%03d: %q failed to describe (%v, retriable %v, throttled %v, error type %v)\n",
				i,
				name,
				err,
				request.IsErrorRetryable(err),
				request.IsErrorThrottle(err),
				reflect.TypeOf(err),
			)

			awsErr, ok := err.(awserr.Error)
			if ok && awsErr.Code() == "ResourceNotFoundException" &&
				strings.HasPrefix(awsErr.Message(), "No cluster found for") {
				fmt.Printf("deleting %q (reason: %v)\n", name, err)
				_, derr := svc.DeleteCluster(&awseks.DeleteClusterInput{Name: aws.String(name)})
				fmt.Println("deleted", name, derr)
			}

			time.Sleep(30 * time.Millisecond)
			continue
		}
		if out.Cluster == nil {
			panic(fmt.Errorf("%03d: %q empty cluster", i, name))
		}

		clus := out.Cluster
		fmt.Printf("%03d: %q [created %v (%q), version %q, status %q, IAM Role %q, VPC %q]\n",
			i,
			name,
			aws.TimeValue(clus.CreatedAt),
			humanize.RelTime(aws.TimeValue(clus.CreatedAt), time.Now().UTC(), "ago", "from now"),
			aws.StringValue(clus.Version),
			aws.StringValue(clus.Status),
			aws.StringValue(clus.RoleArn),
			aws.StringValue(clus.ResourcesVpcConfig.VpcId),
		)

		if deleteFailed && aws.StringValue(clus.Status) == "FAILED" {
			fmt.Printf("deleting %q (reason: %v)\n", name, aws.StringValue(clus.Status))
			_, derr := svc.DeleteCluster(&awseks.DeleteClusterInput{Name: aws.String(name)})
			fmt.Println("deleted", name, derr)
			continue
		}

		if len(deletePrefix) > 0 && strings.HasPrefix(name, deletePrefix) {
			fmt.Printf("deleting %q (reason: %q)\n", name, deletePrefix)
			_, derr := svc.DeleteCluster(&awseks.DeleteClusterInput{Name: aws.String(name)})
			fmt.Println("deleted", name, derr)
		}

		time.Sleep(3 * time.Second)
	}
	fmt.Println("'aws-k8s-tester eks list clusters' returned", len(clusterNames), "clusters")
}
