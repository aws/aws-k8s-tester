package eks

import (
	"bytes"
	"text/template"
)

func createVPCTemplate(v vpcStack) (string, error) {
	tpl := template.Must(template.New("vpcStackTemplate").Parse(vpcStackTemplate))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, v); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type vpcStack struct {
	Description       string
	TagKey            string
	TagValue          string
	Hostname          string
	SecurityGroupName string

	// EnableHA is true to create 3 subnets
	EnableHA bool
}

// https://docs.aws.amazon.com/eks/latest/userguide/getting-started.html
// https://amazon-eks.s3-us-west-2.amazonaws.com/cloudformation/2018-08-30/amazon-eks-vpc-sample.yaml
const vpcStackTemplate = `---
AWSTemplateFormatVersion: '2010-09-09'
Description: {{ .Description }}

Parameters:

  VpcBlock:
    Type: String
    Default: 192.168.0.0/16
    Description: The CIDR range for the VPC. This should be a valid private (RFC 1918) CIDR range

  Subnet01Block:
    Type: String
    Default: {{ if .EnableHA }}192.168.64.0/18{{ else }}192.168.0.0/16{{ end }}
    Description: CidrBlock for subnet 01 within the VPC

{{ if .EnableHA }}
  Subnet02Block:
    Type: String
    Default: 192.168.128.0/18
    Description: CidrBlock for subnet 02 within the VPC

  Subnet03Block:
    Type: String
    Default: 192.168.192.0/18
    Description: CidrBlock for subnet 03 within the VPC
{{ end }}

Metadata:
  AWS::CloudFormation::Interface:
    ParameterGroups:
      -
        Label:
          default: "Worker Network Configuration"
        Parameters:
          - VpcBlock
          - Subnet01Block
          {{ if .EnableHA }}          - Subnet02Block
          - Subnet03Block{{ end }}

Resources:
  VPC:
    Type: AWS::EC2::VPC
    Properties:
      CidrBlock: !Ref VpcBlock
      EnableDnsSupport: true
      EnableDnsHostnames: true
      Tags:
      - Key: Name
        Value: !Sub '${AWS::StackName}-VPC'
      - Key: {{ .TagKey }}
        Value: {{ .TagValue }}
      - Key: HOSTNAME
        Value: {{ .Hostname }}

  InternetGateway:
    Type: "AWS::EC2::InternetGateway"
    Properties:
      Tags:
      - Key: {{ .TagKey }}
        Value: {{ .TagValue }}
      - Key: HOSTNAME
        Value: {{ .Hostname }}

  VPCGatewayAttachment:
    Type: "AWS::EC2::VPCGatewayAttachment"
    Properties:
      InternetGatewayId: !Ref InternetGateway
      VpcId: !Ref VPC

  RouteTable:
    Type: AWS::EC2::RouteTable
    Properties:
      VpcId: !Ref VPC
      Tags:
      - Key: Name
        Value: Public Subnets
      - Key: Network
        Value: Public
      - Key: {{ .TagKey }}
        Value: {{ .TagValue }}
      - Key: HOSTNAME
        Value: {{ .Hostname }}

  Route:
    DependsOn: VPCGatewayAttachment
    Type: AWS::EC2::Route
    Properties:
      RouteTableId: !Ref RouteTable
      DestinationCidrBlock: 0.0.0.0/0
      GatewayId: !Ref InternetGateway

  Subnet01:
    Type: AWS::EC2::Subnet
    Metadata:
      Comment: Subnet 01
    Properties:
      AvailabilityZone:
        Fn::Select:
        - '0'
        - Fn::GetAZs:
            Ref: AWS::Region
      CidrBlock:
        Ref: Subnet01Block
      VpcId:
        Ref: VPC
      Tags:
      - Key: Name
        Value: !Sub "${AWS::StackName}-Subnet01"
      - Key: {{ .TagKey }}
        Value: {{ .TagValue }}
      - Key: HOSTNAME
        Value: {{ .Hostname }}

{{ if .EnableHA }}
  Subnet02:
    Type: AWS::EC2::Subnet
    Metadata:
      Comment: Subnet 02
    Properties:
      AvailabilityZone:
        Fn::Select:
        - '1'
        - Fn::GetAZs:
            Ref: AWS::Region
      CidrBlock:
        Ref: Subnet02Block
      VpcId:
        Ref: VPC
      Tags:
      - Key: Name
        Value: !Sub "${AWS::StackName}-Subnet02"
      - Key: {{ .TagKey }}
        Value: {{ .TagValue }}
      - Key: HOSTNAME
        Value: {{ .Hostname }}

  Subnet03:
    Type: AWS::EC2::Subnet
    Metadata:
      Comment: Subnet 03
    Properties:
      AvailabilityZone:
        Fn::Select:
        - '2'
        - Fn::GetAZs:
            Ref: AWS::Region
      CidrBlock:
        Ref: Subnet03Block
      VpcId:
        Ref: VPC
      Tags:
      - Key: Name
        Value: !Sub "${AWS::StackName}-Subnet03"
      - Key: {{ .TagKey }}
        Value: {{ .TagValue }}
      - Key: HOSTNAME
        Value: {{ .Hostname }}
{{ end }}

  Subnet01RouteTableAssociation:
    Type: AWS::EC2::SubnetRouteTableAssociation
    Properties:
      SubnetId: !Ref Subnet01
      RouteTableId: !Ref RouteTable

{{ if .EnableHA }}
  Subnet02RouteTableAssociation:
    Type: AWS::EC2::SubnetRouteTableAssociation
    Properties:
      SubnetId: !Ref Subnet02
      RouteTableId: !Ref RouteTable

  Subnet03RouteTableAssociation:
    Type: AWS::EC2::SubnetRouteTableAssociation
    Properties:
      SubnetId: !Ref Subnet03
      RouteTableId: !Ref RouteTable
{{ end }}

  ControlPlaneSecurityGroup:
    Type: AWS::EC2::SecurityGroup
    Properties:
      GroupName: {{ .SecurityGroupName }}
      GroupDescription: Cluster communication with worker nodes
      VpcId: !Ref VPC
      Tags:
      - Key: {{ .TagKey }}
        Value: {{ .TagValue }}
      - Key: HOSTNAME
        Value: {{ .Hostname }}

Outputs:

  SubnetIds:
    Description: All subnets in the VPC
    Value: {{ if .EnableHA }}!Join [ ",", [ !Ref Subnet01, !Ref Subnet02, !Ref Subnet03 ] ]{{ else }}!Ref Subnet01{{ end }}

  SecurityGroups:
    Description: Security group for the cluster control plane communication with worker nodes
    Value: !Join [ ",", [ !Ref ControlPlaneSecurityGroup ] ]

  VpcId:
    Description: The VPC Id
    Value: !Ref VPC

`
