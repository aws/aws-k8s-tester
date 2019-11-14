package cfn

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-k8s-tester/eksconfig"
	awsapi_cfn "github.com/aws/aws-k8s-tester/pkg/awsapi/cloudformation"
	awsapi_iam "github.com/aws/aws-k8s-tester/pkg/awsapi/iam"
	goformation "github.com/awslabs/goformation/v3"
	gocfn "github.com/awslabs/goformation/v3/cloudformation"
	"github.com/awslabs/goformation/v3/cloudformation/ec2"
	"github.com/awslabs/goformation/v3/cloudformation/iam"
	"github.com/awslabs/goformation/v3/intrinsics"
)

const (
	// ParameterVpcBlock is the CloudFormation parameter for VPC block.
	ParameterVpcBlock = "VpcBlock"
	// ParameterSubnet01Block is the CloudFormation parameter for subnet block.
	ParameterSubnet01Block = "Subnet01Block"
	// ParameterSubnet02Block is the CloudFormation parameter for subnet block.
	ParameterSubnet02Block = "Subnet02Block"
	// ParameterSubnet03Block is the CloudFormation parameter for subnet block.
	ParameterSubnet03Block = "Subnet03Block"

	// ResourceKeyServiceRole is the CloudFormation resource key for EKS service IAM role.
	ResourceKeyServiceRole = "AWSServiceRoleForAmazonEKS"
	// ResourceKeyVPC is the CloudFormation resource key for VPC.
	ResourceKeyVPC = "VPC"
	// ResourceKeyInternetGateway is the CloudFormation resource key for InternetGateway.
	ResourceKeyInternetGateway = "InternetGateway"
	// ResourceKeyVPCGatewayAttachment is the CloudFormation resource key for VPC Gateway attachment.
	ResourceKeyVPCGatewayAttachment = "VPCGatewayAttachment"
	// ResourceKeyRouteTable is the CloudFormation resource key for RouteTable.
	ResourceKeyRouteTable = "RouteTable"
	// ResourceKeyRoute is the CloudFormation resource key for Route.
	ResourceKeyRoute = "Route"
	// ResourceKeySubnet01 is the CloudFormation resource key for Subnet01.
	ResourceKeySubnet01 = "Subnet01"
	// ResourceKeySubnet02 is the CloudFormation resource key for Subnet02.
	ResourceKeySubnet02 = "Subnet02"
	// ResourceKeySubnet03 is the CloudFormation resource key for Subnet03.
	ResourceKeySubnet03 = "Subnet03"
	// ResourceKeySubnet01RouteTableAssociation is the CloudFormation resource key for Subnet01RouteTableAssociation.
	ResourceKeySubnet01RouteTableAssociation = "Subnet01RouteTableAssociation"
	// ResourceKeySubnet02RouteTableAssociation is the CloudFormation resource key for Subnet02RouteTableAssociation.
	ResourceKeySubnet02RouteTableAssociation = "Subnet02RouteTableAssociation"
	// ResourceKeySubnet03RouteTableAssociation is the CloudFormation resource key for Subnet03RouteTableAssociation.
	ResourceKeySubnet03RouteTableAssociation = "Subnet03RouteTableAssociation"
	// ResourceKeyControlPlaneSecurityGroup is the CloudFormation resource key for security group.
	ResourceKeyControlPlaneSecurityGroup = "ControlPlaneSecurityGroup"

	// OutputKeyRoleArn is the output key of the Role ARN.
	OutputKeyRoleArn = "RoleArn"
	// OutputKeySubnetIds is the output key of the subnet IDs.
	OutputKeySubnetIds = "SubnetIds"
	// OutputKeySecurityGroups is the output key of the security group IDs.
	OutputKeySecurityGroups = "SecurityGroups"
	// OutputKeyVpcId is the output key of the VPC ID.
	OutputKeyVpcId = "VpcId"
)

