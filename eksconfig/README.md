
```
# total 34 add-ons
# set the following *_ENABLE env vars to enable add-ons, rest are set with default values
AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_METRICS_SERVER_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_APP_MESH_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_CSI_EBS_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_KUBERNETES_DASHBOARD_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_PROMETHEUS_GRAFANA_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_NLB_GUESTBOOK_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_JOBS_PI_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_JOBS_ECHO_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_CSRS_LOCAL_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_LOCAL_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_REMOTE_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_IRSA_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_WORDPRESS_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_JUPYTER_HUB_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_KUBEFLOW_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_CUDA_VECTOR_ADD_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_LOCAL_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_VERSION_UPGRADE_ENABLE=true \



*----------------------------------------------------------------*-------------------*----------------------------------------------------------*---------------*
|                     ENVIRONMENTAL VARIABLE                     |     READ ONLY     |                           TYPE                           |    GO TYPE    |
*----------------------------------------------------------------*-------------------*----------------------------------------------------------*---------------*
| AWS_K8S_TESTER_EKS_NAME                                        | read-only "false" | *eksconfig.Config.Name                                   | string        |
| AWS_K8S_TESTER_EKS_PARTITION                                   | read-only "false" | *eksconfig.Config.Partition                              | string        |
| AWS_K8S_TESTER_EKS_REGION                                      | read-only "false" | *eksconfig.Config.Region                                 | string        |
| AWS_K8S_TESTER_EKS_CONFIG_PATH                                 | read-only "false" | *eksconfig.Config.ConfigPath                             | string        |
| AWS_K8S_TESTER_EKS_KUBECTL_COMMANDS_OUTPUT_PATH                | read-only "false" | *eksconfig.Config.KubectlCommandsOutputPath              | string        |
| AWS_K8S_TESTER_EKS_REMOTE_ACCESS_COMMANDS_OUTPUT_PATH          | read-only "false" | *eksconfig.Config.RemoteAccessCommandsOutputPath         | string        |
| AWS_K8S_TESTER_EKS_LOG_COLOR                                   | read-only "false" | *eksconfig.Config.LogColor                               | bool          |
| AWS_K8S_TESTER_EKS_LOG_COLOR_OVERRIDE                          | read-only "false" | *eksconfig.Config.LogColorOverride                       | bool          |
| AWS_K8S_TESTER_EKS_LOG_LEVEL                                   | read-only "false" | *eksconfig.Config.LogLevel                               | string        |
| AWS_K8S_TESTER_EKS_LOG_OUTPUTS                                 | read-only "false" | *eksconfig.Config.LogOutputs                             | []string      |
| AWS_K8S_TESTER_EKS_AWS_CLI_PATH                                | read-only "false" | *eksconfig.Config.AWSCLIPath                             | string        |
| AWS_K8S_TESTER_EKS_KUBECTL_PATH                                | read-only "false" | *eksconfig.Config.KubectlPath                            | string        |
| AWS_K8S_TESTER_EKS_KUBECTL_DOWNLOAD_URL                        | read-only "false" | *eksconfig.Config.KubectlDownloadURL                     | string        |
| AWS_K8S_TESTER_EKS_KUBECONFIG_PATH                             | read-only "false" | *eksconfig.Config.KubeConfigPath                         | string        |
| AWS_K8S_TESTER_EKS_AWS_IAM_AUTHENTICATOR_PATH                  | read-only "false" | *eksconfig.Config.AWSIAMAuthenticatorPath                | string        |
| AWS_K8S_TESTER_EKS_AWS_IAM_AUTHENTICATOR_DOWNLOAD_URL          | read-only "false" | *eksconfig.Config.AWSIAMAuthenticatorDownloadURL         | string        |
| AWS_K8S_TESTER_EKS_ON_FAILURE_DELETE                           | read-only "false" | *eksconfig.Config.OnFailureDelete                        | bool          |
| AWS_K8S_TESTER_EKS_ON_FAILURE_DELETE_WAIT_SECONDS              | read-only "false" | *eksconfig.Config.OnFailureDeleteWaitSeconds             | uint64        |
| AWS_K8S_TESTER_EKS_COMMAND_AFTER_CREATE_CLUSTER                | read-only "false" | *eksconfig.Config.CommandAfterCreateCluster              | string        |
| AWS_K8S_TESTER_EKS_COMMAND_AFTER_CREATE_CLUSTER_OUTPUT_PATH    | read-only "true"  | *eksconfig.Config.CommandAfterCreateClusterOutputPath    | string        |
| AWS_K8S_TESTER_EKS_COMMAND_AFTER_CREATE_CLUSTER_TIMEOUT        | read-only "false" | *eksconfig.Config.CommandAfterCreateClusterTimeout       | time.Duration |
| AWS_K8S_TESTER_EKS_COMMAND_AFTER_CREATE_CLUSTER_TIMEOUT_STRING | read-only "true"  | *eksconfig.Config.CommandAfterCreateClusterTimeoutString | string        |
| AWS_K8S_TESTER_EKS_COMMAND_AFTER_CREATE_ADD_ONS                | read-only "false" | *eksconfig.Config.CommandAfterCreateAddOns               | string        |
| AWS_K8S_TESTER_EKS_COMMAND_AFTER_CREATE_ADD_ONS_OUTPUT_PATH    | read-only "true"  | *eksconfig.Config.CommandAfterCreateAddOnsOutputPath     | string        |
| AWS_K8S_TESTER_EKS_COMMAND_AFTER_CREATE_ADD_ONS_TIMEOUT        | read-only "false" | *eksconfig.Config.CommandAfterCreateAddOnsTimeout        | time.Duration |
| AWS_K8S_TESTER_EKS_COMMAND_AFTER_CREATE_ADD_ONS_TIMEOUT_STRING | read-only "true"  | *eksconfig.Config.CommandAfterCreateAddOnsTimeoutString  | string        |
| AWS_K8S_TESTER_EKS_S3_BUCKET_CREATE                            | read-only "false" | *eksconfig.Config.S3BucketCreate                         | bool          |
| AWS_K8S_TESTER_EKS_S3_BUCKET_CREATE_KEEP                       | read-only "false" | *eksconfig.Config.S3BucketCreateKeep                     | bool          |
| AWS_K8S_TESTER_EKS_S3_BUCKET_NAME                              | read-only "false" | *eksconfig.Config.S3BucketName                           | string        |
| AWS_K8S_TESTER_EKS_S3_BUCKET_LIFECYCLE_EXPIRATION_DAYS         | read-only "false" | *eksconfig.Config.S3BucketLifecycleExpirationDays        | int64         |
| AWS_K8S_TESTER_EKS_CW_NAMESPACE                                | read-only "false" | *eksconfig.Config.CWNamespace                            | string        |
| AWS_K8S_TESTER_EKS_REMOTE_ACCESS_KEY_CREATE                    | read-only "false" | *eksconfig.Config.RemoteAccessKeyCreate                  | bool          |
| AWS_K8S_TESTER_EKS_REMOTE_ACCESS_KEY_NAME                      | read-only "false" | *eksconfig.Config.RemoteAccessKeyName                    | string        |
| AWS_K8S_TESTER_EKS_REMOTE_ACCESS_PRIVATE_KEY_PATH              | read-only "false" | *eksconfig.Config.RemoteAccessPrivateKeyPath             | string        |
| AWS_K8S_TESTER_EKS_CLIENTS                                     | read-only "false" | *eksconfig.Config.Clients                                | int           |
| AWS_K8S_TESTER_EKS_CLIENT_QPS                                  | read-only "false" | *eksconfig.Config.ClientQPS                              | float32       |
| AWS_K8S_TESTER_EKS_CLIENT_BURST                                | read-only "false" | *eksconfig.Config.ClientBurst                            | int           |
| AWS_K8S_TESTER_EKS_CLIENT_TIMEOUT                              | read-only "false" | *eksconfig.Config.ClientTimeout                          | time.Duration |
| AWS_K8S_TESTER_EKS_CLIENT_TIMEOUT_STRING                       | read-only "true"  | *eksconfig.Config.ClientTimeoutString                    | string        |
| AWS_K8S_TESTER_EKS_TOTAL_NODES                                 | read-only "true"  | *eksconfig.Config.TotalNodes                             | int64         |
| AWS_K8S_TESTER_EKS_TOTAL_HOLLOW_NODES                          | read-only "true"  | *eksconfig.Config.TotalHollowNodes                       | int64         |
*----------------------------------------------------------------*-------------------*----------------------------------------------------------*---------------*


*----------------------------------------------------------------*-------------------*----------------------------------------------------*-------------------*
|                     ENVIRONMENTAL VARIABLE                     |     READ ONLY     |                        TYPE                        |      GO TYPE      |
*----------------------------------------------------------------*-------------------*----------------------------------------------------*-------------------*
| AWS_K8S_TESTER_EKS_PARAMETERS_ROLE_NAME                        | read-only "false" | *eksconfig.Parameters.RoleName                     | string            |
| AWS_K8S_TESTER_EKS_PARAMETERS_ROLE_CREATE                      | read-only "false" | *eksconfig.Parameters.RoleCreate                   | bool              |
| AWS_K8S_TESTER_EKS_PARAMETERS_ROLE_ARN                         | read-only "false" | *eksconfig.Parameters.RoleARN                      | string            |
| AWS_K8S_TESTER_EKS_PARAMETERS_ROLE_SERVICE_PRINCIPALS          | read-only "false" | *eksconfig.Parameters.RoleServicePrincipals        | []string          |
| AWS_K8S_TESTER_EKS_PARAMETERS_ROLE_MANAGED_POLICY_ARNS         | read-only "false" | *eksconfig.Parameters.RoleManagedPolicyARNs        | []string          |
| AWS_K8S_TESTER_EKS_PARAMETERS_ROLE_CFN_STACK_ID                | read-only "true"  | *eksconfig.Parameters.RoleCFNStackID               | string            |
| AWS_K8S_TESTER_EKS_PARAMETERS_ROLE_CFN_STACK_YAML_FILE_PATH    | read-only "true"  | *eksconfig.Parameters.RoleCFNStackYAMLFilePath     | string            |
| AWS_K8S_TESTER_EKS_PARAMETERS_TAGS                             | read-only "false" | *eksconfig.Parameters.Tags                         | map[string]string |
| AWS_K8S_TESTER_EKS_PARAMETERS_REQUEST_HEADER_KEY               | read-only "false" | *eksconfig.Parameters.RequestHeaderKey             | string            |
| AWS_K8S_TESTER_EKS_PARAMETERS_REQUEST_HEADER_VALUE             | read-only "false" | *eksconfig.Parameters.RequestHeaderValue           | string            |
| AWS_K8S_TESTER_EKS_PARAMETERS_RESOLVER_URL                     | read-only "false" | *eksconfig.Parameters.ResolverURL                  | string            |
| AWS_K8S_TESTER_EKS_PARAMETERS_SIGNING_NAME                     | read-only "false" | *eksconfig.Parameters.SigningName                  | string            |
| AWS_K8S_TESTER_EKS_PARAMETERS_VPC_CREATE                       | read-only "false" | *eksconfig.Parameters.VPCCreate                    | bool              |
| AWS_K8S_TESTER_EKS_PARAMETERS_VPC_ID                           | read-only "false" | *eksconfig.Parameters.VPCID                        | string            |
| AWS_K8S_TESTER_EKS_PARAMETERS_VPC_CFN_STACK_ID                 | read-only "true"  | *eksconfig.Parameters.VPCCFNStackID                | string            |
| AWS_K8S_TESTER_EKS_PARAMETERS_VPC_CFN_STACK_YAML_FILE_PATH     | read-only "true"  | *eksconfig.Parameters.VPCCFNStackYAMLFilePath      | string            |
| AWS_K8S_TESTER_EKS_PARAMETERS_VPC_CIDR                         | read-only "false" | *eksconfig.Parameters.VPCCIDR                      | string            |
| AWS_K8S_TESTER_EKS_PARAMETERS_PUBLIC_SUBNET_CIDR_1             | read-only "false" | *eksconfig.Parameters.PublicSubnetCIDR1            | string            |
| AWS_K8S_TESTER_EKS_PARAMETERS_PUBLIC_SUBNET_CIDR_2             | read-only "false" | *eksconfig.Parameters.PublicSubnetCIDR2            | string            |
| AWS_K8S_TESTER_EKS_PARAMETERS_PUBLIC_SUBNET_CIDR_3             | read-only "false" | *eksconfig.Parameters.PublicSubnetCIDR3            | string            |
| AWS_K8S_TESTER_EKS_PARAMETERS_PRIVATE_SUBNET_CIDR_1            | read-only "false" | *eksconfig.Parameters.PrivateSubnetCIDR1           | string            |
| AWS_K8S_TESTER_EKS_PARAMETERS_PRIVATE_SUBNET_CIDR_2            | read-only "false" | *eksconfig.Parameters.PrivateSubnetCIDR2           | string            |
| AWS_K8S_TESTER_EKS_PARAMETERS_PUBLIC_SUBNET_IDS                | read-only "true"  | *eksconfig.Parameters.PublicSubnetIDs              | []string          |
| AWS_K8S_TESTER_EKS_PARAMETERS_PRIVATE_SUBNET_IDS               | read-only "true"  | *eksconfig.Parameters.PrivateSubnetIDs             | []string          |
| AWS_K8S_TESTER_EKS_PARAMETERS_DHCP_OPTIONS_DOMAIN_NAME         | read-only "false" | *eksconfig.Parameters.DHCPOptionsDomainName        | string            |
| AWS_K8S_TESTER_EKS_PARAMETERS_DHCP_OPTIONS_DOMAIN_NAME_SERVERS | read-only "false" | *eksconfig.Parameters.DHCPOptionsDomainNameServers | []string          |
| AWS_K8S_TESTER_EKS_PARAMETERS_VERSION                          | read-only "false" | *eksconfig.Parameters.Version                      | string            |
| AWS_K8S_TESTER_EKS_PARAMETERS_VERSION_VALUE                    | read-only "true"  | *eksconfig.Parameters.VersionValue                 | float64           |
| AWS_K8S_TESTER_EKS_PARAMETERS_ENCRYPTION_CMK_CREATE            | read-only "false" | *eksconfig.Parameters.EncryptionCMKCreate          | bool              |
| AWS_K8S_TESTER_EKS_PARAMETERS_ENCRYPTION_CMK_ARN               | read-only "false" | *eksconfig.Parameters.EncryptionCMKARN             | string            |
*----------------------------------------------------------------*-------------------*----------------------------------------------------*-------------------*


*------------------------------------------------------------------------------------------*-------------------*-----------------------------------------------------------------------*--------------------------*
|                                  ENVIRONMENTAL VARIABLE                                  |     READ ONLY     |                                 TYPE                                  |         GO TYPE          |
*------------------------------------------------------------------------------------------*-------------------*-----------------------------------------------------------------------*--------------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ENABLE                                             | read-only "false" | *eksconfig.AddOnNodeGroups.Enable                                     | bool                     |
| AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_CREATED                                            | read-only "true"  | *eksconfig.AddOnNodeGroups.Created                                    | bool                     |
| AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_TIME_FRAME_CREATE                                  | read-only "true"  | *eksconfig.AddOnNodeGroups.TimeFrameCreate                            | timeutil.TimeFrame       |
| AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_TIME_FRAME_DELETE                                  | read-only "true"  | *eksconfig.AddOnNodeGroups.TimeFrameDelete                            | timeutil.TimeFrame       |
| AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_FETCH_LOGS                                         | read-only "false" | *eksconfig.AddOnNodeGroups.FetchLogs                                  | bool                     |
| AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ROLE_NAME                                          | read-only "false" | *eksconfig.AddOnNodeGroups.RoleName                                   | string                   |
| AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ROLE_CREATE                                        | read-only "false" | *eksconfig.AddOnNodeGroups.RoleCreate                                 | bool                     |
| AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ROLE_ARN                                           | read-only "false" | *eksconfig.AddOnNodeGroups.RoleARN                                    | string                   |
| AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ROLE_SERVICE_PRINCIPALS                            | read-only "false" | *eksconfig.AddOnNodeGroups.RoleServicePrincipals                      | []string                 |
| AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ROLE_MANAGED_POLICY_ARNS                           | read-only "false" | *eksconfig.AddOnNodeGroups.RoleManagedPolicyARNs                      | []string                 |
| AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ROLE_CFN_STACK_ID                                  | read-only "true"  | *eksconfig.AddOnNodeGroups.RoleCFNStackID                             | string                   |
| AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ROLE_CFN_STACK_YAML_FILE_PATH                      | read-only "true"  | *eksconfig.AddOnNodeGroups.RoleCFNStackYAMLFilePath                   | string                   |
| AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_NODE_GROUP_SECURITY_GROUP_ID                       | read-only "true"  | *eksconfig.AddOnNodeGroups.NodeGroupSecurityGroupID                   | string                   |
| AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_NODE_GROUP_SECURITY_GROUP_CFN_STACK_ID             | read-only "true"  | *eksconfig.AddOnNodeGroups.NodeGroupSecurityGroupCFNStackID           | string                   |
| AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_NODE_GROUP_SECURITY_GROUP_CFN_STACK_YAML_FILE_PATH | read-only "true"  | *eksconfig.AddOnNodeGroups.NodeGroupSecurityGroupCFNStackYAMLFilePath | string                   |
| AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_LOGS_DIR                                           | read-only "false" | *eksconfig.AddOnNodeGroups.LogsDir                                    | string                   |
| AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_LOGS_TAR_GZ_PATH                                   | read-only "false" | *eksconfig.AddOnNodeGroups.LogsTarGzPath                              | string                   |
| AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ASGS                                               | read-only "false" | *eksconfig.AddOnNodeGroups.ASGs                                       | map[string]eksconfig.ASG |
*------------------------------------------------------------------------------------------*-------------------*-----------------------------------------------------------------------*--------------------------*


*-----------------------------------------------------------------------------*-------------------*------------------------------------------------------------*--------------------------*
|                           ENVIRONMENTAL VARIABLE                            |     READ ONLY     |                            TYPE                            |         GO TYPE          |
*-----------------------------------------------------------------------------*-------------------*------------------------------------------------------------*--------------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE                        | read-only "false" | *eksconfig.AddOnManagedNodeGroups.Enable                   | bool                     |
| AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_CREATED                       | read-only "true"  | *eksconfig.AddOnManagedNodeGroups.Created                  | bool                     |
| AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_TIME_FRAME_CREATE             | read-only "true"  | *eksconfig.AddOnManagedNodeGroups.TimeFrameCreate          | timeutil.TimeFrame       |
| AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_TIME_FRAME_DELETE             | read-only "true"  | *eksconfig.AddOnManagedNodeGroups.TimeFrameDelete          | timeutil.TimeFrame       |
| AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_FETCH_LOGS                    | read-only "false" | *eksconfig.AddOnManagedNodeGroups.FetchLogs                | bool                     |
| AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ROLE_NAME                     | read-only "false" | *eksconfig.AddOnManagedNodeGroups.RoleName                 | string                   |
| AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ROLE_CREATE                   | read-only "false" | *eksconfig.AddOnManagedNodeGroups.RoleCreate               | bool                     |
| AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ROLE_ARN                      | read-only "false" | *eksconfig.AddOnManagedNodeGroups.RoleARN                  | string                   |
| AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ROLE_SERVICE_PRINCIPALS       | read-only "false" | *eksconfig.AddOnManagedNodeGroups.RoleServicePrincipals    | []string                 |
| AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ROLE_MANAGED_POLICY_ARNS      | read-only "false" | *eksconfig.AddOnManagedNodeGroups.RoleManagedPolicyARNs    | []string                 |
| AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ROLE_CFN_STACK_ID             | read-only "true"  | *eksconfig.AddOnManagedNodeGroups.RoleCFNStackID           | string                   |
| AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ROLE_CFN_STACK_YAML_FILE_PATH | read-only "true"  | *eksconfig.AddOnManagedNodeGroups.RoleCFNStackYAMLFilePath | string                   |
| AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_REQUEST_HEADER_KEY            | read-only "false" | *eksconfig.AddOnManagedNodeGroups.RequestHeaderKey         | string                   |
| AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_REQUEST_HEADER_VALUE          | read-only "false" | *eksconfig.AddOnManagedNodeGroups.RequestHeaderValue       | string                   |
| AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_RESOLVER_URL                  | read-only "false" | *eksconfig.AddOnManagedNodeGroups.ResolverURL              | string                   |
| AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_SIGNING_NAME                  | read-only "false" | *eksconfig.AddOnManagedNodeGroups.SigningName              | string                   |
| AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_LOGS_DIR                      | read-only "false" | *eksconfig.AddOnManagedNodeGroups.LogsDir                  | string                   |
| AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_LOGS_TAR_GZ_PATH              | read-only "false" | *eksconfig.AddOnManagedNodeGroups.LogsTarGzPath            | string                   |
| AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_MNGS                          | read-only "false" | *eksconfig.AddOnManagedNodeGroups.MNGs                     | map[string]eksconfig.MNG |
*-----------------------------------------------------------------------------*-------------------*------------------------------------------------------------*--------------------------*


*------------------------------------------------------------*-------------------*-----------------------------------------------*--------------------*
|                   ENVIRONMENTAL VARIABLE                   |     READ ONLY     |                     TYPE                      |      GO TYPE       |
*------------------------------------------------------------*-------------------*-----------------------------------------------*--------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_METRICS_SERVER_ENABLE            | read-only "false" | *eksconfig.AddOnMetricsServer.Enable          | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_METRICS_SERVER_CREATED           | read-only "true"  | *eksconfig.AddOnMetricsServer.Created         | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_METRICS_SERVER_TIME_FRAME_CREATE | read-only "true"  | *eksconfig.AddOnMetricsServer.TimeFrameCreate | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_METRICS_SERVER_TIME_FRAME_DELETE | read-only "true"  | *eksconfig.AddOnMetricsServer.TimeFrameDelete | timeutil.TimeFrame |
*------------------------------------------------------------*-------------------*-----------------------------------------------*--------------------*


*---------------------------------------------------------------------------*-------------------*-------------------------------------------------------------*--------------------*
|                          ENVIRONMENTAL VARIABLE                           |     READ ONLY     |                            TYPE                             |      GO TYPE       |
*---------------------------------------------------------------------------*-------------------*-------------------------------------------------------------*--------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_ENABLE                              | read-only "false" | *eksconfig.AddOnConformance.Enable                          | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_CREATED                             | read-only "true"  | *eksconfig.AddOnConformance.Created                         | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_TIME_FRAME_CREATE                   | read-only "true"  | *eksconfig.AddOnConformance.TimeFrameCreate                 | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_TIME_FRAME_DELETE                   | read-only "true"  | *eksconfig.AddOnConformance.TimeFrameDelete                 | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_NAMESPACE                           | read-only "false" | *eksconfig.AddOnConformance.Namespace                       | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_PATH                       | read-only "false" | *eksconfig.AddOnConformance.SonobuoyPath                    | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_DOWNLOAD_URL               | read-only "false" | *eksconfig.AddOnConformance.SonobuoyDownloadURL             | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_DELETE_TIMEOUT             | read-only "false" | *eksconfig.AddOnConformance.SonobuoyDeleteTimeout           | time.Duration      |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_DELETE_TIMEOUT_STRING      | read-only "true"  | *eksconfig.AddOnConformance.SonobuoyDeleteTimeoutString     | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_RUN_TIMEOUT                | read-only "false" | *eksconfig.AddOnConformance.SonobuoyRunTimeout              | time.Duration      |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_RUN_TIMEOUT_STRING         | read-only "true"  | *eksconfig.AddOnConformance.SonobuoyRunTimeoutString        | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_RUN_MODE                   | read-only "false" | *eksconfig.AddOnConformance.SonobuoyRunMode                 | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_RUN_KUBE_CONFORMANCE_IMAGE | read-only "false" | *eksconfig.AddOnConformance.SonobuoyRunKubeConformanceImage | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_RESULT_TAR_GZ_PATH         | read-only "true"  | *eksconfig.AddOnConformance.SonobuoyResultTarGzPath         | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_RESULT_DIR                 | read-only "true"  | *eksconfig.AddOnConformance.SonobuoyResultDir               | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_RESULT_E2E_LOG_PATH        | read-only "true"  | *eksconfig.AddOnConformance.SonobuoyResultE2eLogPath        | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_RESULT_JUNIT_XML_PATH      | read-only "true"  | *eksconfig.AddOnConformance.SonobuoyResultJunitXMLPath      | string             |
*---------------------------------------------------------------------------*-------------------*-------------------------------------------------------------*--------------------*


*--------------------------------------------------------------------*-------------------*----------------------------------------------------*--------------------*
|                       ENVIRONMENTAL VARIABLE                       |     READ ONLY     |                        TYPE                        |      GO TYPE       |
*--------------------------------------------------------------------*-------------------*----------------------------------------------------*--------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_APP_MESH_ENABLE                          | read-only "false" | *eksconfig.AddOnAppMesh.Enable                     | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_APP_MESH_CREATED                         | read-only "true"  | *eksconfig.AddOnAppMesh.Created                    | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_APP_MESH_TIME_FRAME_CREATE               | read-only "true"  | *eksconfig.AddOnAppMesh.TimeFrameCreate            | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_APP_MESH_TIME_FRAME_DELETE               | read-only "true"  | *eksconfig.AddOnAppMesh.TimeFrameDelete            | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_APP_MESH_NAMESPACE                       | read-only "false" | *eksconfig.AddOnAppMesh.Namespace                  | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_APP_MESH_CONTROLLER_IMAGE                | read-only "false" | *eksconfig.AddOnAppMesh.ControllerImage            | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_APP_MESH_INJECTOR_IMAGE                  | read-only "false" | *eksconfig.AddOnAppMesh.InjectorImage              | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_APP_MESH_POLICY_CFN_STACK_ID             | read-only "true"  | *eksconfig.AddOnAppMesh.PolicyCFNStackID           | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_APP_MESH_POLICY_CFN_STACK_YAML_FILE_PATH | read-only "true"  | *eksconfig.AddOnAppMesh.PolicyCFNStackYAMLFilePath | string             |
*--------------------------------------------------------------------*-------------------*----------------------------------------------------*--------------------*


*-----------------------------------------------------*-------------------*----------------------------------------*--------------------*
|               ENVIRONMENTAL VARIABLE                |     READ ONLY     |                  TYPE                  |      GO TYPE       |
*-----------------------------------------------------*-------------------*----------------------------------------*--------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_CSI_EBS_ENABLE            | read-only "false" | *eksconfig.AddOnCSIEBS.Enable          | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_CSI_EBS_CREATED           | read-only "true"  | *eksconfig.AddOnCSIEBS.Created         | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_CSI_EBS_TIME_FRAME_CREATE | read-only "true"  | *eksconfig.AddOnCSIEBS.TimeFrameCreate | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_CSI_EBS_TIME_FRAME_DELETE | read-only "true"  | *eksconfig.AddOnCSIEBS.TimeFrameDelete | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_CSI_EBS_CHART_REPO_URL    | read-only "false" | *eksconfig.AddOnCSIEBS.ChartRepoURL    | string             |
*-----------------------------------------------------*-------------------*----------------------------------------*--------------------*


*---------------------------------------------------------------------*-------------------*---------------------------------------------------------*--------------------*
|                       ENVIRONMENTAL VARIABLE                        |     READ ONLY     |                          TYPE                           |      GO TYPE       |
*---------------------------------------------------------------------*-------------------*---------------------------------------------------------*--------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_KUBERNETES_DASHBOARD_ENABLE               | read-only "false" | *eksconfig.AddOnKubernetesDashboard.Enable              | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_KUBERNETES_DASHBOARD_CREATED              | read-only "true"  | *eksconfig.AddOnKubernetesDashboard.Created             | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_KUBERNETES_DASHBOARD_TIME_FRAME_CREATE    | read-only "true"  | *eksconfig.AddOnKubernetesDashboard.TimeFrameCreate     | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_KUBERNETES_DASHBOARD_TIME_FRAME_DELETE    | read-only "true"  | *eksconfig.AddOnKubernetesDashboard.TimeFrameDelete     | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_KUBERNETES_DASHBOARD_AUTHENTICATION_TOKEN | read-only "true"  | *eksconfig.AddOnKubernetesDashboard.AuthenticationToken | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_KUBERNETES_DASHBOARD_URL                  | read-only "true"  | *eksconfig.AddOnKubernetesDashboard.URL                 | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_KUBERNETES_DASHBOARD_KUBECTL_PROXY_PID    | read-only "true"  | *eksconfig.AddOnKubernetesDashboard.KubectlProxyPID     | int                |
*---------------------------------------------------------------------*-------------------*---------------------------------------------------------*--------------------*


*----------------------------------------------------------------------*-------------------*--------------------------------------------------------*--------------------*
|                        ENVIRONMENTAL VARIABLE                        |     READ ONLY     |                          TYPE                          |      GO TYPE       |
*----------------------------------------------------------------------*-------------------*--------------------------------------------------------*--------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_PROMETHEUS_GRAFANA_ENABLE                  | read-only "false" | *eksconfig.AddOnPrometheusGrafana.Enable               | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_PROMETHEUS_GRAFANA_CREATED                 | read-only "true"  | *eksconfig.AddOnPrometheusGrafana.Created              | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_PROMETHEUS_GRAFANA_TIME_FRAME_CREATE       | read-only "true"  | *eksconfig.AddOnPrometheusGrafana.TimeFrameCreate      | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_PROMETHEUS_GRAFANA_TIME_FRAME_DELETE       | read-only "true"  | *eksconfig.AddOnPrometheusGrafana.TimeFrameDelete      | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_PROMETHEUS_GRAFANA_GRAFANA_ADMIN_USER_NAME | read-only "false" | *eksconfig.AddOnPrometheusGrafana.GrafanaAdminUserName | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_PROMETHEUS_GRAFANA_GRAFANA_ADMIN_PASSWORD  | read-only "false" | *eksconfig.AddOnPrometheusGrafana.GrafanaAdminPassword | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_PROMETHEUS_GRAFANA_GRAFANA_NLB_ARN         | read-only "true"  | *eksconfig.AddOnPrometheusGrafana.GrafanaNLBARN        | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_PROMETHEUS_GRAFANA_GRAFANA_NLB_NAME        | read-only "true"  | *eksconfig.AddOnPrometheusGrafana.GrafanaNLBName       | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_PROMETHEUS_GRAFANA_GRAFANA_URL             | read-only "true"  | *eksconfig.AddOnPrometheusGrafana.GrafanaURL           | string             |
*----------------------------------------------------------------------*-------------------*--------------------------------------------------------*--------------------*


*--------------------------------------------------------------------*-------------------*------------------------------------------------------*--------------------*
|                       ENVIRONMENTAL VARIABLE                       |     READ ONLY     |                         TYPE                         |      GO TYPE       |
*--------------------------------------------------------------------*-------------------*------------------------------------------------------*--------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_ENABLE                   | read-only "false" | *eksconfig.AddOnNLBHelloWorld.Enable                 | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_CREATED                  | read-only "true"  | *eksconfig.AddOnNLBHelloWorld.Created                | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_TIME_FRAME_CREATE        | read-only "true"  | *eksconfig.AddOnNLBHelloWorld.TimeFrameCreate        | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_TIME_FRAME_DELETE        | read-only "true"  | *eksconfig.AddOnNLBHelloWorld.TimeFrameDelete        | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_NAMESPACE                | read-only "false" | *eksconfig.AddOnNLBHelloWorld.Namespace              | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_DEPLOYMENT_REPLICAS      | read-only "false" | *eksconfig.AddOnNLBHelloWorld.DeploymentReplicas     | int32              |
| AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_DEPLOYMENT_NODE_SELECTOR | read-only "false" | *eksconfig.AddOnNLBHelloWorld.DeploymentNodeSelector | map[string]string  |
| AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_NLB_ARN                  | read-only "true"  | *eksconfig.AddOnNLBHelloWorld.NLBARN                 | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_NLB_NAME                 | read-only "true"  | *eksconfig.AddOnNLBHelloWorld.NLBName                | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_URL                      | read-only "true"  | *eksconfig.AddOnNLBHelloWorld.URL                    | string             |
*--------------------------------------------------------------------*-------------------*------------------------------------------------------*--------------------*


*------------------------------------------------------------------*-------------------*-----------------------------------------------------*--------------------*
|                      ENVIRONMENTAL VARIABLE                      |     READ ONLY     |                        TYPE                         |      GO TYPE       |
*------------------------------------------------------------------*-------------------*-----------------------------------------------------*--------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_NLB_GUESTBOOK_ENABLE                   | read-only "false" | *eksconfig.AddOnNLBGuestbook.Enable                 | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_NLB_GUESTBOOK_CREATED                  | read-only "true"  | *eksconfig.AddOnNLBGuestbook.Created                | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_NLB_GUESTBOOK_TIME_FRAME_CREATE        | read-only "true"  | *eksconfig.AddOnNLBGuestbook.TimeFrameCreate        | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_NLB_GUESTBOOK_TIME_FRAME_DELETE        | read-only "true"  | *eksconfig.AddOnNLBGuestbook.TimeFrameDelete        | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_NLB_GUESTBOOK_NAMESPACE                | read-only "false" | *eksconfig.AddOnNLBGuestbook.Namespace              | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_NLB_GUESTBOOK_DEPLOYMENT_REPLICAS      | read-only "false" | *eksconfig.AddOnNLBGuestbook.DeploymentReplicas     | int32              |
| AWS_K8S_TESTER_EKS_ADD_ON_NLB_GUESTBOOK_DEPLOYMENT_NODE_SELECTOR | read-only "false" | *eksconfig.AddOnNLBGuestbook.DeploymentNodeSelector | map[string]string  |
| AWS_K8S_TESTER_EKS_ADD_ON_NLB_GUESTBOOK_NLB_ARN                  | read-only "true"  | *eksconfig.AddOnNLBGuestbook.NLBARN                 | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_NLB_GUESTBOOK_NLB_NAME                 | read-only "true"  | *eksconfig.AddOnNLBGuestbook.NLBName                | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_NLB_GUESTBOOK_URL                      | read-only "true"  | *eksconfig.AddOnNLBGuestbook.URL                    | string             |
*------------------------------------------------------------------*-------------------*-----------------------------------------------------*--------------------*


*------------------------------------------------------------------*-------------------*----------------------------------------------------*--------------------*
|                      ENVIRONMENTAL VARIABLE                      |     READ ONLY     |                        TYPE                        |      GO TYPE       |
*------------------------------------------------------------------*-------------------*----------------------------------------------------*--------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_ENABLE                        | read-only "false" | *eksconfig.AddOnALB2048.Enable                     | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_CREATED                       | read-only "true"  | *eksconfig.AddOnALB2048.Created                    | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_TIME_FRAME_CREATE             | read-only "true"  | *eksconfig.AddOnALB2048.TimeFrameCreate            | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_TIME_FRAME_DELETE             | read-only "true"  | *eksconfig.AddOnALB2048.TimeFrameDelete            | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_NAMESPACE                     | read-only "false" | *eksconfig.AddOnALB2048.Namespace                  | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_DEPLOYMENT_REPLICAS_ALB       | read-only "false" | *eksconfig.AddOnALB2048.DeploymentReplicasALB      | int32              |
| AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_DEPLOYMENT_REPLICAS_2048      | read-only "false" | *eksconfig.AddOnALB2048.DeploymentReplicas2048     | int32              |
| AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_DEPLOYMENT_NODE_SELECTOR_2048 | read-only "false" | *eksconfig.AddOnALB2048.DeploymentNodeSelector2048 | map[string]string  |
| AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_ALB_ARN                       | read-only "true"  | *eksconfig.AddOnALB2048.ALBARN                     | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_ALB_NAME                      | read-only "true"  | *eksconfig.AddOnALB2048.ALBName                    | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_URL                           | read-only "true"  | *eksconfig.AddOnALB2048.URL                        | string             |
*------------------------------------------------------------------*-------------------*----------------------------------------------------*--------------------*


*-----------------------------------------------------*-------------------*----------------------------------------*--------------------*
|               ENVIRONMENTAL VARIABLE                |     READ ONLY     |                  TYPE                  |      GO TYPE       |
*-----------------------------------------------------*-------------------*----------------------------------------*--------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_JOBS_PI_ENABLE            | read-only "false" | *eksconfig.AddOnJobsPi.Enable          | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_JOBS_PI_CREATED           | read-only "true"  | *eksconfig.AddOnJobsPi.Created         | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_JOBS_PI_TIME_FRAME_CREATE | read-only "true"  | *eksconfig.AddOnJobsPi.TimeFrameCreate | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_JOBS_PI_TIME_FRAME_DELETE | read-only "true"  | *eksconfig.AddOnJobsPi.TimeFrameDelete | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_JOBS_PI_NAMESPACE         | read-only "false" | *eksconfig.AddOnJobsPi.Namespace       | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_JOBS_PI_COMPLETES         | read-only "false" | *eksconfig.AddOnJobsPi.Completes       | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_JOBS_PI_PARALLELS         | read-only "false" | *eksconfig.AddOnJobsPi.Parallels       | int                |
*-----------------------------------------------------*-------------------*----------------------------------------*--------------------*


*-------------------------------------------------------*-------------------*------------------------------------------*--------------------*
|                ENVIRONMENTAL VARIABLE                 |     READ ONLY     |                   TYPE                   |      GO TYPE       |
*-------------------------------------------------------*-------------------*------------------------------------------*--------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_JOBS_ECHO_ENABLE            | read-only "false" | *eksconfig.AddOnJobsEcho.Enable          | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_JOBS_ECHO_CREATED           | read-only "true"  | *eksconfig.AddOnJobsEcho.Created         | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_JOBS_ECHO_TIME_FRAME_CREATE | read-only "true"  | *eksconfig.AddOnJobsEcho.TimeFrameCreate | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_JOBS_ECHO_TIME_FRAME_DELETE | read-only "true"  | *eksconfig.AddOnJobsEcho.TimeFrameDelete | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_JOBS_ECHO_NAMESPACE         | read-only "false" | *eksconfig.AddOnJobsEcho.Namespace       | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_JOBS_ECHO_COMPLETES         | read-only "false" | *eksconfig.AddOnJobsEcho.Completes       | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_JOBS_ECHO_PARALLELS         | read-only "false" | *eksconfig.AddOnJobsEcho.Parallels       | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_JOBS_ECHO_ECHO_SIZE         | read-only "false" | *eksconfig.AddOnJobsEcho.EchoSize        | int                |
*-------------------------------------------------------*-------------------*------------------------------------------*--------------------*


*-------------------------------------------------------------------*-------------------*-----------------------------------------------------*--------------------*
|                      ENVIRONMENTAL VARIABLE                       |     READ ONLY     |                        TYPE                         |      GO TYPE       |
*-------------------------------------------------------------------*-------------------*-----------------------------------------------------*--------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_ENABLE                        | read-only "false" | *eksconfig.AddOnCronJobs.Enable                     | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_CREATED                       | read-only "true"  | *eksconfig.AddOnCronJobs.Created                    | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_TIME_FRAME_CREATE             | read-only "true"  | *eksconfig.AddOnCronJobs.TimeFrameCreate            | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_TIME_FRAME_DELETE             | read-only "true"  | *eksconfig.AddOnCronJobs.TimeFrameDelete            | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_NAMESPACE                     | read-only "false" | *eksconfig.AddOnCronJobs.Namespace                  | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_SCHEDULE                      | read-only "false" | *eksconfig.AddOnCronJobs.Schedule                   | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_COMPLETES                     | read-only "false" | *eksconfig.AddOnCronJobs.Completes                  | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_PARALLELS                     | read-only "false" | *eksconfig.AddOnCronJobs.Parallels                  | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_SUCCESSFUL_JOBS_HISTORY_LIMIT | read-only "false" | *eksconfig.AddOnCronJobs.SuccessfulJobsHistoryLimit | int32              |
| AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_FAILED_JOBS_HISTORY_LIMIT     | read-only "false" | *eksconfig.AddOnCronJobs.FailedJobsHistoryLimit     | int32              |
| AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_ECHO_SIZE                     | read-only "false" | *eksconfig.AddOnCronJobs.EchoSize                   | int                |
*-------------------------------------------------------------------*-------------------*-----------------------------------------------------*--------------------*


*---------------------------------------------------------------------------------*-------------------*-----------------------------------------------------------------*--------------------------------*
|                             ENVIRONMENTAL VARIABLE                              |     READ ONLY     |                              TYPE                               |            GO TYPE             |
*---------------------------------------------------------------------------------*-------------------*-----------------------------------------------------------------*--------------------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_LOCAL_ENABLE                                     | read-only "false" | *eksconfig.AddOnCSRsLocal.Enable                                | bool                           |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_LOCAL_CREATED                                    | read-only "true"  | *eksconfig.AddOnCSRsLocal.Created                               | bool                           |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_LOCAL_TIME_FRAME_CREATE                          | read-only "true"  | *eksconfig.AddOnCSRsLocal.TimeFrameCreate                       | timeutil.TimeFrame             |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_LOCAL_TIME_FRAME_DELETE                          | read-only "true"  | *eksconfig.AddOnCSRsLocal.TimeFrameDelete                       | timeutil.TimeFrame             |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_LOCAL_OBJECTS                                    | read-only "false" | *eksconfig.AddOnCSRsLocal.Objects                               | int                            |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_LOCAL_INITIAL_REQUEST_CONDITION_TYPE             | read-only "false" | *eksconfig.AddOnCSRsLocal.InitialRequestConditionType           | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_LOCAL_CREATED_NAMES                              | read-only "true"  | *eksconfig.AddOnCSRsLocal.CreatedNames                          | []string                       |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_LOCAL_REQUESTS_WRITES_JSON_PATH                  | read-only "true"  | *eksconfig.AddOnCSRsLocal.RequestsWritesJSONPath                | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_LOCAL_REQUESTS_WRITES_SUMMARY                    | read-only "true"  | *eksconfig.AddOnCSRsLocal.RequestsWritesSummary                 | metrics.RequestsSummary        |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_LOCAL_REQUESTS_WRITES_SUMMARY_JSON_PATH          | read-only "true"  | *eksconfig.AddOnCSRsLocal.RequestsWritesSummaryJSONPath         | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_LOCAL_REQUESTS_WRITES_SUMMARY_TABLE_PATH         | read-only "true"  | *eksconfig.AddOnCSRsLocal.RequestsWritesSummaryTablePath        | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_LOCAL_REQUESTS_WRITES_SUMMARY_S3_DIR             | read-only "false" | *eksconfig.AddOnCSRsLocal.RequestsWritesSummaryS3Dir            | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_LOCAL_REQUESTS_WRITES_SUMMARY_COMPARE            | read-only "true"  | *eksconfig.AddOnCSRsLocal.RequestsWritesSummaryCompare          | metrics.RequestsSummaryCompare |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_LOCAL_REQUESTS_WRITES_SUMMARY_COMPARE_JSON_PATH  | read-only "true"  | *eksconfig.AddOnCSRsLocal.RequestsWritesSummaryCompareJSONPath  | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_LOCAL_REQUESTS_WRITES_SUMMARY_COMPARE_TABLE_PATH | read-only "true"  | *eksconfig.AddOnCSRsLocal.RequestsWritesSummaryCompareTablePath | string                         |
*---------------------------------------------------------------------------------*-------------------*-----------------------------------------------------------------*--------------------------------*


*----------------------------------------------------------------------------------*-------------------*------------------------------------------------------------------*--------------------------------*
|                              ENVIRONMENTAL VARIABLE                              |     READ ONLY     |                               TYPE                               |            GO TYPE             |
*----------------------------------------------------------------------------------*-------------------*------------------------------------------------------------------*--------------------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_ENABLE                                     | read-only "false" | *eksconfig.AddOnCSRsRemote.Enable                                | bool                           |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_CREATED                                    | read-only "true"  | *eksconfig.AddOnCSRsRemote.Created                               | bool                           |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_TIME_FRAME_CREATE                          | read-only "true"  | *eksconfig.AddOnCSRsRemote.TimeFrameCreate                       | timeutil.TimeFrame             |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_TIME_FRAME_DELETE                          | read-only "true"  | *eksconfig.AddOnCSRsRemote.TimeFrameDelete                       | timeutil.TimeFrame             |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_NAMESPACE                                  | read-only "false" | *eksconfig.AddOnCSRsRemote.Namespace                             | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_REPOSITORY_ACCOUNT_ID                      | read-only "false" | *eksconfig.AddOnCSRsRemote.RepositoryAccountID                   | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_REPOSITORY_NAME                            | read-only "false" | *eksconfig.AddOnCSRsRemote.RepositoryName                        | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_REPOSITORY_IMAGE_TAG                       | read-only "false" | *eksconfig.AddOnCSRsRemote.RepositoryImageTag                    | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_DEPLOYMENT_REPLICAS                        | read-only "false" | *eksconfig.AddOnCSRsRemote.DeploymentReplicas                    | int32                          |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_OBJECTS                                    | read-only "false" | *eksconfig.AddOnCSRsRemote.Objects                               | int                            |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_INITIAL_REQUEST_CONDITION_TYPE             | read-only "false" | *eksconfig.AddOnCSRsRemote.InitialRequestConditionType           | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_REQUESTS_WRITES_JSON_PATH                  | read-only "true"  | *eksconfig.AddOnCSRsRemote.RequestsWritesJSONPath                | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_REQUESTS_WRITES_SUMMARY                    | read-only "true"  | *eksconfig.AddOnCSRsRemote.RequestsWritesSummary                 | metrics.RequestsSummary        |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_REQUESTS_WRITES_SUMMARY_JSON_PATH          | read-only "true"  | *eksconfig.AddOnCSRsRemote.RequestsWritesSummaryJSONPath         | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_REQUESTS_WRITES_SUMMARY_TABLE_PATH         | read-only "true"  | *eksconfig.AddOnCSRsRemote.RequestsWritesSummaryTablePath        | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_REQUESTS_WRITES_SUMMARY_S3_DIR             | read-only "false" | *eksconfig.AddOnCSRsRemote.RequestsWritesSummaryS3Dir            | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_REQUESTS_WRITES_SUMMARY_COMPARE            | read-only "true"  | *eksconfig.AddOnCSRsRemote.RequestsWritesSummaryCompare          | metrics.RequestsSummaryCompare |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_REQUESTS_WRITES_SUMMARY_COMPARE_JSON_PATH  | read-only "true"  | *eksconfig.AddOnCSRsRemote.RequestsWritesSummaryCompareJSONPath  | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_REQUESTS_WRITES_SUMMARY_COMPARE_TABLE_PATH | read-only "true"  | *eksconfig.AddOnCSRsRemote.RequestsWritesSummaryCompareTablePath | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_REQUESTS_WRITES_SUMMARY_OUTPUT_NAME_PREFIX | read-only "false" | *eksconfig.AddOnCSRsRemote.RequestsWritesSummaryOutputNamePrefix | string                         |
*----------------------------------------------------------------------------------*-------------------*------------------------------------------------------------------*--------------------------------*


*---------------------------------------------------------------------------------------*-------------------*-----------------------------------------------------------------------*--------------------------------*
|                                ENVIRONMENTAL VARIABLE                                 |     READ ONLY     |                                 TYPE                                  |            GO TYPE             |
*---------------------------------------------------------------------------------------*-------------------*-----------------------------------------------------------------------*--------------------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_LOCAL_ENABLE                                     | read-only "false" | *eksconfig.AddOnConfigmapsLocal.Enable                                | bool                           |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_LOCAL_CREATED                                    | read-only "true"  | *eksconfig.AddOnConfigmapsLocal.Created                               | bool                           |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_LOCAL_TIME_FRAME_CREATE                          | read-only "true"  | *eksconfig.AddOnConfigmapsLocal.TimeFrameCreate                       | timeutil.TimeFrame             |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_LOCAL_TIME_FRAME_DELETE                          | read-only "true"  | *eksconfig.AddOnConfigmapsLocal.TimeFrameDelete                       | timeutil.TimeFrame             |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_LOCAL_NAMESPACE                                  | read-only "false" | *eksconfig.AddOnConfigmapsLocal.Namespace                             | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_LOCAL_OBJECTS                                    | read-only "false" | *eksconfig.AddOnConfigmapsLocal.Objects                               | int                            |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_LOCAL_OBJECT_SIZE                                | read-only "false" | *eksconfig.AddOnConfigmapsLocal.ObjectSize                            | int                            |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_LOCAL_CREATED_NAMES                              | read-only "true"  | *eksconfig.AddOnConfigmapsLocal.CreatedNames                          | []string                       |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_LOCAL_REQUESTS_WRITES_JSON_PATH                  | read-only "true"  | *eksconfig.AddOnConfigmapsLocal.RequestsWritesJSONPath                | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_LOCAL_REQUESTS_WRITES_SUMMARY                    | read-only "true"  | *eksconfig.AddOnConfigmapsLocal.RequestsWritesSummary                 | metrics.RequestsSummary        |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_LOCAL_REQUESTS_WRITES_SUMMARY_JSON_PATH          | read-only "true"  | *eksconfig.AddOnConfigmapsLocal.RequestsWritesSummaryJSONPath         | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_LOCAL_REQUESTS_WRITES_SUMMARY_TABLE_PATH         | read-only "true"  | *eksconfig.AddOnConfigmapsLocal.RequestsWritesSummaryTablePath        | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_LOCAL_REQUESTS_WRITES_SUMMARY_S3_DIR             | read-only "false" | *eksconfig.AddOnConfigmapsLocal.RequestsWritesSummaryS3Dir            | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_LOCAL_REQUESTS_WRITES_SUMMARY_COMPARE            | read-only "true"  | *eksconfig.AddOnConfigmapsLocal.RequestsWritesSummaryCompare          | metrics.RequestsSummaryCompare |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_LOCAL_REQUESTS_WRITES_SUMMARY_COMPARE_JSON_PATH  | read-only "true"  | *eksconfig.AddOnConfigmapsLocal.RequestsWritesSummaryCompareJSONPath  | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_LOCAL_REQUESTS_WRITES_SUMMARY_COMPARE_TABLE_PATH | read-only "true"  | *eksconfig.AddOnConfigmapsLocal.RequestsWritesSummaryCompareTablePath | string                         |
*---------------------------------------------------------------------------------------*-------------------*-----------------------------------------------------------------------*--------------------------------*


*----------------------------------------------------------------------------------------*-------------------*------------------------------------------------------------------------*--------------------------------*
|                                 ENVIRONMENTAL VARIABLE                                 |     READ ONLY     |                                  TYPE                                  |            GO TYPE             |
*----------------------------------------------------------------------------------------*-------------------*------------------------------------------------------------------------*--------------------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_REMOTE_ENABLE                                     | read-only "false" | *eksconfig.AddOnConfigmapsRemote.Enable                                | bool                           |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_REMOTE_CREATED                                    | read-only "true"  | *eksconfig.AddOnConfigmapsRemote.Created                               | bool                           |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_REMOTE_TIME_FRAME_CREATE                          | read-only "true"  | *eksconfig.AddOnConfigmapsRemote.TimeFrameCreate                       | timeutil.TimeFrame             |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_REMOTE_TIME_FRAME_DELETE                          | read-only "true"  | *eksconfig.AddOnConfigmapsRemote.TimeFrameDelete                       | timeutil.TimeFrame             |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_REMOTE_NAMESPACE                                  | read-only "false" | *eksconfig.AddOnConfigmapsRemote.Namespace                             | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_REMOTE_REPOSITORY_ACCOUNT_ID                      | read-only "false" | *eksconfig.AddOnConfigmapsRemote.RepositoryAccountID                   | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_REMOTE_REPOSITORY_NAME                            | read-only "false" | *eksconfig.AddOnConfigmapsRemote.RepositoryName                        | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_REMOTE_REPOSITORY_IMAGE_TAG                       | read-only "false" | *eksconfig.AddOnConfigmapsRemote.RepositoryImageTag                    | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_REMOTE_DEPLOYMENT_REPLICAS                        | read-only "false" | *eksconfig.AddOnConfigmapsRemote.DeploymentReplicas                    | int32                          |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_REMOTE_OBJECTS                                    | read-only "false" | *eksconfig.AddOnConfigmapsRemote.Objects                               | int                            |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_REMOTE_OBJECT_SIZE                                | read-only "false" | *eksconfig.AddOnConfigmapsRemote.ObjectSize                            | int                            |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_REMOTE_CREATED_NAMES                              | read-only "true"  | *eksconfig.AddOnConfigmapsRemote.CreatedNames                          | []string                       |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_REMOTE_REQUESTS_WRITES_JSON_PATH                  | read-only "true"  | *eksconfig.AddOnConfigmapsRemote.RequestsWritesJSONPath                | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_REMOTE_REQUESTS_WRITES_SUMMARY                    | read-only "true"  | *eksconfig.AddOnConfigmapsRemote.RequestsWritesSummary                 | metrics.RequestsSummary        |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_REMOTE_REQUESTS_WRITES_SUMMARY_JSON_PATH          | read-only "true"  | *eksconfig.AddOnConfigmapsRemote.RequestsWritesSummaryJSONPath         | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_REMOTE_REQUESTS_WRITES_SUMMARY_TABLE_PATH         | read-only "true"  | *eksconfig.AddOnConfigmapsRemote.RequestsWritesSummaryTablePath        | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_REMOTE_REQUESTS_WRITES_SUMMARY_S3_DIR             | read-only "false" | *eksconfig.AddOnConfigmapsRemote.RequestsWritesSummaryS3Dir            | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_REMOTE_REQUESTS_WRITES_SUMMARY_COMPARE            | read-only "true"  | *eksconfig.AddOnConfigmapsRemote.RequestsWritesSummaryCompare          | metrics.RequestsSummaryCompare |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_REMOTE_REQUESTS_WRITES_SUMMARY_COMPARE_JSON_PATH  | read-only "true"  | *eksconfig.AddOnConfigmapsRemote.RequestsWritesSummaryCompareJSONPath  | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_REMOTE_REQUESTS_WRITES_SUMMARY_COMPARE_TABLE_PATH | read-only "true"  | *eksconfig.AddOnConfigmapsRemote.RequestsWritesSummaryCompareTablePath | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_REMOTE_REQUESTS_WRITES_SUMMARY_OUTPUT_NAME_PREFIX | read-only "false" | *eksconfig.AddOnConfigmapsRemote.RequestsWritesSummaryOutputNamePrefix | string                         |
*----------------------------------------------------------------------------------------*-------------------*------------------------------------------------------------------------*--------------------------------*


*------------------------------------------------------------------------------------*-------------------*--------------------------------------------------------------------*--------------------------------*
|                               ENVIRONMENTAL VARIABLE                               |     READ ONLY     |                                TYPE                                |            GO TYPE             |
*------------------------------------------------------------------------------------*-------------------*--------------------------------------------------------------------*--------------------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_ENABLE                                     | read-only "false" | *eksconfig.AddOnSecretsLocal.Enable                                | bool                           |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_CREATED                                    | read-only "true"  | *eksconfig.AddOnSecretsLocal.Created                               | bool                           |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_TIME_FRAME_CREATE                          | read-only "true"  | *eksconfig.AddOnSecretsLocal.TimeFrameCreate                       | timeutil.TimeFrame             |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_TIME_FRAME_DELETE                          | read-only "true"  | *eksconfig.AddOnSecretsLocal.TimeFrameDelete                       | timeutil.TimeFrame             |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_NAMESPACE                                  | read-only "false" | *eksconfig.AddOnSecretsLocal.Namespace                             | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_OBJECTS                                    | read-only "false" | *eksconfig.AddOnSecretsLocal.Objects                               | int                            |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_OBJECT_SIZE                                | read-only "false" | *eksconfig.AddOnSecretsLocal.ObjectSize                            | int                            |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_NAME_PREFIX                                | read-only "false" | *eksconfig.AddOnSecretsLocal.NamePrefix                            | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_REQUESTS_WRITES_JSON_PATH                  | read-only "true"  | *eksconfig.AddOnSecretsLocal.RequestsWritesJSONPath                | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_REQUESTS_WRITES_SUMMARY                    | read-only "true"  | *eksconfig.AddOnSecretsLocal.RequestsWritesSummary                 | metrics.RequestsSummary        |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_REQUESTS_WRITES_SUMMARY_JSON_PATH          | read-only "true"  | *eksconfig.AddOnSecretsLocal.RequestsWritesSummaryJSONPath         | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_REQUESTS_WRITES_SUMMARY_TABLE_PATH         | read-only "true"  | *eksconfig.AddOnSecretsLocal.RequestsWritesSummaryTablePath        | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_REQUESTS_WRITES_SUMMARY_S3_DIR             | read-only "false" | *eksconfig.AddOnSecretsLocal.RequestsWritesSummaryS3Dir            | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_REQUESTS_WRITES_SUMMARY_COMPARE            | read-only "true"  | *eksconfig.AddOnSecretsLocal.RequestsWritesSummaryCompare          | metrics.RequestsSummaryCompare |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_REQUESTS_WRITES_SUMMARY_COMPARE_JSON_PATH  | read-only "true"  | *eksconfig.AddOnSecretsLocal.RequestsWritesSummaryCompareJSONPath  | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_REQUESTS_WRITES_SUMMARY_COMPARE_TABLE_PATH | read-only "true"  | *eksconfig.AddOnSecretsLocal.RequestsWritesSummaryCompareTablePath | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_REQUESTS_READS_JSON_PATH                   | read-only "true"  | *eksconfig.AddOnSecretsLocal.RequestsReadsJSONPath                 | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_REQUESTS_READS_SUMMARY                     | read-only "true"  | *eksconfig.AddOnSecretsLocal.RequestsReadsSummary                  | metrics.RequestsSummary        |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_REQUESTS_READS_SUMMARY_JSON_PATH           | read-only "true"  | *eksconfig.AddOnSecretsLocal.RequestsReadsSummaryJSONPath          | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_REQUESTS_READS_SUMMARY_TABLE_PATH          | read-only "true"  | *eksconfig.AddOnSecretsLocal.RequestsReadsSummaryTablePath         | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_REQUESTS_READS_SUMMARY_S3_DIR              | read-only "false" | *eksconfig.AddOnSecretsLocal.RequestsReadsSummaryS3Dir             | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_REQUESTS_READS_SUMMARY_COMPARE             | read-only "true"  | *eksconfig.AddOnSecretsLocal.RequestsReadsSummaryCompare           | metrics.RequestsSummaryCompare |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_REQUESTS_READS_SUMMARY_COMPARE_JSON_PATH   | read-only "true"  | *eksconfig.AddOnSecretsLocal.RequestsReadsSummaryCompareJSONPath   | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_REQUESTS_READS_SUMMARY_COMPARE_TABLE_PATH  | read-only "true"  | *eksconfig.AddOnSecretsLocal.RequestsReadsSummaryCompareTablePath  | string                         |
*------------------------------------------------------------------------------------*-------------------*--------------------------------------------------------------------*--------------------------------*


*-------------------------------------------------------------------------------------*-------------------*---------------------------------------------------------------------*--------------------------------*
|                               ENVIRONMENTAL VARIABLE                                |     READ ONLY     |                                TYPE                                 |            GO TYPE             |
*-------------------------------------------------------------------------------------*-------------------*---------------------------------------------------------------------*--------------------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_ENABLE                                     | read-only "false" | *eksconfig.AddOnSecretsRemote.Enable                                | bool                           |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_CREATED                                    | read-only "true"  | *eksconfig.AddOnSecretsRemote.Created                               | bool                           |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_TIME_FRAME_CREATE                          | read-only "true"  | *eksconfig.AddOnSecretsRemote.TimeFrameCreate                       | timeutil.TimeFrame             |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_TIME_FRAME_DELETE                          | read-only "true"  | *eksconfig.AddOnSecretsRemote.TimeFrameDelete                       | timeutil.TimeFrame             |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_NAMESPACE                                  | read-only "false" | *eksconfig.AddOnSecretsRemote.Namespace                             | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REPOSITORY_ACCOUNT_ID                      | read-only "false" | *eksconfig.AddOnSecretsRemote.RepositoryAccountID                   | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REPOSITORY_NAME                            | read-only "false" | *eksconfig.AddOnSecretsRemote.RepositoryName                        | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REPOSITORY_IMAGE_TAG                       | read-only "false" | *eksconfig.AddOnSecretsRemote.RepositoryImageTag                    | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_DEPLOYMENT_REPLICAS                        | read-only "false" | *eksconfig.AddOnSecretsRemote.DeploymentReplicas                    | int32                          |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_OBJECTS                                    | read-only "false" | *eksconfig.AddOnSecretsRemote.Objects                               | int                            |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_OBJECT_SIZE                                | read-only "false" | *eksconfig.AddOnSecretsRemote.ObjectSize                            | int                            |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_NAME_PREFIX                                | read-only "false" | *eksconfig.AddOnSecretsRemote.NamePrefix                            | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REQUESTS_WRITES_JSON_PATH                  | read-only "true"  | *eksconfig.AddOnSecretsRemote.RequestsWritesJSONPath                | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REQUESTS_WRITES_SUMMARY                    | read-only "true"  | *eksconfig.AddOnSecretsRemote.RequestsWritesSummary                 | metrics.RequestsSummary        |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REQUESTS_WRITES_SUMMARY_JSON_PATH          | read-only "true"  | *eksconfig.AddOnSecretsRemote.RequestsWritesSummaryJSONPath         | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REQUESTS_WRITES_SUMMARY_TABLE_PATH         | read-only "true"  | *eksconfig.AddOnSecretsRemote.RequestsWritesSummaryTablePath        | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REQUESTS_WRITES_SUMMARY_S3_DIR             | read-only "false" | *eksconfig.AddOnSecretsRemote.RequestsWritesSummaryS3Dir            | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REQUESTS_WRITES_SUMMARY_COMPARE            | read-only "true"  | *eksconfig.AddOnSecretsRemote.RequestsWritesSummaryCompare          | metrics.RequestsSummaryCompare |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REQUESTS_WRITES_SUMMARY_COMPARE_JSON_PATH  | read-only "true"  | *eksconfig.AddOnSecretsRemote.RequestsWritesSummaryCompareJSONPath  | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REQUESTS_WRITES_SUMMARY_COMPARE_TABLE_PATH | read-only "true"  | *eksconfig.AddOnSecretsRemote.RequestsWritesSummaryCompareTablePath | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REQUESTS_READS_JSON_PATH                   | read-only "true"  | *eksconfig.AddOnSecretsRemote.RequestsReadsJSONPath                 | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REQUESTS_READS_SUMMARY                     | read-only "true"  | *eksconfig.AddOnSecretsRemote.RequestsReadsSummary                  | metrics.RequestsSummary        |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REQUESTS_READS_SUMMARY_JSON_PATH           | read-only "true"  | *eksconfig.AddOnSecretsRemote.RequestsReadsSummaryJSONPath          | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REQUESTS_READS_SUMMARY_TABLE_PATH          | read-only "true"  | *eksconfig.AddOnSecretsRemote.RequestsReadsSummaryTablePath         | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REQUESTS_READS_SUMMARY_S3_DIR              | read-only "false" | *eksconfig.AddOnSecretsRemote.RequestsReadsSummaryS3Dir             | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REQUESTS_READS_SUMMARY_COMPARE             | read-only "true"  | *eksconfig.AddOnSecretsRemote.RequestsReadsSummaryCompare           | metrics.RequestsSummaryCompare |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REQUESTS_READS_SUMMARY_COMPARE_JSON_PATH   | read-only "true"  | *eksconfig.AddOnSecretsRemote.RequestsReadsSummaryCompareJSONPath   | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REQUESTS_READS_SUMMARY_COMPARE_TABLE_PATH  | read-only "true"  | *eksconfig.AddOnSecretsRemote.RequestsReadsSummaryCompareTablePath  | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REQUESTS_WRITES_SUMMARY_OUTPUT_NAME_PREFIX | read-only "false" | *eksconfig.AddOnSecretsRemote.RequestsWritesSummaryOutputNamePrefix | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_REQUESTS_READS_SUMMARY_OUTPUT_NAME_PREFIX  | read-only "false" | *eksconfig.AddOnSecretsRemote.RequestsReadsSummaryOutputNamePrefix  | string                         |
*-------------------------------------------------------------------------------------*-------------------*---------------------------------------------------------------------*--------------------------------*


*-----------------------------------------------------------------*-------------------*--------------------------------------------------*--------------------*
|                     ENVIRONMENTAL VARIABLE                      |     READ ONLY     |                       TYPE                       |      GO TYPE       |
*-----------------------------------------------------------------*-------------------*--------------------------------------------------*--------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_ENABLE                        | read-only "false" | *eksconfig.AddOnFargate.Enable                   | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_CREATED                       | read-only "true"  | *eksconfig.AddOnFargate.Created                  | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_TIME_FRAME_CREATE             | read-only "true"  | *eksconfig.AddOnFargate.TimeFrameCreate          | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_TIME_FRAME_DELETE             | read-only "true"  | *eksconfig.AddOnFargate.TimeFrameDelete          | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_NAMESPACE                     | read-only "false" | *eksconfig.AddOnFargate.Namespace                | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_REPOSITORY_ACCOUNT_ID         | read-only "false" | *eksconfig.AddOnFargate.RepositoryAccountID      | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_REPOSITORY_NAME               | read-only "false" | *eksconfig.AddOnFargate.RepositoryName           | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_REPOSITORY_IMAGE_TAG          | read-only "false" | *eksconfig.AddOnFargate.RepositoryImageTag       | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_ROLE_NAME                     | read-only "false" | *eksconfig.AddOnFargate.RoleName                 | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_ROLE_CREATE                   | read-only "false" | *eksconfig.AddOnFargate.RoleCreate               | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_ROLE_ARN                      | read-only "false" | *eksconfig.AddOnFargate.RoleARN                  | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_ROLE_SERVICE_PRINCIPALS       | read-only "false" | *eksconfig.AddOnFargate.RoleServicePrincipals    | []string           |
| AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_ROLE_MANAGED_POLICY_ARNS      | read-only "false" | *eksconfig.AddOnFargate.RoleManagedPolicyARNs    | []string           |
| AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_ROLE_CFN_STACK_ID             | read-only "true"  | *eksconfig.AddOnFargate.RoleCFNStackID           | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_ROLE_CFN_STACK_YAML_FILE_PATH | read-only "true"  | *eksconfig.AddOnFargate.RoleCFNStackYAMLFilePath | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_PROFILE_NAME                  | read-only "false" | *eksconfig.AddOnFargate.ProfileName              | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_SECRET_NAME                   | read-only "false" | *eksconfig.AddOnFargate.SecretName               | string             |
*-----------------------------------------------------------------*-------------------*--------------------------------------------------*--------------------*


*--------------------------------------------------------------*-------------------*-----------------------------------------------*--------------------*
|                    ENVIRONMENTAL VARIABLE                    |     READ ONLY     |                     TYPE                      |      GO TYPE       |
*--------------------------------------------------------------*-------------------*-----------------------------------------------*--------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_ENABLE                        | read-only "false" | *eksconfig.AddOnIRSA.Enable                   | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_CREATED                       | read-only "true"  | *eksconfig.AddOnIRSA.Created                  | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_TIME_FRAME_CREATE             | read-only "true"  | *eksconfig.AddOnIRSA.TimeFrameCreate          | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_TIME_FRAME_DELETE             | read-only "true"  | *eksconfig.AddOnIRSA.TimeFrameDelete          | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_NAMESPACE                     | read-only "false" | *eksconfig.AddOnIRSA.Namespace                | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_REPOSITORY_ACCOUNT_ID         | read-only "false" | *eksconfig.AddOnIRSA.RepositoryAccountID      | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_REPOSITORY_NAME               | read-only "false" | *eksconfig.AddOnIRSA.RepositoryName           | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_REPOSITORY_IMAGE_TAG          | read-only "false" | *eksconfig.AddOnIRSA.RepositoryImageTag       | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_ROLE_NAME                     | read-only "false" | *eksconfig.AddOnIRSA.RoleName                 | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_ROLE_ARN                      | read-only "false" | *eksconfig.AddOnIRSA.RoleARN                  | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_ROLE_MANAGED_POLICY_ARNS      | read-only "false" | *eksconfig.AddOnIRSA.RoleManagedPolicyARNs    | []string           |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_ROLE_CFN_STACK_ID             | read-only "true"  | *eksconfig.AddOnIRSA.RoleCFNStackID           | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_ROLE_CFN_STACK_YAML_FILE_PATH | read-only "true"  | *eksconfig.AddOnIRSA.RoleCFNStackYAMLFilePath | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_S3_KEY                        | read-only "false" | *eksconfig.AddOnIRSA.S3Key                    | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_DEPLOYMENT_REPLICAS           | read-only "false" | *eksconfig.AddOnIRSA.DeploymentReplicas       | int32              |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_DEPLOYMENT_RESULT_PATH        | read-only "false" | *eksconfig.AddOnIRSA.DeploymentResultPath     | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_DEPLOYMENT_TOOK               | read-only "true"  | *eksconfig.AddOnIRSA.DeploymentTook           | time.Duration      |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_DEPLOYMENT_TOOK_STRING        | read-only "true"  | *eksconfig.AddOnIRSA.DeploymentTookString     | string             |
*--------------------------------------------------------------*-------------------*-----------------------------------------------*--------------------*


*----------------------------------------------------------------------*-------------------*------------------------------------------------------*--------------------*
|                        ENVIRONMENTAL VARIABLE                        |     READ ONLY     |                         TYPE                         |      GO TYPE       |
*----------------------------------------------------------------------*-------------------*------------------------------------------------------*--------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_ENABLE                        | read-only "false" | *eksconfig.AddOnIRSAFargate.Enable                   | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_CREATED                       | read-only "true"  | *eksconfig.AddOnIRSAFargate.Created                  | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_TIME_FRAME_CREATE             | read-only "true"  | *eksconfig.AddOnIRSAFargate.TimeFrameCreate          | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_TIME_FRAME_DELETE             | read-only "true"  | *eksconfig.AddOnIRSAFargate.TimeFrameDelete          | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_NAMESPACE                     | read-only "false" | *eksconfig.AddOnIRSAFargate.Namespace                | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_REPOSITORY_ACCOUNT_ID         | read-only "false" | *eksconfig.AddOnIRSAFargate.RepositoryAccountID      | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_REPOSITORY_NAME               | read-only "false" | *eksconfig.AddOnIRSAFargate.RepositoryName           | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_REPOSITORY_IMAGE_TAG          | read-only "false" | *eksconfig.AddOnIRSAFargate.RepositoryImageTag       | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_ROLE_NAME                     | read-only "false" | *eksconfig.AddOnIRSAFargate.RoleName                 | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_ROLE_ARN                      | read-only "false" | *eksconfig.AddOnIRSAFargate.RoleARN                  | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_ROLE_SERVICE_PRINCIPALS       | read-only "false" | *eksconfig.AddOnIRSAFargate.RoleServicePrincipals    | []string           |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_ROLE_MANAGED_POLICY_ARNS      | read-only "false" | *eksconfig.AddOnIRSAFargate.RoleManagedPolicyARNs    | []string           |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_ROLE_CFN_STACK_ID             | read-only "true"  | *eksconfig.AddOnIRSAFargate.RoleCFNStackID           | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_ROLE_CFN_STACK_YAML_FILE_PATH | read-only "true"  | *eksconfig.AddOnIRSAFargate.RoleCFNStackYAMLFilePath | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_S3_KEY                        | read-only "false" | *eksconfig.AddOnIRSAFargate.S3Key                    | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_PROFILE_NAME                  | read-only "false" | *eksconfig.AddOnIRSAFargate.ProfileName              | string             |
*----------------------------------------------------------------------*-------------------*------------------------------------------------------*--------------------*


*-------------------------------------------------------*-------------------*-------------------------------------------*--------------------*
|                ENVIRONMENTAL VARIABLE                 |     READ ONLY     |                   TYPE                    |      GO TYPE       |
*-------------------------------------------------------*-------------------*-------------------------------------------*--------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_WORDPRESS_ENABLE            | read-only "false" | *eksconfig.AddOnWordpress.Enable          | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_WORDPRESS_CREATED           | read-only "true"  | *eksconfig.AddOnWordpress.Created         | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_WORDPRESS_TIME_FRAME_CREATE | read-only "true"  | *eksconfig.AddOnWordpress.TimeFrameCreate | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_WORDPRESS_TIME_FRAME_DELETE | read-only "true"  | *eksconfig.AddOnWordpress.TimeFrameDelete | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_WORDPRESS_NAMESPACE         | read-only "false" | *eksconfig.AddOnWordpress.Namespace       | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_WORDPRESS_USER_NAME         | read-only "false" | *eksconfig.AddOnWordpress.UserName        | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_WORDPRESS_PASSWORD          | read-only "false" | *eksconfig.AddOnWordpress.Password        | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_WORDPRESS_NLB_ARN           | read-only "true"  | *eksconfig.AddOnWordpress.NLBARN          | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_WORDPRESS_NLB_NAME          | read-only "true"  | *eksconfig.AddOnWordpress.NLBName         | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_WORDPRESS_URL               | read-only "true"  | *eksconfig.AddOnWordpress.URL             | string             |
*-------------------------------------------------------*-------------------*-------------------------------------------*--------------------*


*----------------------------------------------------------*-------------------*---------------------------------------------*--------------------*
|                  ENVIRONMENTAL VARIABLE                  |     READ ONLY     |                    TYPE                     |      GO TYPE       |
*----------------------------------------------------------*-------------------*---------------------------------------------*--------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_JUPYTER_HUB_ENABLE             | read-only "false" | *eksconfig.AddOnJupyterHub.Enable           | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_JUPYTER_HUB_CREATED            | read-only "true"  | *eksconfig.AddOnJupyterHub.Created          | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_JUPYTER_HUB_TIME_FRAME_CREATE  | read-only "true"  | *eksconfig.AddOnJupyterHub.TimeFrameCreate  | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_JUPYTER_HUB_TIME_FRAME_DELETE  | read-only "true"  | *eksconfig.AddOnJupyterHub.TimeFrameDelete  | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_JUPYTER_HUB_NAMESPACE          | read-only "false" | *eksconfig.AddOnJupyterHub.Namespace        | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_JUPYTER_HUB_PROXY_SECRET_TOKEN | read-only "false" | *eksconfig.AddOnJupyterHub.ProxySecretToken | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_JUPYTER_HUB_NLB_ARN            | read-only "true"  | *eksconfig.AddOnJupyterHub.NLBARN           | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_JUPYTER_HUB_NLB_NAME           | read-only "true"  | *eksconfig.AddOnJupyterHub.NLBName          | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_JUPYTER_HUB_URL                | read-only "true"  | *eksconfig.AddOnJupyterHub.URL              | string             |
*----------------------------------------------------------*-------------------*---------------------------------------------*--------------------*


# NOT WORKING...
*-------------------------------------------------------*-------------------*-------------------------------------------*--------------------*
|                ENVIRONMENTAL VARIABLE                 |     READ ONLY     |                   TYPE                    |      GO TYPE       |
*-------------------------------------------------------*-------------------*-------------------------------------------*--------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_KUBEFLOW_ENABLE             | read-only "false" | *eksconfig.AddOnKubeflow.Enable           | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_KUBEFLOW_CREATED            | read-only "true"  | *eksconfig.AddOnKubeflow.Created          | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_KUBEFLOW_TIME_FRAME_CREATE  | read-only "true"  | *eksconfig.AddOnKubeflow.TimeFrameCreate  | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_KUBEFLOW_TIME_FRAME_DELETE  | read-only "true"  | *eksconfig.AddOnKubeflow.TimeFrameDelete  | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_KUBEFLOW_KFCTL_PATH         | read-only "false" | *eksconfig.AddOnKubeflow.KfctlPath        | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_KUBEFLOW_KFCTL_DOWNLOAD_URL | read-only "false" | *eksconfig.AddOnKubeflow.KfctlDownloadURL | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_KUBEFLOW_BASE_DIR           | read-only "false" | *eksconfig.AddOnKubeflow.BaseDir          | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_KUBEFLOW_KF_DIR             | read-only "true"  | *eksconfig.AddOnKubeflow.KfDir            | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_KUBEFLOW_KFCTL_CONFIG_PATH  | read-only "true"  | *eksconfig.AddOnKubeflow.KfctlConfigPath  | string             |
*-------------------------------------------------------*-------------------*-------------------------------------------*--------------------*


*-------------------------------------------------------------*-------------------*-----------------------------------------------*--------------------*
|                   ENVIRONMENTAL VARIABLE                    |     READ ONLY     |                     TYPE                      |      GO TYPE       |
*-------------------------------------------------------------*-------------------*-----------------------------------------------*--------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_CUDA_VECTOR_ADD_ENABLE            | read-only "false" | *eksconfig.AddOnCUDAVectorAdd.Enable          | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_CUDA_VECTOR_ADD_CREATED           | read-only "true"  | *eksconfig.AddOnCUDAVectorAdd.Created         | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_CUDA_VECTOR_ADD_TIME_FRAME_CREATE | read-only "true"  | *eksconfig.AddOnCUDAVectorAdd.TimeFrameCreate | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_CUDA_VECTOR_ADD_TIME_FRAME_DELETE | read-only "true"  | *eksconfig.AddOnCUDAVectorAdd.TimeFrameDelete | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_CUDA_VECTOR_ADD_NAMESPACE         | read-only "false" | *eksconfig.AddOnCUDAVectorAdd.Namespace       | string             |
*-------------------------------------------------------------*-------------------*-----------------------------------------------*--------------------*


*-----------------------------------------------------------------------------------*-------------------*-------------------------------------------------------------------*--------------------*
|                              ENVIRONMENTAL VARIABLE                               |     READ ONLY     |                               TYPE                                |      GO TYPE       |
*-----------------------------------------------------------------------------------*-------------------*-------------------------------------------------------------------*--------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_ENABLE                             | read-only "false" | *eksconfig.AddOnClusterLoaderLocal.Enable                         | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_CREATED                            | read-only "true"  | *eksconfig.AddOnClusterLoaderLocal.Created                        | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_TIME_FRAME_CREATE                  | read-only "true"  | *eksconfig.AddOnClusterLoaderLocal.TimeFrameCreate                | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_TIME_FRAME_DELETE                  | read-only "true"  | *eksconfig.AddOnClusterLoaderLocal.TimeFrameDelete                | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_CLUSTER_LOADER_PATH                | read-only "false" | *eksconfig.AddOnClusterLoaderLocal.ClusterLoaderPath              | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_CLUSTER_LOADER_DOWNLOAD_URL        | read-only "false" | *eksconfig.AddOnClusterLoaderLocal.ClusterLoaderDownloadURL       | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_TEST_CONFIG_PATH                   | read-only "false" | *eksconfig.AddOnClusterLoaderLocal.TestConfigPath                 | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_REPORT_DIR                         | read-only "false" | *eksconfig.AddOnClusterLoaderLocal.ReportDir                      | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_REPORT_TAR_GZ_PATH                 | read-only "true"  | *eksconfig.AddOnClusterLoaderLocal.ReportTarGzPath                | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_LOG_PATH                           | read-only "true"  | *eksconfig.AddOnClusterLoaderLocal.LogPath                        | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_RUNS                               | read-only "false" | *eksconfig.AddOnClusterLoaderLocal.Runs                           | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_TIMEOUT                            | read-only "false" | *eksconfig.AddOnClusterLoaderLocal.Timeout                        | time.Duration      |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_NODES                              | read-only "false" | *eksconfig.AddOnClusterLoaderLocal.Nodes                          | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_NODES_PER_NAMESPACE                | read-only "false" | *eksconfig.AddOnClusterLoaderLocal.NodesPerNamespace              | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_PODS_PER_NODE                      | read-only "false" | *eksconfig.AddOnClusterLoaderLocal.PodsPerNode                    | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_BIG_GROUP_SIZE                     | read-only "false" | *eksconfig.AddOnClusterLoaderLocal.BigGroupSize                   | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_MEDIUM_GROUP_SIZE                  | read-only "false" | *eksconfig.AddOnClusterLoaderLocal.MediumGroupSize                | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_SMALL_GROUP_SIZE                   | read-only "false" | *eksconfig.AddOnClusterLoaderLocal.SmallGroupSize                 | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_SMALL_STATEFUL_SETS_PER_NAMESPACE  | read-only "false" | *eksconfig.AddOnClusterLoaderLocal.SmallStatefulSetsPerNamespace  | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_MEDIUM_STATEFUL_SETS_PER_NAMESPACE | read-only "false" | *eksconfig.AddOnClusterLoaderLocal.MediumStatefulSetsPerNamespace | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_CL2_LOAD_TEST_THROUGHPUT           | read-only "false" | *eksconfig.AddOnClusterLoaderLocal.CL2LoadTestThroughput          | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_CL2_ENABLE_PVS                     | read-only "false" | *eksconfig.AddOnClusterLoaderLocal.CL2EnablePVS                   | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_PROMETHEUS_SCRAPE_KUBE_PROXY       | read-only "false" | *eksconfig.AddOnClusterLoaderLocal.PrometheusScrapeKubeProxy      | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_ENABLE_SYSTEM_POD_METRICS          | read-only "false" | *eksconfig.AddOnClusterLoaderLocal.EnableSystemPodMetrics         | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_POD_STARTUP_LATENCY                | read-only "true"  | *eksconfig.AddOnClusterLoaderLocal.PodStartupLatency              | util.PerfData      |
*-----------------------------------------------------------------------------------*-------------------*-------------------------------------------------------------------*--------------------*


*------------------------------------------------------------------------------------*-------------------*--------------------------------------------------------------------*--------------------*
|                               ENVIRONMENTAL VARIABLE                               |     READ ONLY     |                                TYPE                                |      GO TYPE       |
*------------------------------------------------------------------------------------*-------------------*--------------------------------------------------------------------*--------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_ENABLE                             | read-only "false" | *eksconfig.AddOnClusterLoaderRemote.Enable                         | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_CREATED                            | read-only "true"  | *eksconfig.AddOnClusterLoaderRemote.Created                        | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_TIME_FRAME_CREATE                  | read-only "true"  | *eksconfig.AddOnClusterLoaderRemote.TimeFrameCreate                | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_TIME_FRAME_DELETE                  | read-only "true"  | *eksconfig.AddOnClusterLoaderRemote.TimeFrameDelete                | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_NAMESPACE                          | read-only "false" | *eksconfig.AddOnClusterLoaderRemote.Namespace                      | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_REPOSITORY_ACCOUNT_ID              | read-only "false" | *eksconfig.AddOnClusterLoaderRemote.RepositoryAccountID            | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_REPOSITORY_NAME                    | read-only "false" | *eksconfig.AddOnClusterLoaderRemote.RepositoryName                 | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_REPOSITORY_IMAGE_TAG               | read-only "false" | *eksconfig.AddOnClusterLoaderRemote.RepositoryImageTag             | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_DEPLOYMENT_REPLICAS                | read-only "true"  | *eksconfig.AddOnClusterLoaderRemote.DeploymentReplicas             | int32              |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_CLUSTER_LOADER_PATH                | read-only "false" | *eksconfig.AddOnClusterLoaderRemote.ClusterLoaderPath              | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_CLUSTER_LOADER_DOWNLOAD_URL        | read-only "false" | *eksconfig.AddOnClusterLoaderRemote.ClusterLoaderDownloadURL       | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_REPORT_TAR_GZ_PATH                 | read-only "true"  | *eksconfig.AddOnClusterLoaderRemote.ReportTarGzPath                | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_LOG_PATH                           | read-only "true"  | *eksconfig.AddOnClusterLoaderRemote.LogPath                        | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_RUNS                               | read-only "false" | *eksconfig.AddOnClusterLoaderRemote.Runs                           | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_NODES                              | read-only "false" | *eksconfig.AddOnClusterLoaderRemote.Nodes                          | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_TIMEOUT                            | read-only "false" | *eksconfig.AddOnClusterLoaderRemote.Timeout                        | time.Duration      |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_NODES_PER_NAMESPACE                | read-only "false" | *eksconfig.AddOnClusterLoaderRemote.NodesPerNamespace              | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_PODS_PER_NODE                      | read-only "false" | *eksconfig.AddOnClusterLoaderRemote.PodsPerNode                    | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_BIG_GROUP_SIZE                     | read-only "false" | *eksconfig.AddOnClusterLoaderRemote.BigGroupSize                   | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_MEDIUM_GROUP_SIZE                  | read-only "false" | *eksconfig.AddOnClusterLoaderRemote.MediumGroupSize                | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_SMALL_GROUP_SIZE                   | read-only "false" | *eksconfig.AddOnClusterLoaderRemote.SmallGroupSize                 | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_SMALL_STATEFUL_SETS_PER_NAMESPACE  | read-only "false" | *eksconfig.AddOnClusterLoaderRemote.SmallStatefulSetsPerNamespace  | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_MEDIUM_STATEFUL_SETS_PER_NAMESPACE | read-only "false" | *eksconfig.AddOnClusterLoaderRemote.MediumStatefulSetsPerNamespace | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_CL2_LOAD_TEST_THROUGHPUT           | read-only "false" | *eksconfig.AddOnClusterLoaderRemote.CL2LoadTestThroughput          | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_CL2_ENABLE_PVS                     | read-only "false" | *eksconfig.AddOnClusterLoaderRemote.CL2EnablePVS                   | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_PROMETHEUS_SCRAPE_KUBE_PROXY       | read-only "false" | *eksconfig.AddOnClusterLoaderRemote.PrometheusScrapeKubeProxy      | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_ENABLE_SYSTEM_POD_METRICS          | read-only "false" | *eksconfig.AddOnClusterLoaderRemote.EnableSystemPodMetrics         | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_REMOTE_POD_STARTUP_LATENCY                | read-only "true"  | *eksconfig.AddOnClusterLoaderRemote.PodStartupLatency              | util.PerfData      |
*------------------------------------------------------------------------------------*-------------------*--------------------------------------------------------------------*--------------------*


*-----------------------------------------------------------------*-------------------*---------------------------------------------------*--------------------*
|                     ENVIRONMENTAL VARIABLE                      |     READ ONLY     |                       TYPE                        |      GO TYPE       |
*-----------------------------------------------------------------*-------------------*---------------------------------------------------*--------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_LOCAL_ENABLE             | read-only "false" | *eksconfig.AddOnHollowNodesLocal.Enable           | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_LOCAL_CREATED            | read-only "true"  | *eksconfig.AddOnHollowNodesLocal.Created          | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_LOCAL_TIME_FRAME_CREATE  | read-only "true"  | *eksconfig.AddOnHollowNodesLocal.TimeFrameCreate  | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_LOCAL_TIME_FRAME_DELETE  | read-only "true"  | *eksconfig.AddOnHollowNodesLocal.TimeFrameDelete  | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_LOCAL_NODES              | read-only "false" | *eksconfig.AddOnHollowNodesLocal.Nodes            | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_LOCAL_NODE_NAME_PREFIX   | read-only "false" | *eksconfig.AddOnHollowNodesLocal.NodeNamePrefix   | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_LOCAL_NODE_LABEL_PREFIX  | read-only "false" | *eksconfig.AddOnHollowNodesLocal.NodeLabelPrefix  | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_LOCAL_NODE_LABELS        | read-only "true"  | *eksconfig.AddOnHollowNodesLocal.NodeLabels       | map[string]string  |
| AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_LOCAL_MAX_OPEN_FILES     | read-only "false" | *eksconfig.AddOnHollowNodesLocal.MaxOpenFiles     | int64              |
| AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_LOCAL_CREATED_NODE_NAMES | read-only "true"  | *eksconfig.AddOnHollowNodesLocal.CreatedNodeNames | []string           |
*-----------------------------------------------------------------*-------------------*---------------------------------------------------*--------------------*


*---------------------------------------------------------------------*-------------------*-------------------------------------------------------*--------------------*
|                       ENVIRONMENTAL VARIABLE                        |     READ ONLY     |                         TYPE                          |      GO TYPE       |
*---------------------------------------------------------------------*-------------------*-------------------------------------------------------*--------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_ENABLE                | read-only "false" | *eksconfig.AddOnHollowNodesRemote.Enable              | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_CREATED               | read-only "true"  | *eksconfig.AddOnHollowNodesRemote.Created             | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_TIME_FRAME_CREATE     | read-only "true"  | *eksconfig.AddOnHollowNodesRemote.TimeFrameCreate     | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_TIME_FRAME_DELETE     | read-only "true"  | *eksconfig.AddOnHollowNodesRemote.TimeFrameDelete     | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_NAMESPACE             | read-only "false" | *eksconfig.AddOnHollowNodesRemote.Namespace           | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_REPOSITORY_ACCOUNT_ID | read-only "false" | *eksconfig.AddOnHollowNodesRemote.RepositoryAccountID | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_REPOSITORY_NAME       | read-only "false" | *eksconfig.AddOnHollowNodesRemote.RepositoryName      | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_REPOSITORY_IMAGE_TAG  | read-only "false" | *eksconfig.AddOnHollowNodesRemote.RepositoryImageTag  | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_DEPLOYMENT_REPLICAS   | read-only "false" | *eksconfig.AddOnHollowNodesRemote.DeploymentReplicas  | int32              |
| AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_NODES                 | read-only "false" | *eksconfig.AddOnHollowNodesRemote.Nodes               | int                |
| AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_NODE_NAME_PREFIX      | read-only "false" | *eksconfig.AddOnHollowNodesRemote.NodeNamePrefix      | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_NODE_LABEL_PREFIX     | read-only "false" | *eksconfig.AddOnHollowNodesRemote.NodeLabelPrefix     | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_MAX_OPEN_FILES        | read-only "false" | *eksconfig.AddOnHollowNodesRemote.MaxOpenFiles        | int64              |
| AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_CREATED_NODE_NAMES    | read-only "true"  | *eksconfig.AddOnHollowNodesRemote.CreatedNodeNames    | []string           |
*---------------------------------------------------------------------*-------------------*-------------------------------------------------------*--------------------*


*-------------------------------------------------------------------------------------*-------------------*---------------------------------------------------------------------*--------------------------------*
|                               ENVIRONMENTAL VARIABLE                                |     READ ONLY     |                                TYPE                                 |            GO TYPE             |
*-------------------------------------------------------------------------------------*-------------------*---------------------------------------------------------------------*--------------------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_ENABLE                                     | read-only "false" | *eksconfig.AddOnStresserLocal.Enable                                | bool                           |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_CREATED                                    | read-only "true"  | *eksconfig.AddOnStresserLocal.Created                               | bool                           |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_TIME_FRAME_CREATE                          | read-only "true"  | *eksconfig.AddOnStresserLocal.TimeFrameCreate                       | timeutil.TimeFrame             |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_TIME_FRAME_DELETE                          | read-only "true"  | *eksconfig.AddOnStresserLocal.TimeFrameDelete                       | timeutil.TimeFrame             |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_NAMESPACE                                  | read-only "false" | *eksconfig.AddOnStresserLocal.Namespace                             | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_OBJECT_SIZE                                | read-only "false" | *eksconfig.AddOnStresserLocal.ObjectSize                            | int                            |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_LIST_LIMIT                                 | read-only "false" | *eksconfig.AddOnStresserLocal.ListLimit                             | int64                          |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_DURATION                                   | read-only "false" | *eksconfig.AddOnStresserLocal.Duration                              | time.Duration                  |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_DURATION_STRING                            | read-only "true"  | *eksconfig.AddOnStresserLocal.DurationString                        | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_REQUESTS_WRITES_JSON_PATH                  | read-only "true"  | *eksconfig.AddOnStresserLocal.RequestsWritesJSONPath                | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_REQUESTS_WRITES_SUMMARY                    | read-only "true"  | *eksconfig.AddOnStresserLocal.RequestsWritesSummary                 | metrics.RequestsSummary        |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_REQUESTS_WRITES_SUMMARY_JSON_PATH          | read-only "true"  | *eksconfig.AddOnStresserLocal.RequestsWritesSummaryJSONPath         | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_REQUESTS_WRITES_SUMMARY_TABLE_PATH         | read-only "true"  | *eksconfig.AddOnStresserLocal.RequestsWritesSummaryTablePath        | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_REQUESTS_WRITES_SUMMARY_S3_DIR             | read-only "false" | *eksconfig.AddOnStresserLocal.RequestsWritesSummaryS3Dir            | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_REQUESTS_WRITES_SUMMARY_COMPARE            | read-only "true"  | *eksconfig.AddOnStresserLocal.RequestsWritesSummaryCompare          | metrics.RequestsSummaryCompare |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_REQUESTS_WRITES_SUMMARY_COMPARE_JSON_PATH  | read-only "true"  | *eksconfig.AddOnStresserLocal.RequestsWritesSummaryCompareJSONPath  | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_REQUESTS_WRITES_SUMMARY_COMPARE_TABLE_PATH | read-only "true"  | *eksconfig.AddOnStresserLocal.RequestsWritesSummaryCompareTablePath | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_REQUESTS_READS_JSON_PATH                   | read-only "true"  | *eksconfig.AddOnStresserLocal.RequestsReadsJSONPath                 | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_REQUESTS_READS_SUMMARY                     | read-only "true"  | *eksconfig.AddOnStresserLocal.RequestsReadsSummary                  | metrics.RequestsSummary        |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_REQUESTS_READS_SUMMARY_JSON_PATH           | read-only "true"  | *eksconfig.AddOnStresserLocal.RequestsReadsSummaryJSONPath          | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_REQUESTS_READS_SUMMARY_TABLE_PATH          | read-only "true"  | *eksconfig.AddOnStresserLocal.RequestsReadsSummaryTablePath         | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_REQUESTS_READS_SUMMARY_S3_DIR              | read-only "false" | *eksconfig.AddOnStresserLocal.RequestsReadsSummaryS3Dir             | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_REQUESTS_READS_SUMMARY_COMPARE             | read-only "true"  | *eksconfig.AddOnStresserLocal.RequestsReadsSummaryCompare           | metrics.RequestsSummaryCompare |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_REQUESTS_READS_SUMMARY_COMPARE_JSON_PATH   | read-only "true"  | *eksconfig.AddOnStresserLocal.RequestsReadsSummaryCompareJSONPath   | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_LOCAL_REQUESTS_READS_SUMMARY_COMPARE_TABLE_PATH  | read-only "true"  | *eksconfig.AddOnStresserLocal.RequestsReadsSummaryCompareTablePath  | string                         |
*-------------------------------------------------------------------------------------*-------------------*---------------------------------------------------------------------*--------------------------------*


*--------------------------------------------------------------------------------------*-------------------*----------------------------------------------------------------------*--------------------------------*
|                                ENVIRONMENTAL VARIABLE                                |     READ ONLY     |                                 TYPE                                 |            GO TYPE             |
*--------------------------------------------------------------------------------------*-------------------*----------------------------------------------------------------------*--------------------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_ENABLE                                     | read-only "false" | *eksconfig.AddOnStresserRemote.Enable                                | bool                           |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_CREATED                                    | read-only "true"  | *eksconfig.AddOnStresserRemote.Created                               | bool                           |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_TIME_FRAME_CREATE                          | read-only "true"  | *eksconfig.AddOnStresserRemote.TimeFrameCreate                       | timeutil.TimeFrame             |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_TIME_FRAME_DELETE                          | read-only "true"  | *eksconfig.AddOnStresserRemote.TimeFrameDelete                       | timeutil.TimeFrame             |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_NAMESPACE                                  | read-only "false" | *eksconfig.AddOnStresserRemote.Namespace                             | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REPOSITORY_ACCOUNT_ID                      | read-only "false" | *eksconfig.AddOnStresserRemote.RepositoryAccountID                   | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REPOSITORY_NAME                            | read-only "false" | *eksconfig.AddOnStresserRemote.RepositoryName                        | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REPOSITORY_IMAGE_TAG                       | read-only "false" | *eksconfig.AddOnStresserRemote.RepositoryImageTag                    | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_DEPLOYMENT_REPLICAS                        | read-only "false" | *eksconfig.AddOnStresserRemote.DeploymentReplicas                    | int32                          |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_OBJECT_SIZE                                | read-only "false" | *eksconfig.AddOnStresserRemote.ObjectSize                            | int                            |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_LIST_LIMIT                                 | read-only "false" | *eksconfig.AddOnStresserRemote.ListLimit                             | int64                          |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_DURATION                                   | read-only "false" | *eksconfig.AddOnStresserRemote.Duration                              | time.Duration                  |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_DURATION_STRING                            | read-only "true"  | *eksconfig.AddOnStresserRemote.DurationString                        | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REQUESTS_WRITES_JSON_PATH                  | read-only "true"  | *eksconfig.AddOnStresserRemote.RequestsWritesJSONPath                | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REQUESTS_WRITES_SUMMARY                    | read-only "true"  | *eksconfig.AddOnStresserRemote.RequestsWritesSummary                 | metrics.RequestsSummary        |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REQUESTS_WRITES_SUMMARY_JSON_PATH          | read-only "true"  | *eksconfig.AddOnStresserRemote.RequestsWritesSummaryJSONPath         | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REQUESTS_WRITES_SUMMARY_TABLE_PATH         | read-only "true"  | *eksconfig.AddOnStresserRemote.RequestsWritesSummaryTablePath        | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REQUESTS_WRITES_SUMMARY_S3_DIR             | read-only "false" | *eksconfig.AddOnStresserRemote.RequestsWritesSummaryS3Dir            | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REQUESTS_WRITES_SUMMARY_COMPARE            | read-only "true"  | *eksconfig.AddOnStresserRemote.RequestsWritesSummaryCompare          | metrics.RequestsSummaryCompare |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REQUESTS_WRITES_SUMMARY_COMPARE_JSON_PATH  | read-only "true"  | *eksconfig.AddOnStresserRemote.RequestsWritesSummaryCompareJSONPath  | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REQUESTS_WRITES_SUMMARY_COMPARE_TABLE_PATH | read-only "true"  | *eksconfig.AddOnStresserRemote.RequestsWritesSummaryCompareTablePath | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REQUESTS_READS_JSON_PATH                   | read-only "true"  | *eksconfig.AddOnStresserRemote.RequestsReadsJSONPath                 | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REQUESTS_READS_SUMMARY                     | read-only "true"  | *eksconfig.AddOnStresserRemote.RequestsReadsSummary                  | metrics.RequestsSummary        |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REQUESTS_READS_SUMMARY_JSON_PATH           | read-only "true"  | *eksconfig.AddOnStresserRemote.RequestsReadsSummaryJSONPath          | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REQUESTS_READS_SUMMARY_TABLE_PATH          | read-only "true"  | *eksconfig.AddOnStresserRemote.RequestsReadsSummaryTablePath         | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REQUESTS_READS_SUMMARY_S3_DIR              | read-only "false" | *eksconfig.AddOnStresserRemote.RequestsReadsSummaryS3Dir             | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REQUESTS_READS_SUMMARY_COMPARE             | read-only "true"  | *eksconfig.AddOnStresserRemote.RequestsReadsSummaryCompare           | metrics.RequestsSummaryCompare |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REQUESTS_READS_SUMMARY_COMPARE_JSON_PATH   | read-only "true"  | *eksconfig.AddOnStresserRemote.RequestsReadsSummaryCompareJSONPath   | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REQUESTS_READS_SUMMARY_COMPARE_TABLE_PATH  | read-only "true"  | *eksconfig.AddOnStresserRemote.RequestsReadsSummaryCompareTablePath  | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REQUESTS_WRITES_SUMMARY_OUTPUT_NAME_PREFIX | read-only "false" | *eksconfig.AddOnStresserRemote.RequestsWritesSummaryOutputNamePrefix | string                         |
| AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_REQUESTS_READS_SUMMARY_OUTPUT_NAME_PREFIX  | read-only "false" | *eksconfig.AddOnStresserRemote.RequestsReadsSummaryOutputNamePrefix  | string                         |
*--------------------------------------------------------------------------------------*-------------------*----------------------------------------------------------------------*--------------------------------*


*---------------------------------------------------------------------*-------------------*-------------------------------------------------------*--------------------*
|                       ENVIRONMENTAL VARIABLE                        |     READ ONLY     |                         TYPE                          |      GO TYPE       |
*---------------------------------------------------------------------*-------------------*-------------------------------------------------------*--------------------*
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_VERSION_UPGRADE_ENABLE            | read-only "false" | *eksconfig.AddOnClusterVersionUpgrade.Enable          | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_VERSION_UPGRADE_CREATED           | read-only "true"  | *eksconfig.AddOnClusterVersionUpgrade.Created         | bool               |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_VERSION_UPGRADE_TIME_FRAME_CREATE | read-only "true"  | *eksconfig.AddOnClusterVersionUpgrade.TimeFrameCreate | timeutil.TimeFrame |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_VERSION_UPGRADE_VERSION           | read-only "false" | *eksconfig.AddOnClusterVersionUpgrade.Version         | string             |
| AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_VERSION_UPGRADE_VERSION_VALUE     | read-only "true"  | *eksconfig.AddOnClusterVersionUpgrade.VersionValue    | float64            |
*---------------------------------------------------------------------*-------------------*-------------------------------------------------------*--------------------*


```
