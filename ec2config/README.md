
```
*-------------------------------------------------------*-------------------*--------------------------------------------------*--------------------------*
|                ENVIRONMENTAL VARIABLE                 |     READ ONLY     |                       TYPE                       |         GO TYPE          |
*-------------------------------------------------------*-------------------*--------------------------------------------------*--------------------------*
| AWS_K8S_TESTER_EC2_UP                                 | read-only "false" | *ec2config.Config.Up                             | bool                     |
| AWS_K8S_TESTER_EC2_TIME_FRAME_CREATE                  | read-only "true"  | *ec2config.Config.TimeFrameCreate                | timeutil.TimeFrame       |
| AWS_K8S_TESTER_EC2_TIME_FRAME_DELETE                  | read-only "true"  | *ec2config.Config.TimeFrameDelete                | timeutil.TimeFrame       |
| AWS_K8S_TESTER_EC2_STATUS_CURRENT                     | read-only "false" | *ec2config.Config.StatusCurrent                  | string                   |
| AWS_K8S_TESTER_EC2_STATUS                             | read-only "false" | *ec2config.Config.Status                         | []ec2config.Status       |
| AWS_K8S_TESTER_EC2_DELETED_RESOURCES                  | read-only "false" | *ec2config.Config.DeletedResources               | map[string]string        |
| AWS_K8S_TESTER_EC2_NAME                               | read-only "false" | *ec2config.Config.Name                           | string                   |
| AWS_K8S_TESTER_EC2_PARTITION                          | read-only "false" | *ec2config.Config.Partition                      | string                   |
| AWS_K8S_TESTER_EC2_REGION                             | read-only "false" | *ec2config.Config.Region                         | string                   |
| AWS_K8S_TESTER_EC2_AVAILABILITY_ZONE_NAMES            | read-only "true"  | *ec2config.Config.AvailabilityZoneNames          | []string                 |
| AWS_K8S_TESTER_EC2_CONFIG_PATH                        | read-only "false" | *ec2config.Config.ConfigPath                     | string                   |
| AWS_K8S_TESTER_EC2_AWS_ACCOUNT_ID                     | read-only "true"  | *ec2config.Config.AWSAccountID                   | string                   |
| AWS_K8S_TESTER_EC2_AWS_USER_ID                        | read-only "true"  | *ec2config.Config.AWSUserID                      | string                   |
| AWS_K8S_TESTER_EC2_AWS_IAM_ROLE_ARN                   | read-only "true"  | *ec2config.Config.AWSIAMRoleARN                  | string                   |
| AWS_K8S_TESTER_EC2_AWS_CREDENTIAL_PATH                | read-only "true"  | *ec2config.Config.AWSCredentialPath              | string                   |
| AWS_K8S_TESTER_EC2_LOG_COLOR                          | read-only "false" | *ec2config.Config.LogColor                       | bool                     |
| AWS_K8S_TESTER_EC2_LOG_COLOR_OVERRIDE                 | read-only "false" | *ec2config.Config.LogColorOverride               | string                   |
| AWS_K8S_TESTER_EC2_LOG_LEVEL                          | read-only "false" | *ec2config.Config.LogLevel                       | string                   |
| AWS_K8S_TESTER_EC2_LOG_OUTPUTS                        | read-only "false" | *ec2config.Config.LogOutputs                     | []string                 |
| AWS_K8S_TESTER_EC2_ON_FAILURE_DELETE                  | read-only "false" | *ec2config.Config.OnFailureDelete                | bool                     |
| AWS_K8S_TESTER_EC2_ON_FAILURE_DELETE_WAIT_SECONDS     | read-only "false" | *ec2config.Config.OnFailureDeleteWaitSeconds     | uint64                   |
| AWS_K8S_TESTER_EC2_REMOTE_ACCESS_KEY_CREATE           | read-only "false" | *ec2config.Config.RemoteAccessKeyCreate          | bool                     |
| AWS_K8S_TESTER_EC2_REMOTE_ACCESS_KEY_NAME             | read-only "false" | *ec2config.Config.RemoteAccessKeyName            | string                   |
| AWS_K8S_TESTER_EC2_REMOTE_ACCESS_PRIVATE_KEY_PATH     | read-only "false" | *ec2config.Config.RemoteAccessPrivateKeyPath     | string                   |
| AWS_K8S_TESTER_EC2_REMOTE_ACCESS_COMMANDS_OUTPUT_PATH | read-only "false" | *ec2config.Config.RemoteAccessCommandsOutputPath | string                   |
| AWS_K8S_TESTER_EC2_ASGS_FETCH_LOGS                    | read-only "false" | *ec2config.Config.ASGsFetchLogs                  | bool                     |
| AWS_K8S_TESTER_EC2_ASGS_LOGS_DIR                      | read-only "false" | *ec2config.Config.ASGsLogsDir                    | string                   |
| AWS_K8S_TESTER_EC2_ASGS                               | read-only "false" | *ec2config.Config.ASGs                           | map[string]ec2config.ASG |
| AWS_K8S_TESTER_EC2_TOTAL_NODES                        | read-only "true"  | *ec2config.Config.TotalNodes                     | int32                    |
*-------------------------------------------------------*-------------------*--------------------------------------------------*--------------------------*


*-----------------------------------------------*-------------------*-------------------------------------*----------*
|            ENVIRONMENTAL VARIABLE             |     READ ONLY     |                TYPE                 | GO TYPE  |
*-----------------------------------------------*-------------------*-------------------------------------*----------*
| AWS_K8S_TESTER_EC2_ROLE_NAME                  | read-only "false" | *ec2config.Role.Name                | string   |
| AWS_K8S_TESTER_EC2_ROLE_CREATE                | read-only "false" | *ec2config.Role.Create              | bool     |
| AWS_K8S_TESTER_EC2_ROLE_ARN                   | read-only "false" | *ec2config.Role.ARN                 | string   |
| AWS_K8S_TESTER_EC2_ROLE_SERVICE_PRINCIPALS    | read-only "false" | *ec2config.Role.ServicePrincipals   | []string |
| AWS_K8S_TESTER_EC2_ROLE_MANAGED_POLICY_ARNS   | read-only "false" | *ec2config.Role.ManagedPolicyARNs   | []string |
| AWS_K8S_TESTER_EC2_ROLE_POLICY_NAME           | read-only "true"  | *ec2config.Role.PolicyName          | string   |
| AWS_K8S_TESTER_EC2_ROLE_POLICY_ARN            | read-only "true"  | *ec2config.Role.PolicyARN           | string   |
| AWS_K8S_TESTER_EC2_ROLE_INSTANCE_PROFILE_NAME | read-only "true"  | *ec2config.Role.InstanceProfileName | string   |
| AWS_K8S_TESTER_EC2_ROLE_INSTANCE_PROFILE_ARN  | read-only "true"  | *ec2config.Role.InstanceProfileARN  | string   |
*-----------------------------------------------*-------------------*-------------------------------------*----------*


*-------------------------------------------------------------------*-------------------*------------------------------------------------------*----------*
|                      ENVIRONMENTAL VARIABLE                       |     READ ONLY     |                         TYPE                         | GO TYPE  |
*-------------------------------------------------------------------*-------------------*------------------------------------------------------*----------*
| AWS_K8S_TESTER_EC2_VPC_CREATE                                     | read-only "false" | *ec2config.VPC.Create                                | bool     |
| AWS_K8S_TESTER_EC2_VPC_ID                                         | read-only "false" | *ec2config.VPC.ID                                    | string   |
| AWS_K8S_TESTER_EC2_VPC_SECURITY_GROUP_ID                          | read-only "true"  | *ec2config.VPC.SecurityGroupID                       | string   |
| AWS_K8S_TESTER_EC2_VPC_CIDRS                                      | read-only "false" | *ec2config.VPC.CIDRs                                 | []string |
| AWS_K8S_TESTER_EC2_VPC_PUBLIC_SUBNET_CIDRS                        | read-only "false" | *ec2config.VPC.PublicSubnetCIDRs                     | []string |
| AWS_K8S_TESTER_EC2_VPC_PUBLIC_SUBNET_IDS                          | read-only "true"  | *ec2config.VPC.PublicSubnetIDs                       | []string |
| AWS_K8S_TESTER_EC2_VPC_INTERNET_GATEWAY_ID                        | read-only "true"  | *ec2config.VPC.InternetGatewayID                     | string   |
| AWS_K8S_TESTER_EC2_VPC_PUBLIC_ROUTE_TABLE_ID                      | read-only "true"  | *ec2config.VPC.PublicRouteTableID                    | string   |
| AWS_K8S_TESTER_EC2_VPC_PUBLIC_SUBNET_ROUTE_TABLE_ASSOCIATION_IDS  | read-only "true"  | *ec2config.VPC.PublicSubnetRouteTableAssociationIDs  | []string |
| AWS_K8S_TESTER_EC2_VPC_EIP_ALLOCATION_IDS                         | read-only "true"  | *ec2config.VPC.EIPAllocationIDs                      | []string |
| AWS_K8S_TESTER_EC2_VPC_NAT_GATEWAY_IDS                            | read-only "true"  | *ec2config.VPC.NATGatewayIDs                         | []string |
| AWS_K8S_TESTER_EC2_VPC_PRIVATE_SUBNET_CIDRS                       | read-only "false" | *ec2config.VPC.PrivateSubnetCIDRs                    | []string |
| AWS_K8S_TESTER_EC2_VPC_PRIVATE_SUBNET_IDS                         | read-only "true"  | *ec2config.VPC.PrivateSubnetIDs                      | []string |
| AWS_K8S_TESTER_EC2_VPC_PRIVATE_ROUTE_TABLE_IDS                    | read-only "true"  | *ec2config.VPC.PrivateRouteTableIDs                  | []string |
| AWS_K8S_TESTER_EC2_VPC_PRIVATE_SUBNET_ROUTE_TABLE_ASSOCIATION_IDS | read-only "true"  | *ec2config.VPC.PrivateSubnetRouteTableAssociationIDs | []string |
| AWS_K8S_TESTER_EC2_VPC_DHCP_OPTIONS_DOMAIN_NAME                   | read-only "false" | *ec2config.VPC.DHCPOptionsDomainName                 | string   |
| AWS_K8S_TESTER_EC2_VPC_DHCP_OPTIONS_DOMAIN_NAME_SERVERS           | read-only "false" | *ec2config.VPC.DHCPOptionsDomainNameServers          | []string |
| AWS_K8S_TESTER_EC2_VPC_DHCP_OPTIONS_ID                            | read-only "true"  | *ec2config.VPC.DHCPOptionsID                         | string   |
*-------------------------------------------------------------------*-------------------*------------------------------------------------------*----------*


```