// NewTemplate returns the new template.
//
// Reference:
//  - https://docs.aws.amazon.com/eks/latest/userguide/service_IAM_role.html
//  - https://docs.aws.amazon.com/eks/latest/userguide/create-public-private-vpc.html
//  - https://amazon-eks.s3-us-west-2.amazonaws.com/cloudformation/2018-08-30/amazon-eks-vpc-sample.yaml
//
func NewTemplate(cfg *eksconfig.Config) (tp *gocfn.Template, err error) {
	tp = gocfn.NewTemplate()
	tp.Description = "Amazon EKS"

	tp.Parameters = map[string]interface{}{
		ParameterVpcBlock: awsapi_cfn.Parameter{
			Type:        "String",
			Default:     "192.168.0.0/16",
			Description: "The CIDR range for the VPC. This should be a valid private (RFC 1918) CIDR range.",
		},
		ParameterSubnet01Block: awsapi_cfn.Parameter{
			Type:        "String",
			Default:     "192.168.64.0/18",
			Description: "CidrBlock for subnet 01 within the VPC",
		},
		ParameterSubnet02Block: awsapi_cfn.Parameter{
			Type:        "String",
			Default:     "192.168.128.0/18",
			Description: "CidrBlock for subnet 02 within the VPC",
		},
		ParameterSubnet03Block: awsapi_cfn.Parameter{
			Type:        "String",
			Default:     "192.168.192.0/18",
			Description: "CidrBlock for subnet 03 within the VPC. This is used only if the region has more than 2 AZs.",
		},
	}

	tp.Metadata = map[string]interface{}{
		"AWS::CloudFormation::Interface": awsapi_cfn.Interface{
			ParameterGroups: []awsapi_cfn.ParameterGroupEntry{
				{
					Label: map[string]string{
						"default": "Worker Network Configuration",
					},
					Parameters: []string{
						ParameterVpcBlock,
						ParameterSubnet01Block,
						ParameterSubnet02Block,
						ParameterSubnet03Block,
					},
				},
			},
		},
	}

	tp.Conditions = awsapi_cfn.NewConditionsForAZText()

	tp.Resources = map[string]gocfn.Resource{
		ResourceKeyServiceRole: newRole(cfg),

		ResourceKeyVPC: newVPC([]string{
			ResourceKeyServiceRole,
		}),

		ResourceKeyInternetGateway: newInternetGateway([]string{
			ResourceKeyServiceRole,
		}),

		ResourceKeyVPCGatewayAttachment: newVPCGatewayAttachment([]string{
			ResourceKeyServiceRole,
			ResourceKeyVPC,
			ResourceKeyInternetGateway,
		}),

		ResourceKeyRouteTable: newRouteTable([]string{
			ResourceKeyServiceRole,
			ResourceKeyVPC,
		}),

		ResourceKeyRoute: newRoute([]string{
			ResourceKeyServiceRole,
			ResourceKeyVPC,
			ResourceKeyRouteTable,
			ResourceKeyInternetGateway,
		}),

		ResourceKeySubnet01: newSubnet(0, ParameterSubnet01Block, "", []string{
			ResourceKeyServiceRole,
			ResourceKeyVPC,
			ResourceKeyRouteTable,
			ResourceKeyInternetGateway,
		}),
		ResourceKeySubnet02: newSubnet(1, ParameterSubnet02Block, "", []string{
			ResourceKeyServiceRole,
			ResourceKeyVPC,
			ResourceKeyRouteTable,
			ResourceKeyInternetGateway,
		}),
		ResourceKeySubnet03: newSubnet(2, ParameterSubnet03Block, awsapi_cfn.ConditionKeyHasMoreThan2Azs, []string{
			ResourceKeyServiceRole,
			ResourceKeyVPC,
			ResourceKeyRouteTable,
			ResourceKeyInternetGateway,
		}),

		ResourceKeySubnet01RouteTableAssociation: newSubnetRouteTableAssociation(ResourceKeySubnet01,
			"",
			[]string{
				ResourceKeyServiceRole,
				ResourceKeyVPC,
				ResourceKeyRouteTable,
			}),
		ResourceKeySubnet02RouteTableAssociation: newSubnetRouteTableAssociation(ResourceKeySubnet02,
			"",
			[]string{
				ResourceKeyServiceRole,
				ResourceKeyVPC,
				ResourceKeyRouteTable,
			}),
		ResourceKeySubnet03RouteTableAssociation: newSubnetRouteTableAssociation(ResourceKeySubnet03,
			awsapi_cfn.ConditionKeyHasMoreThan2Azs,
			[]string{
				ResourceKeyServiceRole,
				ResourceKeyVPC,
				ResourceKeyRouteTable,
			}),

		ResourceKeyControlPlaneSecurityGroup: newControlPlaneSecurityGroup([]string{
			ResourceKeyServiceRole,
			ResourceKeyVPC,
		}),
	}

	tp.Outputs = map[string]interface{}{
		OutputKeyRoleArn: awsapi_cfn.Output{
			Description: "The role that EKS will use to create AWS resources for Kubernetes clusters",
			Value:       gocfn.GetAtt(ResourceKeyServiceRole, "Arn"),
			// Export: map[string]string{
			// 	"Name": gocfn.Sub(fmt.Sprintf(`${%s}-%s`, awsapi_cfn.PseudoParameterStackName, OutputKeyRoleArn)),
			// },
		},

		OutputKeySubnetIds: awsapi_cfn.Output{
			Description: "All subnets in the VPC",
			Value: gocfn.If(
				awsapi_cfn.ConditionKeyHasMoreThan2Azs,
				gocfn.Join(",", []string{
					gocfn.Ref(ResourceKeySubnet01),
					gocfn.Ref(ResourceKeySubnet02),
					gocfn.Ref(ResourceKeySubnet03),
				}),
				gocfn.Join(",", []string{
					gocfn.Ref(ResourceKeySubnet01),
					gocfn.Ref(ResourceKeySubnet02),
				}),
			),
		},
		OutputKeySecurityGroups: awsapi_cfn.Output{
			Description: "All subnets in the VPC",
			Value:       gocfn.Ref(ResourceKeyVPC),
		},
	}

	return tp, nil
}

