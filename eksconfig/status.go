package eksconfig

import (
	"encoding/json"
	"fmt"
	"time"

	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	aws_eks "github.com/aws/aws-sdk-go/service/eks"
)

// Status represents the current status of AWS resources.
// Read-only. Cannot be configured via environmental variables.
type Status struct {
	// Up is true if the cluster is up.
	Up bool `json:"up"`

	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	// ServerVersionInfo is the server version from EKS kube-apiserver.
	ServerVersionInfo k8s_client.ServerVersionInfo `json:"server-version-info" read-only:"true"`

	// AWSAccountID is the account ID of the eks tester caller session.
	AWSAccountID string `json:"aws-account-id"`
	// AWSUserID is the user ID of the eks tester caller session.
	AWSUserID string `json:"aws-user-id"`
	// AWSIAMRoleARN is the user IAM Role ARN of the eks tester caller session.
	AWSIAMRoleARN string `json:"aws-iam-role-arn"`
	// AWSCredentialPath is automatically set via AWS SDK Go.
	// And to be mounted as a volume as 'Secret' object.
	AWSCredentialPath string `json:"aws-credential-path"`

	ClusterARN                  string `json:"cluster-arn"`
	ClusterCFNStackID           string `json:"cluster-cfn-stack-id"`
	ClusterCFNStackYAMLFilePath string `json:"cluster-cfn-stack-yaml-file-path" read-only:"true"`

	// ClusterControlPlaneSecurityGroupID is the security group ID for the cluster control
	// plane communication with worker nodes.
	ClusterControlPlaneSecurityGroupID string `json:"cluster-control-plane-security-group-id"`
	// ClusterAPIServerEndpoint is the cluster endpoint of the EKS cluster,
	// required for KUBECONFIG write.
	ClusterAPIServerEndpoint string `json:"cluster-api-server-endpoint"`
	// ClusterOIDCIssuerURL is the issuer URL for the OpenID Connect
	// (https://openid.net/connect/) identity provider .
	ClusterOIDCIssuerURL string `json:"cluster-oidc-issuer-url"`
	// ClusterOIDCIssuerHostPath is the issuer host path.
	ClusterOIDCIssuerHostPath string `json:"cluster-oidc-issuer-host-path"`
	// ClusterOIDCIssuerARN is the issuer ARN for the OpenID Connect
	// (https://openid.net/connect/) identity provider .
	ClusterOIDCIssuerARN string `json:"cluster-oidc-issuer-arn"`
	// ClusterOIDCIssuerCAThumbprint is the issuer CA thumbprint.
	ClusterOIDCIssuerCAThumbprint string `json:"cluster-oidc-issuer-ca-thumbprint"`

	// ClusterCA is the EKS cluster CA, required for KUBECONFIG write.
	ClusterCA string `json:"cluster-ca"`
	// ClusterCADecoded is the decoded EKS cluster CA, required for k8s.io/client-go.
	ClusterCADecoded string `json:"cluster-ca-decoded"`

	// ClusterStatusCurrent represents the current status of the cluster.
	ClusterStatusCurrent string `json:"cluster-status-current"`
	// ClusterStatus represents the status of the cluster.
	ClusterStatus []ClusterStatus `json:"cluster-status"`

	// PrivateDNSToSSHConfig maps each worker node's private IP to its public IP,
	// public DNS, and SSH access user name.
	// Worker node name in AWS is the node's EC2 instance private DNS.
	// This is used for SSH access.
	PrivateDNSToSSHConfig map[string]SSHConfig `json:"private-dns-to-ssh-config"`
}

// SSHConfig represents basic SSH access configuration for worker nodes.
type SSHConfig struct {
	PublicIP      string `json:"public-ip"`
	PublicDNSName string `json:"public-dns-name"`
	UserName      string `json:"user-name"`
}

func (sc SSHConfig) ToString() string {
	b, err := json.Marshal(sc)
	if err != nil {
		return fmt.Sprintf("%+v", sc)
	}
	return string(b)
}

/*
map all private IPs to public IP + public DNS
map node name to internal ip, private ip
pod node name to internal ip ->
*/

// ClusterStatus represents the cluster status.
type ClusterStatus struct {
	Time   time.Time `json:"time"`
	Status string    `json:"status"`
}

// ClusterStatusDELETEDORNOTEXIST defines the cluster status when the cluster is not found.
//
// ref. https://docs.aws.amazon.com/eks/latest/APIReference/API_Cluster.html#AmazonEKS-Type-Cluster-status
//
//  CREATING
//  ACTIVE
//  UPDATING
//  DELETING
//  FAILED
//
const ClusterStatusDELETEDORNOTEXIST = "DELETED/NOT-EXIST"

// RecordStatus records cluster status.
func (cfg *Config) RecordStatus(status string) {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()

	if cfg.Status == nil {
		cfg.Status = &Status{}
	}
	cfg.Status.ClusterStatusCurrent = status
	switch status {
	case ClusterStatusDELETEDORNOTEXIST:
		cfg.Status.Up = false
	case aws_eks.ClusterStatusActive:
		cfg.Status.Up = true
	}

	sv := ClusterStatus{Time: time.Now(), Status: status}
	n := len(cfg.Status.ClusterStatus)
	if n == 0 {
		cfg.Status.ClusterStatus = []ClusterStatus{sv}
		cfg.unsafeSync()
		return
	}

	copied := make([]ClusterStatus, n+1)
	copy(copied[1:], cfg.Status.ClusterStatus)
	copied[0] = sv
	cfg.Status.ClusterStatus = copied
	cfg.unsafeSync()
}
