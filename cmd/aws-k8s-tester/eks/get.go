package eks

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-k8s-tester/pkg/awsapi"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func newGet() *cobra.Command {
	ac := &cobra.Command{
		Use:   "get <subcommand>",
		Short: "Get EKS resources",
	}
	ac.PersistentFlags().StringVar(&region, "region", "us-west-2", "EKS region")
	ac.PersistentFlags().StringVar(&kubernetesVersion, "kubernetes-version", "1.14", "EKS version for optimized AMI")
	ac.PersistentFlags().StringVar(&amiType, "ami-type", "amazon-linux-2", "'amazon-linux-2' or 'amazon-linux-2-gpu'")
	ac.AddCommand(
		newGetWorkerNodeAMI(),
	)
	return ac
}

func newGetWorkerNodeAMI() *cobra.Command {
	return &cobra.Command{
		Use:   "worker-node-ami",
		Short: "Get EKS worker node AMI",
		Run:   getWorkerNodeAMI,
	}
}

type workerNodeAMISSM struct {
	SchemaVersion string `json:"schema_version"`
	ImageID       string `json:"image_id"`
	ImageName     string `json:"image_name"`
}

func getWorkerNodeAMI(cmd *cobra.Command, args []string) {
	lg, _ := logutil.GetDefaultZapLogger()
	awsCfgEKS := &awsapi.Config{
		Logger: lg,
		Region: region,
	}
	ssSSM, _, _, err := awsapi.New(awsCfgEKS)
	if err != nil {
		panic(err)
	}
	svc := ssm.New(ssSSM)

	ssmKey := fmt.Sprintf("/aws/service/eks/optimized-ami/%s/%s/recommended", kubernetesVersion, amiType)
	lg.Info("getting SSM parameter to get latest worker node AMI", zap.String("ssm-key", ssmKey))
	so, err := svc.GetParameters(&ssm.GetParametersInput{
		Names: aws.StringSlice([]string{ssmKey}),
	})
	if err != nil {
		panic(fmt.Errorf("failed to get latest worker node AMI %v", err))
	}
	value := ""
	for _, pm := range so.Parameters {
		if *pm.Name != ssmKey {
			continue
		}
		value = *pm.Value
	}
	if value == "" {
		panic(fmt.Errorf("SSM key %q not found", ssmKey))
	}
	var output workerNodeAMISSM
	if err = json.Unmarshal([]byte(value), &output); err != nil {
		panic(err)
	}
	if output.ImageID == "" || output.ImageName == "" {
		panic(fmt.Errorf("latest worker node AMI not found (AMI %q, name %q)", output.ImageID, output.ImageName))
	}
	lg.Info("successfully got latest worker node AMI from SSM parameter",
		zap.String("worker-node-ami-type", amiType),
		zap.String("worker-node-ami-id", output.ImageID),
		zap.String("worker-node-ami-name", output.ImageName),
	)
}