func newRole(cfg *eksconfig.Config) gocfn.Resource {
	return &iam.Role{
		AssumeRolePolicyDocument: awsapi_iam.PolicyDocument{
			Version: "2012-10-17",
			Statement: []awsapi_iam.StatementEntry{
				{
					Principal: &awsapi_iam.PrincipalEntry{
						Service: []string{
							"eks.amazonaws.com",
							"eks-beta-pdx.aws.internal",
							"eks-dev.aws.internal",
						},
					},
					Effect: "Allow",
					Action: []string{
						"sts:AssumeRole",
					},
				},
			},
		},
		ManagedPolicyArns: []string{
			"arn:aws:iam::aws:policy/AmazonEKSServicePolicy",
			"arn:aws:iam::aws:policy/AmazonEKSClusterPolicy",
		},
	}
}

func newVPC(dependencies []string) gocfn.Resource {
	rsc := &ec2.VPC{
		CidrBlock:          gocfn.Ref(ParameterVpcBlock),
		EnableDnsSupport:   true,
		EnableDnsHostnames: true,
		Tags: awsapi_cfn.NewTags(map[string]string{
			"Kind": "aws-k8s-tester",
			"Name": gocfn.Sub(fmt.Sprintf(`${%s}-%s`, awsapi_cfn.PseudoParameterStackName, ResourceKeyVPC)),
		}),
	}
	rsc.SetDependsOn(dependencies)
	return rsc
}

