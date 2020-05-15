
```
*-----------------------------------------------------------*-------------------*-----------------------------------------------------*--------------------------*
|                  ENVIRONMENTAL VARIABLE                   |     READ ONLY     |                        TYPE                         |         GO TYPE          |
*-----------------------------------------------------------*-------------------*-----------------------------------------------------*--------------------------*
|                  AWS_K8S_TESTER_EC2_NAME                  | read-only "false" |               *ec2config.Config.Name                |          string          |
|               AWS_K8S_TESTER_EC2_PARTITION                | read-only "false" |             *ec2config.Config.Partition             |          string          |
|                 AWS_K8S_TESTER_EC2_REGION                 | read-only "false" |              *ec2config.Config.Region               |          string          |
|              AWS_K8S_TESTER_EC2_CONFIG_PATH               | read-only "false" |            *ec2config.Config.ConfigPath             |          string          |
|             AWS_K8S_TESTER_EC2_AWS_ACCOUNT_ID             | read-only "true"  |           *ec2config.Config.AWSAccountID            |          string          |
|              AWS_K8S_TESTER_EC2_AWS_USER_ID               | read-only "true"  |             *ec2config.Config.AWSUserID             |          string          |
|            AWS_K8S_TESTER_EC2_AWS_IAM_ROLE_ARN            | read-only "true"  |           *ec2config.Config.AWSIAMRoleARN           |          string          |
|          AWS_K8S_TESTER_EC2_AWS_CREDENTIAL_PATH           | read-only "true"  |         *ec2config.Config.AWSCredentialPath         |          string          |
|              AWS_K8S_TESTER_EC2_CREATE_TOOK               | read-only "true"  |            *ec2config.Config.CreateTook             |      time.Duration       |
|           AWS_K8S_TESTER_EC2_CREATE_TOOK_STRING           | read-only "true"  |         *ec2config.Config.CreateTookString          |          string          |
|              AWS_K8S_TESTER_EC2_DELETE_TOOK               | read-only "true"  |            *ec2config.Config.DeleteTook             |      time.Duration       |
|           AWS_K8S_TESTER_EC2_DELETE_TOOK_STRING           | read-only "true"  |         *ec2config.Config.DeleteTookString          |          string          |
|        AWS_K8S_TESTER_EC2_TIME_UTC_CREATE_COMPLETE        | read-only "true"  |       *ec2config.Config.TimeUTCCreateComplete       |        time.Time         |
| AWS_K8S_TESTER_EC2_TIME_UTC_CREATE_COMPLETE_RFC3339_MICRO | read-only "true"  | *ec2config.Config.TimeUTCCreateCompleteRFC3339Micro |          string          |
|         AWS_K8S_TESTER_EC2_TIME_UTC_DELETE_START          | read-only "true"  |        *ec2config.Config.TimeUTCDeleteStart         |        time.Time         |
|  AWS_K8S_TESTER_EC2_TIME_UTC_DELETE_START_RFC3339_MICRO   | read-only "true"  |  *ec2config.Config.TimeUTCDeleteStartRFC3339Micro   |          string          |
|               AWS_K8S_TESTER_EC2_LOG_LEVEL                | read-only "false" |             *ec2config.Config.LogLevel              |          string          |
|              AWS_K8S_TESTER_EC2_LOG_OUTPUTS               | read-only "false" |            *ec2config.Config.LogOutputs             |         []string         |
|                   AWS_K8S_TESTER_EC2_UP                   | read-only "false" |                *ec2config.Config.Up                 |           bool           |
|             AWS_K8S_TESTER_EC2_STATUS_CURRENT             | read-only "false" |           *ec2config.Config.StatusCurrent           |          string          |
|                 AWS_K8S_TESTER_EC2_STATUS                 | read-only "false" |              *ec2config.Config.Status               |    []ec2config.Status    |
|           AWS_K8S_TESTER_EC2_ON_FAILURE_DELETE            | read-only "false" |          *ec2config.Config.OnFailureDelete          |           bool           |
|     AWS_K8S_TESTER_EC2_ON_FAILURE_DELETE_WAIT_SECONDS     | read-only "false" |    *ec2config.Config.OnFailureDeleteWaitSeconds     |          uint64          |
|            AWS_K8S_TESTER_EC2_S3_BUCKET_CREATE            | read-only "false" |          *ec2config.Config.S3BucketCreate           |           bool           |
|         AWS_K8S_TESTER_EC2_S3_BUCKET_CREATE_KEEP          | read-only "false" |        *ec2config.Config.S3BucketCreateKeep         |           bool           |
|             AWS_K8S_TESTER_EC2_S3_BUCKET_NAME             | read-only "false" |           *ec2config.Config.S3BucketName            |          string          |
|  AWS_K8S_TESTER_EC2_S3_BUCKET_LIFECYCLE_EXPIRATION_DAYS   | read-only "false" |  *ec2config.Config.S3BucketLifecycleExpirationDays  |          int64           |
|               AWS_K8S_TESTER_EC2_ROLE_NAME                | read-only "false" |             *ec2config.Config.RoleName              |          string          |
|              AWS_K8S_TESTER_EC2_ROLE_CREATE               | read-only "false" |            *ec2config.Config.RoleCreate             |           bool           |
|                AWS_K8S_TESTER_EC2_ROLE_ARN                | read-only "false" |              *ec2config.Config.RoleARN              |          string          |
|        AWS_K8S_TESTER_EC2_ROLE_SERVICE_PRINCIPALS         | read-only "false" |       *ec2config.Config.RoleServicePrincipals       |         []string         |
|        AWS_K8S_TESTER_EC2_ROLE_MANAGED_POLICY_ARNS        | read-only "false" |       *ec2config.Config.RoleManagedPolicyARNs       |         []string         |
|           AWS_K8S_TESTER_EC2_ROLE_CFN_STACK_ID            | read-only "true"  |          *ec2config.Config.RoleCFNStackID           |          string          |
|               AWS_K8S_TESTER_EC2_VPC_CREATE               | read-only "false" |             *ec2config.Config.VPCCreate             |           bool           |
|                 AWS_K8S_TESTER_EC2_VPC_ID                 | read-only "false" |               *ec2config.Config.VPCID               |          string          |
|            AWS_K8S_TESTER_EC2_VPC_CFN_STACK_ID            | read-only "true"  |           *ec2config.Config.VPCCFNStackID           |          string          |
|         AWS_K8S_TESTER_EC2_SSH_INGRESS_IPV4_RANGE         | read-only "false" |        *ec2config.Config.SSHIngressIPv4Range        |          string          |
|                AWS_K8S_TESTER_EC2_VPC_CIDR                | read-only "false" |              *ec2config.Config.VPCCIDR              |          string          |
|          AWS_K8S_TESTER_EC2_PUBLIC_SUBNET_CIDR_1          | read-only "false" |         *ec2config.Config.PublicSubnetCIDR1         |          string          |
|          AWS_K8S_TESTER_EC2_PUBLIC_SUBNET_CIDR_2          | read-only "false" |         *ec2config.Config.PublicSubnetCIDR2         |          string          |
|          AWS_K8S_TESTER_EC2_PUBLIC_SUBNET_CIDR_3          | read-only "false" |         *ec2config.Config.PublicSubnetCIDR3         |          string          |
|         AWS_K8S_TESTER_EC2_PRIVATE_SUBNET_CIDR_1          | read-only "false" |        *ec2config.Config.PrivateSubnetCIDR1         |          string          |
|         AWS_K8S_TESTER_EC2_PRIVATE_SUBNET_CIDR_2          | read-only "false" |        *ec2config.Config.PrivateSubnetCIDR2         |          string          |
|           AWS_K8S_TESTER_EC2_PUBLIC_SUBNET_IDS            | read-only "true"  |          *ec2config.Config.PublicSubnetIDs          |         []string         |
|           AWS_K8S_TESTER_EC2_PRIVATE_SUBNET_IDS           | read-only "true"  |         *ec2config.Config.PrivateSubnetIDs          |         []string         |
|        AWS_K8S_TESTER_EC2_DHCP_OPTIONS_DOMAIN_NAME        | read-only "false" |       *ec2config.Config.DHCPOptionsDomainName       |          string          |
|    AWS_K8S_TESTER_EC2_DHCP_OPTIONS_DOMAIN_NAME_SERVERS    | read-only "false" |   *ec2config.Config.DHCPOptionsDomainNameServers    |         []string         |
|           AWS_K8S_TESTER_EC2_SECURITY_GROUP_ID            | read-only "true"  |          *ec2config.Config.SecurityGroupID          |          string          |
|        AWS_K8S_TESTER_EC2_REMOTE_ACCESS_KEY_CREATE        | read-only "false" |       *ec2config.Config.RemoteAccessKeyCreate       |           bool           |
|         AWS_K8S_TESTER_EC2_REMOTE_ACCESS_KEY_NAME         | read-only "false" |        *ec2config.Config.RemoteAccessKeyName        |          string          |
|     AWS_K8S_TESTER_EC2_REMOTE_ACCESS_PRIVATE_KEY_PATH     | read-only "false" |    *ec2config.Config.RemoteAccessPrivateKeyPath     |          string          |
|   AWS_K8S_TESTER_EC2_REMOTE_ACCESS_COMMANDS_OUTPUT_PATH   | read-only "false" |  *ec2config.Config.RemoteAccessCommandsOutputPath   |          string          |
|            AWS_K8S_TESTER_EC2_ASGS_FETCH_LOGS             | read-only "false" |           *ec2config.Config.ASGsFetchLogs           |           bool           |
|             AWS_K8S_TESTER_EC2_ASGS_LOGS_DIR              | read-only "false" |            *ec2config.Config.ASGsLogsDir            |          string          |
|                  AWS_K8S_TESTER_EC2_ASGS                  | read-only "false" |               *ec2config.Config.ASGs                | map[string]ec2config.ASG |
*-----------------------------------------------------------*-------------------*-----------------------------------------------------*--------------------------*
```
