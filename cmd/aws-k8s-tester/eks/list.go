package eks

import (
	"fmt"

	"github.com/aws/aws-k8s-tester/pkg/awsapi"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-sdk-go/aws"
	awseks "github.com/aws/aws-sdk-go/service/eks"
	"github.com/spf13/cobra"
)

func newList() *cobra.Command {
	ac := &cobra.Command{
		Use:   "list <subcommand>",
		Short: "List EKS resources",
	}
	ac.PersistentFlags().StringVarP(&region, "region", "r", "us-west-2", "EKS region")
	ac.PersistentFlags().StringVarP(&resolverURL, "resolver-url", "u", "", "EKS resolver endpoint URL")
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
	}
	ssEKS, _, _, err := awsapi.New(awsCfgEKS)
	if err != nil {
		panic(err)
	}
	svc := awseks.New(ssEKS)
	cnt := 0
	if err = svc.ListClustersPages(&awseks.ListClustersInput{},
		func(output *awseks.ListClustersOutput, lastPage bool) bool {
			for _, c := range output.Clusters {
				cnt++
				fmt.Printf("%03d: %q\n", cnt, aws.StringValue(c))
			}
			return true
		}); err != nil {
		panic(err)
	}
	if cnt == 0 {
		fmt.Println("'aws-k8s-tester eks list clusters' returned 0 cluster")
		return
	}

	fmt.Println("'aws-k8s-tester eks list clusters' returned", cnt, "clusters")
}