func newInternetGateway(dependencies []string) gocfn.Resource {
	rsc := &ec2.InternetGateway{
		Tags: awsapi_cfn.NewTags(map[string]string{
			"Kind": "aws-k8s-tester",
			"Name": gocfn.Sub(fmt.Sprintf(`${%s}-%s`, awsapi_cfn.PseudoParameterStackName, ResourceKeyInternetGateway)),
		}),
	}
	rsc.SetDependsOn(dependencies)
	return rsc
}

func newVPCGatewayAttachment(dependencies []string) gocfn.Resource {
	rsc := &ec2.VPCGatewayAttachment{
		InternetGatewayId: gocfn.Ref(ResourceKeyInternetGateway),
		VpcId:             gocfn.Ref(ResourceKeyVPC),
	}
	rsc.SetDependsOn(dependencies)
	return rsc
}

func newRouteTable(dependencies []string) gocfn.Resource {
	rsc := &ec2.RouteTable{
		VpcId: gocfn.Ref(ResourceKeyVPC),
		Tags: awsapi_cfn.NewTags(map[string]string{
			"Kind":    "aws-k8s-tester",
			"Name":    gocfn.Sub(fmt.Sprintf(`${%s}-%s`, awsapi_cfn.PseudoParameterStackName, ResourceKeyRouteTable)),
			"Network": "Public",
		}),
	}
	rsc.SetDependsOn(dependencies)
	return rsc
}

func newRoute(dependencies []string) gocfn.Resource {
	rsc := &ec2.Route{
		RouteTableId:         gocfn.Ref(ResourceKeyRouteTable),
		DestinationCidrBlock: "0.0.0.0/0",
		GatewayId:            gocfn.Ref(ResourceKeyInternetGateway),
	}
	rsc.SetDependsOn(dependencies)
	return rsc
}

func newSubnet(idx int, cidrBlockRef string, condition string, dependencies []string) gocfn.Resource {
	rsc := &ec2.Subnet{
		AvailabilityZone: gocfn.Select(
			0,
			[]string{gocfn.GetAZs(gocfn.Ref(awsapi_cfn.PseudoParameterRegion))},
		),
		CidrBlock: gocfn.Ref(cidrBlockRef),
		VpcId:     gocfn.Ref(ResourceKeyVPC),
		Tags: awsapi_cfn.NewTags(map[string]string{
			"Kind":    "aws-k8s-tester",
			"Name":    gocfn.Sub(fmt.Sprintf(`${%s}-%s`, awsapi_cfn.PseudoParameterStackName, ResourceKeySubnet01)),
			"Network": "Public",
		}),
	}
	rsc.SetDependsOn(dependencies)
	if len(condition) > 0 {
		// TODO: this requires custom patch
		rsc.SetResourceCondition(condition)
	}
	return rsc
}

func newSubnetRouteTableAssociation(subnetResourceKey string, condition string, dependencies []string) gocfn.Resource {
	rsc := &ec2.SubnetRouteTableAssociation{
		SubnetId:     gocfn.Ref(subnetResourceKey),
		RouteTableId: gocfn.Ref(ResourceKeyRouteTable),
	}
	rsc.SetDependsOn(dependencies)
	if len(condition) > 0 {
		// TODO: this requires custom patch
		rsc.SetResourceCondition(condition)
	}
	return rsc
}

func newControlPlaneSecurityGroup(dependencies []string) gocfn.Resource {
	rsc := &ec2.SecurityGroup{
		VpcId:            gocfn.Ref(ResourceKeyVPC),
		GroupDescription: "Cluster communication with worker nodes",
	}
	rsc.SetDependsOn(dependencies)
	return rsc
}

func reparseTemplate(t *gocfn.Template) (*gocfn.Template, error) {
	j, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return nil, err
	}

	rendered, err := intrinsics.ProcessJSON(j, &intrinsics.ProcessorOptions{
		NoProcess: true,
	})
	if err != nil {
		return nil, err
	}
	return goformation.ParseJSONWithOptions(
		rendered, &intrinsics.ProcessorOptions{
			IntrinsicHandlerOverrides: gocfn.EncoderIntrinsics,
		},
	)
}
