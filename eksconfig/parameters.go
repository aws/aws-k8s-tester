package eksconfig

// Parameters defines parameters for EKS "cluster" creation.
type Parameters struct {
	// RoleName is the name of cluster role.
	RoleName string `json:"role-name"`
	// RoleCreate is true to auto-create and delete cluster role.
	RoleCreate bool `json:"role-create"`
	// RoleARN is the role ARN that EKS uses to create AWS resources for Kubernetes.
	// By default, it's empty which triggers tester to create one.
	RoleARN string `json:"role-arn"`
	// RoleServicePrincipals is the EKS Role Service Principals
	RoleServicePrincipals []string `json:"role-service-principals"`
	// RoleManagedPolicyARNs is EKS Role managed policy ARNs.
	RoleManagedPolicyARNs []string `json:"role-managed-policy-arns"`
	RoleCFNStackID        string   `json:"role-cfn-stack-id" read-only:"true"`

	// Tags defines EKS create cluster tags.
	Tags map[string]string `json:"tags"`
	// RequestHeaderKey defines EKS create cluster request header key.
	RequestHeaderKey string `json:"request-header-key"`
	// RequestHeaderValue defines EKS create cluster request header value.
	RequestHeaderValue string `json:"request-header-value"`

	// ResolverURL defines an AWS resolver endpoint for EKS API.
	// Must be left empty to use production EKS service.
	ResolverURL string `json:"resolver-url"`
	// SigningName is the EKS create request signing name.
	SigningName string `json:"signing-name"`

	// VPCCreate is true to auto-create and delete VPC.
	VPCCreate bool `json:"vpc-create"`
	// VPCID is the VPC ID for cluster creation.
	// If not empty, VPC is reused and not deleted.
	// If empty, VPC is created anew and deleted on cluster deletion.
	VPCID         string `json:"vpc-id"`
	VPCCFNStackID string `json:"vpc-cfn-stack-id" read-only:"true"`
	// VpcCIDR is the IP range (CIDR notation) for VPC, must be a valid private
	// (RFC 1918) CIDR range.
	VPCCIDR string `json:"vpc-cidr,omitempty"`
	// PublicSubnetCIDR1 is the CIDR Block for subnet 1 within the VPC.
	PublicSubnetCIDR1 string `json:"public-subnet-cidr-1,omitempty"`
	// PublicSubnetCIDR2 is the CIDR Block for subnet 2 within the VPC.
	PublicSubnetCIDR2 string `json:"public-subnet-cidr-2,omitempty"`
	// PublicSubnetCIDR3 is the CIDR Block for subnet 3 within the VPC.
	PublicSubnetCIDR3 string `json:"public-subnet-cidr-3,omitempty"`
	// PrivateSubnetCIDR1 is the CIDR Block for subnet 1 within the VPC.
	PrivateSubnetCIDR1 string `json:"private-subnet-cidr-1,omitempty"`
	// PrivateSubnetCIDR2 is the CIDR Block for subnet 2 within the VPC.
	PrivateSubnetCIDR2 string `json:"private-subnet-cidr-2,omitempty"`
	// PublicSubnetIDs is the list of all public subnets in the VPC.
	PublicSubnetIDs []string `json:"public-subnet-ids" read-only:"true"`
	// PrivateSubnetIDs is the list of all private subnets in the VPC.
	PrivateSubnetIDs []string `json:"private-subnet-ids" read-only:"true"`

	// DHCPOptionsDomainName is used to complete unqualified DNS hostnames for VPC.
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-dhcp-options.html
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/cluster-endpoint.html
	DHCPOptionsDomainName string `json:"dhcp-options-domain-name"`
	// DHCPOptionsDomainNameServers is a list of strings.
	// The IPv4 addresses of up to four domain name servers, or AmazonProvidedDNS, for VPC.
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-dhcp-options.html
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/cluster-endpoint.html
	DHCPOptionsDomainNameServers []string `json:"dhcp-options-domain-name-servers"`

	// Version is the version of EKS Kubernetes "cluster".
	// If empty, set default version.
	Version      string  `json:"version"`
	VersionValue float64 `json:"version-value" read-only:"true"`

	// EncryptionCMKCreate is true to auto-create and delete KMS CMK
	// for encryption feature.
	EncryptionCMKCreate bool `json:"encryption-cmk-create"`
	// EncryptionCMKARN is the KMS CMK ARN for encryption feature.
	// If not empty, the cluster is created with encryption feature
	// enabled.
	EncryptionCMKARN string `json:"encryption-cmk-arn"`
}
