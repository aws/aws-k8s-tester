package config

import "github.com/aws/aws-sdk-go/service/ec2"

// Instance represents an EC2 instance.
type Instance struct {
	ImageID             string               `json:"image-id,omitempty"`
	InstanceID          string               `json:"instance-id,omitempty"`
	InstanceType        string               `json:"instance-type,omitempty"`
	KeyName             string               `json:"key-name,omitempty"`
	Placement           Placement            `json:"placement,omitempty"`
	PrivateDNSName      string               `json:"private-dns-name,omitempty"`
	PrivateIP           string               `json:"private-ip,omitempty"`
	PublicDNSName       string               `json:"public-dns-name,omitempty"`
	PublicIP            string               `json:"public-ip,omitempty"`
	State               State                `json:"state,omitempty"`
	SubnetID            string               `json:"subnet-id,omitempty"`
	VPCID               string               `json:"vpc-id,omitempty"`
	BlockDeviceMappings []BlockDeviceMapping `json:"block-device-mappings,omitempty"`
	EBSOptimized        bool                 `json:"ebs-optimized"`
	RootDeviceName      string               `json:"root-device-name,omitempty"`
	RootDeviceType      string               `json:"root-device-type,omitempty"`
	SecurityGroups      []SecurityGroup      `json:"security-groups,omitempty"`
}

// Placement defines EC2 placement.
type Placement struct {
	AvailabilityZone string `json:"availability-zone,omitempty"`
	Tenancy          string `json:"tenancy,omitempty"`
}

// State defines an EC2 state.
type State struct {
	Code int64  `json:"code,omitempty"`
	Name string `json:"name,omitempty"`
}

// BlockDeviceMapping defines a block device mapping.
type BlockDeviceMapping struct {
	DeviceName string `json:"device-name,omitempty"`
	EBS        EBS    `json:"ebs,omitempty"`
}

// EBS defines an EBS volume.
type EBS struct {
	DeleteOnTermination bool   `json:"delete-on-termination,omitempty"`
	Status              string `json:"status,omitempty"`
	VolumeID            string `json:"volume-id,omitempty"`
}

// SecurityGroup defines a security group.
type SecurityGroup struct {
	GroupName string `json:"group-name,omitempty"`
	GroupID   string `json:"group-id,omitempty"`
}

// ConvertEC2Instance converts "aws ec2 describe-instances" to "config.Instance".
func ConvertEC2Instance(iv *ec2.Instance) (instance Instance) {
	instance = Instance{
		ImageID:      *iv.ImageId,
		InstanceID:   *iv.InstanceId,
		InstanceType: *iv.InstanceType,
		KeyName:      *iv.KeyName,
		Placement: Placement{
			AvailabilityZone: *iv.Placement.AvailabilityZone,
			Tenancy:          *iv.Placement.Tenancy,
		},
		PrivateDNSName: *iv.PrivateDnsName,
		PrivateIP:      *iv.PrivateIpAddress,
		PublicDNSName:  *iv.PublicDnsName,
		PublicIP:       *iv.PublicIpAddress,
		State: State{
			Code: *iv.State.Code,
			Name: *iv.State.Name,
		},
		SubnetID:            *iv.SubnetId,
		VPCID:               *iv.VpcId,
		BlockDeviceMappings: make([]BlockDeviceMapping, len(iv.BlockDeviceMappings)),
		EBSOptimized:        *iv.EbsOptimized,
		RootDeviceName:      *iv.RootDeviceName,
		RootDeviceType:      *iv.RootDeviceType,
		SecurityGroups:      make([]SecurityGroup, len(iv.SecurityGroups)),
	}
	for j := range iv.BlockDeviceMappings {
		instance.BlockDeviceMappings[j] = BlockDeviceMapping{
			DeviceName: *iv.BlockDeviceMappings[j].DeviceName,
			EBS: EBS{
				DeleteOnTermination: *iv.BlockDeviceMappings[j].Ebs.DeleteOnTermination,
				Status:              *iv.BlockDeviceMappings[j].Ebs.Status,
				VolumeID:            *iv.BlockDeviceMappings[j].Ebs.VolumeId,
			},
		}
	}
	for j := range iv.SecurityGroups {
		instance.SecurityGroups[j] = SecurityGroup{
			GroupName: *iv.SecurityGroups[j].GroupName,
			GroupID:   *iv.SecurityGroups[j].GroupId,
		}
	}
	return instance
}
