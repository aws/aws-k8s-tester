package eksconfig

import (
	"fmt"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-sdk-go/service/eks"
)

func init() {
	if ec2config.AMITypeAL2X8664 != eks.AMITypesAl2X8664 {
		panic(fmt.Errorf("ec2config.AMITypeAL2X8664 %q != eks.AMITypesAl2X8664 %q", ec2config.AMITypeAL2X8664, eks.AMITypesAl2X8664))
	}
	if ec2config.AMITypeAL2X8664GPU != eks.AMITypesAl2X8664Gpu {
		panic(fmt.Errorf("ec2config.AMITypeAL2X8664GPU %q != eks.AMITypesAl2X8664Gpu %q", ec2config.AMITypeAL2X8664GPU, eks.AMITypesAl2X8664Gpu))
	}
}
