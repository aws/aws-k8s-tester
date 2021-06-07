
```


*----------------------------------*----------------------*----------------------------------------*---------------*
|      ENVIRONMENTAL VARIABLE      |      FIELD TYPE      |                  TYPE                  |    GO TYPE    |
*----------------------------------*----------------------*----------------------------------------*---------------*
| K8S_TESTER_PROMPT                | SETTABLE VIA ENV VAR | *k8s_tester.Config.Prompt              | bool          |
| K8S_TESTER_CLUSTER_NAME          | SETTABLE VIA ENV VAR | *k8s_tester.Config.ClusterName         | string        |
| K8S_TESTER_CONFIG_PATH           | SETTABLE VIA ENV VAR | *k8s_tester.Config.ConfigPath          | string        |
| K8S_TESTER_LOG_COLOR             | SETTABLE VIA ENV VAR | *k8s_tester.Config.LogColor            | bool          |
| K8S_TESTER_LOG_COLOR_OVERRIDE    | SETTABLE VIA ENV VAR | *k8s_tester.Config.LogColorOverride    | string        |
| K8S_TESTER_LOG_LEVEL             | SETTABLE VIA ENV VAR | *k8s_tester.Config.LogLevel            | string        |
| K8S_TESTER_LOG_OUTPUTS           | SETTABLE VIA ENV VAR | *k8s_tester.Config.LogOutputs          | []string      |
| K8S_TESTER_KUBECTL_DOWNLOAD_URL  | SETTABLE VIA ENV VAR | *k8s_tester.Config.KubectlDownloadURL  | string        |
| K8S_TESTER_KUBECTL_PATH          | SETTABLE VIA ENV VAR | *k8s_tester.Config.KubectlPath         | string        |
| K8S_TESTER_KUBECONFIG_PATH       | SETTABLE VIA ENV VAR | *k8s_tester.Config.KubeconfigPath      | string        |
| K8S_TESTER_KUBECONFIG_CONTEXT    | SETTABLE VIA ENV VAR | *k8s_tester.Config.KubeconfigContext   | string        |
| K8S_TESTER_CLIENTS               | SETTABLE VIA ENV VAR | *k8s_tester.Config.Clients             | int           |
| K8S_TESTER_CLIENT_QPS            | SETTABLE VIA ENV VAR | *k8s_tester.Config.ClientQPS           | float32       |
| K8S_TESTER_CLIENT_BURST          | SETTABLE VIA ENV VAR | *k8s_tester.Config.ClientBurst         | int           |
| K8S_TESTER_CLIENT_TIMEOUT        | SETTABLE VIA ENV VAR | *k8s_tester.Config.ClientTimeout       | time.Duration |
| K8S_TESTER_CLIENT_TIMEOUT_STRING | READ-ONLY            | *k8s_tester.Config.ClientTimeoutString | string        |
| K8S_TESTER_MINIMUM_NODES         | SETTABLE VIA ENV VAR | *k8s_tester.Config.MinimumNodes        | int           |
| K8S_TESTER_TOTAL_NODES           | READ-ONLY            | *k8s_tester.Config.TotalNodes          | int           |
*----------------------------------*----------------------*----------------------------------------*---------------*


*--------------------------------------------------*----------------------*---------------------------------------*---------*
|              ENVIRONMENTAL VARIABLE              |      FIELD TYPE      |                 TYPE                  | GO TYPE |
*--------------------------------------------------*----------------------*---------------------------------------*---------*
| K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT_ENABLE        | SETTABLE VIA ENV VAR | *cloudwatch_agent.Config.Enable       | bool    |
| K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT_REGION        | SETTABLE VIA ENV VAR | *cloudwatch_agent.Config.Region       | string  |
| K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT_CLUSTER_NAME  | READ-ONLY            | *cloudwatch_agent.Config.ClusterName  | string  |
| K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT_MINIMUM_NODES | SETTABLE VIA ENV VAR | *cloudwatch_agent.Config.MinimumNodes | int     |
| K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT_NAMESPACE     | SETTABLE VIA ENV VAR | *cloudwatch_agent.Config.Namespace    | string  |
*--------------------------------------------------*----------------------*---------------------------------------*---------*


*--------------------------------------------*----------------------*---------------------------------*---------*
|           ENVIRONMENTAL VARIABLE           |      FIELD TYPE      |              TYPE               | GO TYPE |
*--------------------------------------------*----------------------*---------------------------------*---------*
| K8S_TESTER_ADD_ON_FLUENT_BIT_ENABLE        | SETTABLE VIA ENV VAR | *fluent_bit.Config.Enable       | bool    |
| K8S_TESTER_ADD_ON_FLUENT_BIT_MINIMUM_NODES | SETTABLE VIA ENV VAR | *fluent_bit.Config.MinimumNodes | int     |
| K8S_TESTER_ADD_ON_FLUENT_BIT_NAMESPACE     | SETTABLE VIA ENV VAR | *fluent_bit.Config.Namespace    | string  |
*--------------------------------------------*----------------------*---------------------------------*---------*


*------------------------------------------------*----------------------*-------------------------------------*---------*
|             ENVIRONMENTAL VARIABLE             |      FIELD TYPE      |                TYPE                 | GO TYPE |
*------------------------------------------------*----------------------*-------------------------------------*---------*
| K8S_TESTER_ADD_ON_METRICS_SERVER_ENABLE        | SETTABLE VIA ENV VAR | *metrics_server.Config.Enable       | bool    |
| K8S_TESTER_ADD_ON_METRICS_SERVER_MINIMUM_NODES | SETTABLE VIA ENV VAR | *metrics_server.Config.MinimumNodes | int     |
*------------------------------------------------*----------------------*-------------------------------------*---------*


*-------------------------------------------------------------------*----------------------*-----------------------------------------------------*---------------*
|                      ENVIRONMENTAL VARIABLE                       |      FIELD TYPE      |                        TYPE                         |    GO TYPE    |
*-------------------------------------------------------------------*----------------------*-----------------------------------------------------*---------------*
| K8S_TESTER_ADD_ON_CONFORMANCE_ENABLE                              | SETTABLE VIA ENV VAR | *conformance.Config.Enable                          | bool          |
| K8S_TESTER_ADD_ON_CONFORMANCE_MINIMUM_NODES                       | SETTABLE VIA ENV VAR | *conformance.Config.MinimumNodes                    | int           |
| K8S_TESTER_ADD_ON_CONFORMANCE_NAMESPACE                           | SETTABLE VIA ENV VAR | *conformance.Config.Namespace                       | string        |
| K8S_TESTER_ADD_ON_CONFORMANCE_SONOBUOY_PATH                       | SETTABLE VIA ENV VAR | *conformance.Config.SonobuoyPath                    | string        |
| K8S_TESTER_ADD_ON_CONFORMANCE_SONOBUOY_DOWNLOAD_URL               | SETTABLE VIA ENV VAR | *conformance.Config.SonobuoyDownloadURL             | string        |
| K8S_TESTER_ADD_ON_CONFORMANCE_SONOBUOY_RUN_TIMEOUT                | SETTABLE VIA ENV VAR | *conformance.Config.SonobuoyRunTimeout              | time.Duration |
| K8S_TESTER_ADD_ON_CONFORMANCE_SONOBUOY_RUN_TIMEOUT_STRING         | READ-ONLY            | *conformance.Config.SonobuoyRunTimeoutString        | string        |
| K8S_TESTER_ADD_ON_CONFORMANCE_SONOBUOY_DELETE_TIMEOUT             | SETTABLE VIA ENV VAR | *conformance.Config.SonobuoyDeleteTimeout           | time.Duration |
| K8S_TESTER_ADD_ON_CONFORMANCE_SONOBUOY_DELETE_TIMEOUT_STRING      | READ-ONLY            | *conformance.Config.SonobuoyDeleteTimeoutString     | string        |
| K8S_TESTER_ADD_ON_CONFORMANCE_SONOBUOY_RUN_MODE                   | SETTABLE VIA ENV VAR | *conformance.Config.SonobuoyRunMode                 | string        |
| K8S_TESTER_ADD_ON_CONFORMANCE_SONOBUOY_RUN_E2E_FOCUS              | SETTABLE VIA ENV VAR | *conformance.Config.SonobuoyRunE2EFocus             | string        |
| K8S_TESTER_ADD_ON_CONFORMANCE_SONOBUOY_RUN_E2E_SKIP               | SETTABLE VIA ENV VAR | *conformance.Config.SonobuoyRunE2ESkip              | string        |
| K8S_TESTER_ADD_ON_CONFORMANCE_SONOBUOY_RUN_KUBE_CONFORMANCE_IMAGE | SETTABLE VIA ENV VAR | *conformance.Config.SonobuoyRunKubeConformanceImage | string        |
| K8S_TESTER_ADD_ON_CONFORMANCE_SONOBUOY_RUN_E2E_REPO_CONFIG        | SETTABLE VIA ENV VAR | *conformance.Config.SonobuoyRunE2ERepoConfig        | string        |
| K8S_TESTER_ADD_ON_CONFORMANCE_SONOBUOY_RUN_IMAGE                  | SETTABLE VIA ENV VAR | *conformance.Config.SonobuoyRunImage                | string        |
| K8S_TESTER_ADD_ON_CONFORMANCE_SONOBUOY_RUN_SYSTEMD_LOGS_IMAGE     | SETTABLE VIA ENV VAR | *conformance.Config.SonobuoyRunSystemdLogsImage     | string        |
| K8S_TESTER_ADD_ON_CONFORMANCE_SONOBUOY_RESULTS_TAR_GZ_PATH        | SETTABLE VIA ENV VAR | *conformance.Config.SonobuoyResultsTarGzPath        | string        |
| K8S_TESTER_ADD_ON_CONFORMANCE_SONOBUOY_RESULTS_E2E_LOG_PATH       | SETTABLE VIA ENV VAR | *conformance.Config.SonobuoyResultsE2ELogPath       | string        |
| K8S_TESTER_ADD_ON_CONFORMANCE_SONOBUOY_RESULTS_JUNIT_XML_PATH     | SETTABLE VIA ENV VAR | *conformance.Config.SonobuoyResultsJunitXMLPath     | string        |
| K8S_TESTER_ADD_ON_CONFORMANCE_SONOBUOY_RESULTS_OUTPUT_DIR         | SETTABLE VIA ENV VAR | *conformance.Config.SonobuoyResultsOutputDir        | string        |
*-------------------------------------------------------------------*----------------------*-----------------------------------------------------*---------------*


*-----------------------------------------------*----------------------*----------------------------------*---------*
|            ENVIRONMENTAL VARIABLE             |      FIELD TYPE      |               TYPE               | GO TYPE |
*-----------------------------------------------*----------------------*----------------------------------*---------*
| K8S_TESTER_ADD_ON_CSI_EBS_ENABLE              | SETTABLE VIA ENV VAR | *csi_ebs.Config.Enable           | bool    |
| K8S_TESTER_ADD_ON_CSI_EBS_MINIMUM_NODES       | SETTABLE VIA ENV VAR | *csi_ebs.Config.MinimumNodes     | int     |
| K8S_TESTER_ADD_ON_CSI_EBS_HELM_CHART_REPO_URL | SETTABLE VIA ENV VAR | *csi_ebs.Config.HelmChartRepoURL | string  |
*-----------------------------------------------*----------------------*----------------------------------*---------*


*------------------------------------------------------*----------------------*-------------------------------------------*---------*
|                ENVIRONMENTAL VARIABLE                |      FIELD TYPE      |                   TYPE                    | GO TYPE |
*------------------------------------------------------*----------------------*-------------------------------------------*---------*
| K8S_TESTER_ADD_ON_KUBERNETES_DASHBOARD_ENABLE        | SETTABLE VIA ENV VAR | *kubernetes_dashboard.Config.Enable       | bool    |
| K8S_TESTER_ADD_ON_KUBERNETES_DASHBOARD_MINIMUM_NODES | SETTABLE VIA ENV VAR | *kubernetes_dashboard.Config.MinimumNodes | int     |
*------------------------------------------------------*----------------------*-------------------------------------------*---------*


*-------------------------------------------------------*----------------------*-------------------------------------------*-------------------*
|                ENVIRONMENTAL VARIABLE                 |      FIELD TYPE      |                   TYPE                    |      GO TYPE      |
*-------------------------------------------------------*----------------------*-------------------------------------------*-------------------*
| K8S_TESTER_ADD_ON_PHP_APACHE_ENABLE                   | SETTABLE VIA ENV VAR | *php_apache.Config.Enable                 | bool              |
| K8S_TESTER_ADD_ON_PHP_APACHE_MINIMUM_NODES            | SETTABLE VIA ENV VAR | *php_apache.Config.MinimumNodes           | int               |
| K8S_TESTER_ADD_ON_PHP_APACHE_NAMESPACE                | SETTABLE VIA ENV VAR | *php_apache.Config.Namespace              | string            |
| K8S_TESTER_ADD_ON_PHP_APACHE_DEPLOYMENT_NODE_SELECTOR | SETTABLE VIA ENV VAR | *php_apache.Config.DeploymentNodeSelector | map[string]string |
| K8S_TESTER_ADD_ON_PHP_APACHE_DEPLOYMENT_REPLICAS      | SETTABLE VIA ENV VAR | *php_apache.Config.DeploymentReplicas     | int32             |
*-------------------------------------------------------*----------------------*-------------------------------------------*-------------------*


*----------------------------------------------------*----------------------*---------------------------*---------*
|               ENVIRONMENTAL VARIABLE               |      FIELD TYPE      |           TYPE            | GO TYPE |
*----------------------------------------------------*----------------------*---------------------------*---------*
| K8S_TESTER_ADD_ON_PHP_APACHE_REPOSITORY_PARTITION  | SETTABLE VIA ENV VAR | *ecr.Repository.Partition | string  |
| K8S_TESTER_ADD_ON_PHP_APACHE_REPOSITORY_ACCOUNT_ID | SETTABLE VIA ENV VAR | *ecr.Repository.AccountID | string  |
| K8S_TESTER_ADD_ON_PHP_APACHE_REPOSITORY_REGION     | SETTABLE VIA ENV VAR | *ecr.Repository.Region    | string  |
| K8S_TESTER_ADD_ON_PHP_APACHE_REPOSITORY_NAME       | SETTABLE VIA ENV VAR | *ecr.Repository.Name      | string  |
| K8S_TESTER_ADD_ON_PHP_APACHE_REPOSITORY_IMAGE_TAG  | SETTABLE VIA ENV VAR | *ecr.Repository.ImageTag  | string  |
*----------------------------------------------------*----------------------*---------------------------*---------*


*------------------------------------------------------------*----------------------*------------------------------------------------*-------------------*
|                   ENVIRONMENTAL VARIABLE                   |      FIELD TYPE      |                      TYPE                      |      GO TYPE      |
*------------------------------------------------------------*----------------------*------------------------------------------------*-------------------*
| K8S_TESTER_ADD_ON_NLB_HELLO_WORLD_ENABLE                   | SETTABLE VIA ENV VAR | *nlb_hello_world.Config.Enable                 | bool              |
| K8S_TESTER_ADD_ON_NLB_HELLO_WORLD_ACCOUNT_ID               | READ-ONLY            | *nlb_hello_world.Config.AccountID              | string            |
| K8S_TESTER_ADD_ON_NLB_HELLO_WORLD_PARTITION                | SETTABLE VIA ENV VAR | *nlb_hello_world.Config.Partition              | string            |
| K8S_TESTER_ADD_ON_NLB_HELLO_WORLD_REGION                   | SETTABLE VIA ENV VAR | *nlb_hello_world.Config.Region                 | string            |
| K8S_TESTER_ADD_ON_NLB_HELLO_WORLD_MINIMUM_NODES            | SETTABLE VIA ENV VAR | *nlb_hello_world.Config.MinimumNodes           | int               |
| K8S_TESTER_ADD_ON_NLB_HELLO_WORLD_NAMESPACE                | SETTABLE VIA ENV VAR | *nlb_hello_world.Config.Namespace              | string            |
| K8S_TESTER_ADD_ON_NLB_HELLO_WORLD_DEPLOYMENT_NODE_SELECTOR | SETTABLE VIA ENV VAR | *nlb_hello_world.Config.DeploymentNodeSelector | map[string]string |
| K8S_TESTER_ADD_ON_NLB_HELLO_WORLD_DEPLOYMENT_REPLICAS      | SETTABLE VIA ENV VAR | *nlb_hello_world.Config.DeploymentReplicas     | int32             |
| K8S_TESTER_ADD_ON_NLB_HELLO_WORLD_ELB_ARN                  | READ-ONLY            | *nlb_hello_world.Config.ELBARN                 | string            |
| K8S_TESTER_ADD_ON_NLB_HELLO_WORLD_ELB_NAME                 | READ-ONLY            | *nlb_hello_world.Config.ELBName                | string            |
| K8S_TESTER_ADD_ON_NLB_HELLO_WORLD_ELB_URL                  | READ-ONLY            | *nlb_hello_world.Config.ELBURL                 | string            |
*------------------------------------------------------------*----------------------*------------------------------------------------*-------------------*


*-----------------------------------------*----------------------*------------------------------*---------*
|         ENVIRONMENTAL VARIABLE          |      FIELD TYPE      |             TYPE             | GO TYPE |
*-----------------------------------------*----------------------*------------------------------*---------*
| K8S_TESTER_ADD_ON_JOBS_PI_ENABLE        | SETTABLE VIA ENV VAR | *jobs_pi.Config.Enable       | bool    |
| K8S_TESTER_ADD_ON_JOBS_PI_MINIMUM_NODES | SETTABLE VIA ENV VAR | *jobs_pi.Config.MinimumNodes | int     |
| K8S_TESTER_ADD_ON_JOBS_PI_NAMESPACE     | SETTABLE VIA ENV VAR | *jobs_pi.Config.Namespace    | string  |
| K8S_TESTER_ADD_ON_JOBS_PI_COMPLETES     | SETTABLE VIA ENV VAR | *jobs_pi.Config.Completes    | int32   |
| K8S_TESTER_ADD_ON_JOBS_PI_PARALLELS     | SETTABLE VIA ENV VAR | *jobs_pi.Config.Parallels    | int32   |
*-----------------------------------------*----------------------*------------------------------*---------*


*-----------------------------------------------------------*----------------------*----------------------------------------------*---------*
|                  ENVIRONMENTAL VARIABLE                   |      FIELD TYPE      |                     TYPE                     | GO TYPE |
*-----------------------------------------------------------*----------------------*----------------------------------------------*---------*
| K8S_TESTER_ADD_ON_JOBS_ECHO_ENABLE                        | SETTABLE VIA ENV VAR | *jobs_echo.Config.Enable                     | bool    |
| K8S_TESTER_ADD_ON_JOBS_ECHO_MINIMUM_NODES                 | SETTABLE VIA ENV VAR | *jobs_echo.Config.MinimumNodes               | int     |
| K8S_TESTER_ADD_ON_JOBS_ECHO_NAMESPACE                     | SETTABLE VIA ENV VAR | *jobs_echo.Config.Namespace                  | string  |
| K8S_TESTER_ADD_ON_JOBS_ECHO_JOB_TYPE                      | SETTABLE VIA ENV VAR | *jobs_echo.Config.JobType                    | string  |
| K8S_TESTER_ADD_ON_JOBS_ECHO_COMPLETES                     | SETTABLE VIA ENV VAR | *jobs_echo.Config.Completes                  | int32   |
| K8S_TESTER_ADD_ON_JOBS_ECHO_PARALLELS                     | SETTABLE VIA ENV VAR | *jobs_echo.Config.Parallels                  | int32   |
| K8S_TESTER_ADD_ON_JOBS_ECHO_ECHO_SIZE                     | SETTABLE VIA ENV VAR | *jobs_echo.Config.EchoSize                   | int32   |
| K8S_TESTER_ADD_ON_JOBS_ECHO_SCHEDULE                      | SETTABLE VIA ENV VAR | *jobs_echo.Config.Schedule                   | string  |
| K8S_TESTER_ADD_ON_JOBS_ECHO_SUCCESSFUL_JOBS_HISTORY_LIMIT | SETTABLE VIA ENV VAR | *jobs_echo.Config.SuccessfulJobsHistoryLimit | int32   |
| K8S_TESTER_ADD_ON_JOBS_ECHO_FAILED_JOBS_HISTORY_LIMIT     | SETTABLE VIA ENV VAR | *jobs_echo.Config.FailedJobsHistoryLimit     | int32   |
*-----------------------------------------------------------*----------------------*----------------------------------------------*---------*


*---------------------------------------------------*----------------------*---------------------------*---------*
|              ENVIRONMENTAL VARIABLE               |      FIELD TYPE      |           TYPE            | GO TYPE |
*---------------------------------------------------*----------------------*---------------------------*---------*
| K8S_TESTER_ADD_ON_JOBS_ECHO_REPOSITORY_PARTITION  | SETTABLE VIA ENV VAR | *ecr.Repository.Partition | string  |
| K8S_TESTER_ADD_ON_JOBS_ECHO_REPOSITORY_ACCOUNT_ID | SETTABLE VIA ENV VAR | *ecr.Repository.AccountID | string  |
| K8S_TESTER_ADD_ON_JOBS_ECHO_REPOSITORY_REGION     | SETTABLE VIA ENV VAR | *ecr.Repository.Region    | string  |
| K8S_TESTER_ADD_ON_JOBS_ECHO_REPOSITORY_NAME       | SETTABLE VIA ENV VAR | *ecr.Repository.Name      | string  |
| K8S_TESTER_ADD_ON_JOBS_ECHO_REPOSITORY_IMAGE_TAG  | SETTABLE VIA ENV VAR | *ecr.Repository.ImageTag  | string  |
*---------------------------------------------------*----------------------*---------------------------*---------*


*----------------------------------------------------------------*----------------------*----------------------------------------------*---------*
|                     ENVIRONMENTAL VARIABLE                     |      FIELD TYPE      |                     TYPE                     | GO TYPE |
*----------------------------------------------------------------*----------------------*----------------------------------------------*---------*
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_ENABLE                        | SETTABLE VIA ENV VAR | *jobs_echo.Config.Enable                     | bool    |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_MINIMUM_NODES                 | SETTABLE VIA ENV VAR | *jobs_echo.Config.MinimumNodes               | int     |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_NAMESPACE                     | SETTABLE VIA ENV VAR | *jobs_echo.Config.Namespace                  | string  |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_JOB_TYPE                      | SETTABLE VIA ENV VAR | *jobs_echo.Config.JobType                    | string  |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_COMPLETES                     | SETTABLE VIA ENV VAR | *jobs_echo.Config.Completes                  | int32   |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_PARALLELS                     | SETTABLE VIA ENV VAR | *jobs_echo.Config.Parallels                  | int32   |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_ECHO_SIZE                     | SETTABLE VIA ENV VAR | *jobs_echo.Config.EchoSize                   | int32   |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_SCHEDULE                      | SETTABLE VIA ENV VAR | *jobs_echo.Config.Schedule                   | string  |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_SUCCESSFUL_JOBS_HISTORY_LIMIT | SETTABLE VIA ENV VAR | *jobs_echo.Config.SuccessfulJobsHistoryLimit | int32   |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_FAILED_JOBS_HISTORY_LIMIT     | SETTABLE VIA ENV VAR | *jobs_echo.Config.FailedJobsHistoryLimit     | int32   |
*----------------------------------------------------------------*----------------------*----------------------------------------------*---------*


*--------------------------------------------------------*----------------------*---------------------------*---------*
|                 ENVIRONMENTAL VARIABLE                 |      FIELD TYPE      |           TYPE            | GO TYPE |
*--------------------------------------------------------*----------------------*---------------------------*---------*
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_REPOSITORY_PARTITION  | SETTABLE VIA ENV VAR | *ecr.Repository.Partition | string  |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_REPOSITORY_ACCOUNT_ID | SETTABLE VIA ENV VAR | *ecr.Repository.AccountID | string  |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_REPOSITORY_REGION     | SETTABLE VIA ENV VAR | *ecr.Repository.Region    | string  |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_REPOSITORY_NAME       | SETTABLE VIA ENV VAR | *ecr.Repository.Name      | string  |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_REPOSITORY_IMAGE_TAG  | SETTABLE VIA ENV VAR | *ecr.Repository.ImageTag  | string  |
*--------------------------------------------------------*----------------------*---------------------------*---------*


*-------------------------------------------------------*----------------------*------------------------------------------*-----------------*
|                ENVIRONMENTAL VARIABLE                 |      FIELD TYPE      |                   TYPE                   |     GO TYPE     |
*-------------------------------------------------------*----------------------*------------------------------------------*-----------------*
| K8S_TESTER_ADD_ON_CSRS_ENABLE                         | SETTABLE VIA ENV VAR | *csrs.Config.Enable                      | bool            |
| K8S_TESTER_ADD_ON_CSRS_MINIMUM_NODES                  | SETTABLE VIA ENV VAR | *csrs.Config.MinimumNodes                | int             |
| K8S_TESTER_ADD_ON_CSRS_OBJECTS                        | SETTABLE VIA ENV VAR | *csrs.Config.Objects                     | int             |
| K8S_TESTER_ADD_ON_CSRS_INITIAL_REQUEST_CONDITION_TYPE | SETTABLE VIA ENV VAR | *csrs.Config.InitialRequestConditionType | string          |
| K8S_TESTER_ADD_ON_CSRS_LATENCY_SUMMARY                | READ-ONLY            | *csrs.Config.LatencySummary              | latency.Summary |
*-------------------------------------------------------*----------------------*------------------------------------------*-----------------*


*----------------------------------------------*----------------------*-----------------------------------*-----------------*
|            ENVIRONMENTAL VARIABLE            |      FIELD TYPE      |               TYPE                |     GO TYPE     |
*----------------------------------------------*----------------------*-----------------------------------*-----------------*
| K8S_TESTER_ADD_ON_CONFIGMAPS_ENABLE          | SETTABLE VIA ENV VAR | *configmaps.Config.Enable         | bool            |
| K8S_TESTER_ADD_ON_CONFIGMAPS_MINIMUM_NODES   | SETTABLE VIA ENV VAR | *configmaps.Config.MinimumNodes   | int             |
| K8S_TESTER_ADD_ON_CONFIGMAPS_NAMESPACE       | SETTABLE VIA ENV VAR | *configmaps.Config.Namespace      | string          |
| K8S_TESTER_ADD_ON_CONFIGMAPS_OBJECTS         | SETTABLE VIA ENV VAR | *configmaps.Config.Objects        | int             |
| K8S_TESTER_ADD_ON_CONFIGMAPS_OBJECT_SIZE     | SETTABLE VIA ENV VAR | *configmaps.Config.ObjectSize     | int             |
| K8S_TESTER_ADD_ON_CONFIGMAPS_LATENCY_SUMMARY | READ-ONLY            | *configmaps.Config.LatencySummary | latency.Summary |
*----------------------------------------------*----------------------*-----------------------------------*-----------------*


*-------------------------------------------*----------------------*--------------------------------*-----------------*
|          ENVIRONMENTAL VARIABLE           |      FIELD TYPE      |              TYPE              |     GO TYPE     |
*-------------------------------------------*----------------------*--------------------------------*-----------------*
| K8S_TESTER_ADD_ON_SECRETS_ENABLE          | SETTABLE VIA ENV VAR | *secrets.Config.Enable         | bool            |
| K8S_TESTER_ADD_ON_SECRETS_MINIMUM_NODES   | SETTABLE VIA ENV VAR | *secrets.Config.MinimumNodes   | int             |
| K8S_TESTER_ADD_ON_SECRETS_NAMESPACE       | SETTABLE VIA ENV VAR | *secrets.Config.Namespace      | string          |
| K8S_TESTER_ADD_ON_SECRETS_OBJECTS         | SETTABLE VIA ENV VAR | *secrets.Config.Objects        | int             |
| K8S_TESTER_ADD_ON_SECRETS_OBJECT_SIZE     | SETTABLE VIA ENV VAR | *secrets.Config.ObjectSize     | int             |
| K8S_TESTER_ADD_ON_SECRETS_LATENCY_SUMMARY | READ-ONLY            | *secrets.Config.LatencySummary | latency.Summary |
*-------------------------------------------*----------------------*--------------------------------*-----------------*


*-------------------------------------------------------------*----------------------*------------------------------------------------*------------------------*
|                   ENVIRONMENTAL VARIABLE                    |      FIELD TYPE      |                      TYPE                      |        GO TYPE         |
*-------------------------------------------------------------*----------------------*------------------------------------------------*------------------------*
| K8S_TESTER_ADD_ON_CLUSTERLOADER_ENABLE                      | SETTABLE VIA ENV VAR | *clusterloader.Config.Enable                   | bool                   |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_MINIMUM_NODES               | SETTABLE VIA ENV VAR | *clusterloader.Config.MinimumNodes             | int                    |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_CLUSTERLOADER_PATH          | SETTABLE VIA ENV VAR | *clusterloader.Config.ClusterloaderPath        | string                 |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_CLUSTERLOADER_DOWNLOAD_URL  | SETTABLE VIA ENV VAR | *clusterloader.Config.ClusterloaderDownloadURL | string                 |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_PROVIDER                    | SETTABLE VIA ENV VAR | *clusterloader.Config.Provider                 | string                 |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_RUNS                        | SETTABLE VIA ENV VAR | *clusterloader.Config.Runs                     | int                    |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_RUN_TIMEOUT                 | SETTABLE VIA ENV VAR | *clusterloader.Config.RunTimeout               | time.Duration          |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_RUN_TIMEOUT_STRING          | READ-ONLY            | *clusterloader.Config.RunTimeoutString         | string                 |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_TEST_CONFIG_PATH            | SETTABLE VIA ENV VAR | *clusterloader.Config.TestConfigPath           | string                 |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_RUN_FROM_CLUSTER            | SETTABLE VIA ENV VAR | *clusterloader.Config.RunFromCluster           | bool                   |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_NODES                       | SETTABLE VIA ENV VAR | *clusterloader.Config.Nodes                    | int                    |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_TEST_REPORT_DIR             | READ-ONLY            | *clusterloader.Config.TestReportDir            | string                 |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_TEST_REPORT_DIR_TAR_GZ_PATH | READ-ONLY            | *clusterloader.Config.TestReportDirTarGzPath   | string                 |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_TEST_LOG_PATH               | READ-ONLY            | *clusterloader.Config.TestLogPath              | string                 |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_POD_STARTUP_LATENCY         | READ-ONLY            | *clusterloader.Config.PodStartupLatency        | clusterloader.PerfData |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_POD_STARTUP_LATENCY_PATH    | READ-ONLY            | *clusterloader.Config.PodStartupLatencyPath    | string                 |
*-------------------------------------------------------------*----------------------*------------------------------------------------*------------------------*


*----------------------------------------------------------------------------------*----------------------*-------------------------------------------------------------*---------*
|                              ENVIRONMENTAL VARIABLE                              |      FIELD TYPE      |                            TYPE                             | GO TYPE |
*----------------------------------------------------------------------------------*----------------------*-------------------------------------------------------------*---------*
| K8S_TESTER_ADD_ON_CLUSTERLOADER_TEST_OVERRIDE_PATH                               | READ-ONLY            | *clusterloader.TestOverride.Path                            | string  |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_TEST_OVERRIDE_NODES_PER_NAMESPACE                | SETTABLE VIA ENV VAR | *clusterloader.TestOverride.NodesPerNamespace               | int     |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_TEST_OVERRIDE_PODS_PER_NODE                      | SETTABLE VIA ENV VAR | *clusterloader.TestOverride.PodsPerNode                     | int     |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_TEST_OVERRIDE_BIG_GROUP_SIZE                     | SETTABLE VIA ENV VAR | *clusterloader.TestOverride.BigGroupSize                    | int     |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_TEST_OVERRIDE_MEDIUM_GROUP_SIZE                  | SETTABLE VIA ENV VAR | *clusterloader.TestOverride.MediumGroupSize                 | int     |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_TEST_OVERRIDE_SMALL_GROUP_SIZE                   | SETTABLE VIA ENV VAR | *clusterloader.TestOverride.SmallGroupSize                  | int     |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_TEST_OVERRIDE_SMALL_STATEFUL_SETS_PER_NAMESPACE  | SETTABLE VIA ENV VAR | *clusterloader.TestOverride.SmallStatefulSetsPerNamespace   | int     |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_TEST_OVERRIDE_MEDIUM_STATEFUL_SETS_PER_NAMESPACE | SETTABLE VIA ENV VAR | *clusterloader.TestOverride.MediumStatefulSetsPerNamespace  | int     |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_TEST_OVERRIDE_CL2_USE_HOST_NETWORK_PODS          | SETTABLE VIA ENV VAR | *clusterloader.TestOverride.CL2UseHostNetworkPods           | bool    |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_TEST_OVERRIDE_CL2_LOAD_TEST_THROUGHPUT           | SETTABLE VIA ENV VAR | *clusterloader.TestOverride.CL2LoadTestThroughput           | int     |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_TEST_OVERRIDE_CL2_ENABLE_PVS                     | SETTABLE VIA ENV VAR | *clusterloader.TestOverride.CL2EnablePVS                    | bool    |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_TEST_OVERRIDE_CL2_SCHEDULER_THROUGHPUT_THRESHOLD | SETTABLE VIA ENV VAR | *clusterloader.TestOverride.CL2SchedulerThroughputThreshold | int     |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_TEST_OVERRIDE_PROMETHEUS_SCRAPE_KUBE_PROXY       | SETTABLE VIA ENV VAR | *clusterloader.TestOverride.PrometheusScrapeKubeProxy       | bool    |
| K8S_TESTER_ADD_ON_CLUSTERLOADER_TEST_OVERRIDE_ENABLE_SYSTEM_POD_METRICS          | SETTABLE VIA ENV VAR | *clusterloader.TestOverride.EnableSystemPodMetrics          | bool    |
*----------------------------------------------------------------------------------*----------------------*-------------------------------------------------------------*---------*


*-----------------------------------------------------*----------------------*----------------------------------------*-----------------*
|               ENVIRONMENTAL VARIABLE                |      FIELD TYPE      |                  TYPE                  |     GO TYPE     |
*-----------------------------------------------------*----------------------*----------------------------------------*-----------------*
| K8S_TESTER_ADD_ON_STRESS_ENABLE                     | SETTABLE VIA ENV VAR | *stress.Config.Enable                  | bool            |
| K8S_TESTER_ADD_ON_STRESS_MINIMUM_NODES              | SETTABLE VIA ENV VAR | *stress.Config.MinimumNodes            | int             |
| K8S_TESTER_ADD_ON_STRESS_NAMESPACE                  | SETTABLE VIA ENV VAR | *stress.Config.Namespace               | string          |
| K8S_TESTER_ADD_ON_STRESS_RUN_TIMEOUT                | SETTABLE VIA ENV VAR | *stress.Config.RunTimeout              | time.Duration   |
| K8S_TESTER_ADD_ON_STRESS_RUN_TIMEOUT_STRING         | READ-ONLY            | *stress.Config.RunTimeoutString        | string          |
| K8S_TESTER_ADD_ON_STRESS_OBJECT_KEY_PREFIX          | SETTABLE VIA ENV VAR | *stress.Config.ObjectKeyPrefix         | string          |
| K8S_TESTER_ADD_ON_STRESS_OBJECTS                    | SETTABLE VIA ENV VAR | *stress.Config.Objects                 | int             |
| K8S_TESTER_ADD_ON_STRESS_OBJECT_SIZE                | SETTABLE VIA ENV VAR | *stress.Config.ObjectSize              | int             |
| K8S_TESTER_ADD_ON_STRESS_UPDATE_CONCURRENCY         | SETTABLE VIA ENV VAR | *stress.Config.UpdateConcurrency       | int             |
| K8S_TESTER_ADD_ON_STRESS_LIST_LIMIT                 | SETTABLE VIA ENV VAR | *stress.Config.ListLimit               | int64           |
| K8S_TESTER_ADD_ON_STRESS_LATENCY_SUMMARY_WRITES     | READ-ONLY            | *stress.Config.LatencySummaryWrites    | latency.Summary |
| K8S_TESTER_ADD_ON_STRESS_LATENCY_SUMMARY_GETS       | READ-ONLY            | *stress.Config.LatencySummaryGets      | latency.Summary |
| K8S_TESTER_ADD_ON_STRESS_LATENCY_SUMMARY_RANGE_GETS | READ-ONLY            | *stress.Config.LatencySummaryRangeGets | latency.Summary |
*-----------------------------------------------------*----------------------*----------------------------------------*-----------------*


*------------------------------------------------*----------------------*---------------------------*---------*
|             ENVIRONMENTAL VARIABLE             |      FIELD TYPE      |           TYPE            | GO TYPE |
*------------------------------------------------*----------------------*---------------------------*---------*
| K8S_TESTER_ADD_ON_STRESS_REPOSITORY_PARTITION  | SETTABLE VIA ENV VAR | *ecr.Repository.Partition | string  |
| K8S_TESTER_ADD_ON_STRESS_REPOSITORY_ACCOUNT_ID | SETTABLE VIA ENV VAR | *ecr.Repository.AccountID | string  |
| K8S_TESTER_ADD_ON_STRESS_REPOSITORY_REGION     | SETTABLE VIA ENV VAR | *ecr.Repository.Region    | string  |
| K8S_TESTER_ADD_ON_STRESS_REPOSITORY_NAME       | SETTABLE VIA ENV VAR | *ecr.Repository.Name      | string  |
| K8S_TESTER_ADD_ON_STRESS_REPOSITORY_IMAGE_TAG  | SETTABLE VIA ENV VAR | *ecr.Repository.ImageTag  | string  |
*------------------------------------------------*----------------------*---------------------------*---------*


```
