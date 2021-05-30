
```


*---------------------------------*----------------------*---------------------------------------*----------*
|     ENVIRONMENTAL VARIABLE      |      FIELD TYPE      |                 TYPE                  | GO TYPE  |
*---------------------------------*----------------------*---------------------------------------*----------*
| K8S_TESTER_PROMPT               | SETTABLE VIA ENV VAR | *k8s_tester.Config.Prompt             | bool     |
| K8S_TESTER_CLUSTER_NAME         | SETTABLE VIA ENV VAR | *k8s_tester.Config.ClusterName        | string   |
| K8S_TESTER_CONFIG_PATH          | SETTABLE VIA ENV VAR | *k8s_tester.Config.ConfigPath         | string   |
| K8S_TESTER_LOG_COLOR            | SETTABLE VIA ENV VAR | *k8s_tester.Config.LogColor           | bool     |
| K8S_TESTER_LOG_COLOR_OVERRIDE   | SETTABLE VIA ENV VAR | *k8s_tester.Config.LogColorOverride   | string   |
| K8S_TESTER_LOG_LEVEL            | SETTABLE VIA ENV VAR | *k8s_tester.Config.LogLevel           | string   |
| K8S_TESTER_LOG_OUTPUTS          | SETTABLE VIA ENV VAR | *k8s_tester.Config.LogOutputs         | []string |
| K8S_TESTER_KUBECTL_DOWNLOAD_URL | SETTABLE VIA ENV VAR | *k8s_tester.Config.KubectlDownloadURL | string   |
| K8S_TESTER_KUBECTL_PATH         | SETTABLE VIA ENV VAR | *k8s_tester.Config.KubectlPath        | string   |
| K8S_TESTER_KUBECONFIG_PATH      | SETTABLE VIA ENV VAR | *k8s_tester.Config.KubeconfigPath     | string   |
| K8S_TESTER_KUBECONFIG_CONTEXT   | SETTABLE VIA ENV VAR | *k8s_tester.Config.KubeconfigContext  | string   |
| K8S_TESTER_MINIMUM_NODES        | SETTABLE VIA ENV VAR | *k8s_tester.Config.MinimumNodes       | int      |
| K8S_TESTER_TOTAL_NODES          | READ-ONLY            | *k8s_tester.Config.TotalNodes         | int      |
*---------------------------------*----------------------*---------------------------------------*----------*


*--------------------------------------------------*----------------------*---------------------------------------*---------*
|              ENVIRONMENTAL VARIABLE              |      FIELD TYPE      |                 TYPE                  | GO TYPE |
*--------------------------------------------------*----------------------*---------------------------------------*---------*
| K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT_ENABLE        | SETTABLE VIA ENV VAR | *cloudwatch_agent.Config.Enable       | bool    |
| K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT_REGION        | SETTABLE VIA ENV VAR | *cloudwatch_agent.Config.Region       | string  |
| K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT_CLUSTER_NAME  | SETTABLE VIA ENV VAR | *cloudwatch_agent.Config.ClusterName  | string  |
| K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT_MINIMUM_NODES | SETTABLE VIA ENV VAR | *cloudwatch_agent.Config.MinimumNodes | int     |
| K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT_NAMESPACE     | SETTABLE VIA ENV VAR | *cloudwatch_agent.Config.Namespace    | string  |
*--------------------------------------------------*----------------------*---------------------------------------*---------*


*------------------------------------------------*----------------------*-------------------------------------*---------*
|             ENVIRONMENTAL VARIABLE             |      FIELD TYPE      |                TYPE                 | GO TYPE |
*------------------------------------------------*----------------------*-------------------------------------*---------*
| K8S_TESTER_ADD_ON_METRICS_SERVER_ENABLE        | SETTABLE VIA ENV VAR | *metrics_server.Config.Enable       | bool    |
| K8S_TESTER_ADD_ON_METRICS_SERVER_MINIMUM_NODES | SETTABLE VIA ENV VAR | *metrics_server.Config.MinimumNodes | int     |
| K8S_TESTER_ADD_ON_METRICS_SERVER_NAMESPACE     | SETTABLE VIA ENV VAR | *metrics_server.Config.Namespace    | string  |
*------------------------------------------------*----------------------*-------------------------------------*---------*


*--------------------------------------------*----------------------*---------------------------------*---------*
|           ENVIRONMENTAL VARIABLE           |      FIELD TYPE      |              TYPE               | GO TYPE |
*--------------------------------------------*----------------------*---------------------------------*---------*
| K8S_TESTER_ADD_ON_FLUENT_BIT_ENABLE        | SETTABLE VIA ENV VAR | *fluent_bit.Config.Enable       | bool    |
| K8S_TESTER_ADD_ON_FLUENT_BIT_MINIMUM_NODES | SETTABLE VIA ENV VAR | *fluent_bit.Config.MinimumNodes | int     |
| K8S_TESTER_ADD_ON_FLUENT_BIT_NAMESPACE     | SETTABLE VIA ENV VAR | *fluent_bit.Config.Namespace    | string  |
*--------------------------------------------*----------------------*---------------------------------*---------*


*------------------------------------------------------*----------------------*-------------------------------------------*---------*
|                ENVIRONMENTAL VARIABLE                |      FIELD TYPE      |                   TYPE                    | GO TYPE |
*------------------------------------------------------*----------------------*-------------------------------------------*---------*
| K8S_TESTER_ADD_ON_KUBERNETES_DASHBOARD_ENABLE        | SETTABLE VIA ENV VAR | *kubernetes_dashboard.Config.Enable       | bool    |
| K8S_TESTER_ADD_ON_KUBERNETES_DASHBOARD_MINIMUM_NODES | SETTABLE VIA ENV VAR | *kubernetes_dashboard.Config.MinimumNodes | int     |
*------------------------------------------------------*----------------------*-------------------------------------------*---------*


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
| K8S_TESTER_ADD_ON_JOBS_ECHO_REPOSITORY_BUSYBOX_PARTITION  | SETTABLE VIA ENV VAR | *jobs_echo.Config.RepositoryBusyboxPartition | string  |
| K8S_TESTER_ADD_ON_JOBS_ECHO_REPOSITORY_BUSYBOX_ACCOUNT_ID | SETTABLE VIA ENV VAR | *jobs_echo.Config.RepositoryBusyboxAccountID | string  |
| K8S_TESTER_ADD_ON_JOBS_ECHO_REPOSITORY_BUSYBOX_REGION     | SETTABLE VIA ENV VAR | *jobs_echo.Config.RepositoryBusyboxRegion    | string  |
| K8S_TESTER_ADD_ON_JOBS_ECHO_REPOSITORY_BUSYBOX_NAME       | SETTABLE VIA ENV VAR | *jobs_echo.Config.RepositoryBusyboxName      | string  |
| K8S_TESTER_ADD_ON_JOBS_ECHO_REPOSITORY_BUSYBOX_IMAGE_TAG  | SETTABLE VIA ENV VAR | *jobs_echo.Config.RepositoryBusyboxImageTag  | string  |
| K8S_TESTER_ADD_ON_JOBS_ECHO_JOB_TYPE                      | SETTABLE VIA ENV VAR | *jobs_echo.Config.JobType                    | string  |
| K8S_TESTER_ADD_ON_JOBS_ECHO_COMPLETES                     | SETTABLE VIA ENV VAR | *jobs_echo.Config.Completes                  | int32   |
| K8S_TESTER_ADD_ON_JOBS_ECHO_PARALLELS                     | SETTABLE VIA ENV VAR | *jobs_echo.Config.Parallels                  | int32   |
| K8S_TESTER_ADD_ON_JOBS_ECHO_ECHO_SIZE                     | SETTABLE VIA ENV VAR | *jobs_echo.Config.EchoSize                   | int32   |
| K8S_TESTER_ADD_ON_JOBS_ECHO_SCHEDULE                      | SETTABLE VIA ENV VAR | *jobs_echo.Config.Schedule                   | string  |
| K8S_TESTER_ADD_ON_JOBS_ECHO_SUCCESSFUL_JOBS_HISTORY_LIMIT | SETTABLE VIA ENV VAR | *jobs_echo.Config.SuccessfulJobsHistoryLimit | int32   |
| K8S_TESTER_ADD_ON_JOBS_ECHO_FAILED_JOBS_HISTORY_LIMIT     | SETTABLE VIA ENV VAR | *jobs_echo.Config.FailedJobsHistoryLimit     | int32   |
*-----------------------------------------------------------*----------------------*----------------------------------------------*---------*


*----------------------------------------------------------------*----------------------*----------------------------------------------*---------*
|                     ENVIRONMENTAL VARIABLE                     |      FIELD TYPE      |                     TYPE                     | GO TYPE |
*----------------------------------------------------------------*----------------------*----------------------------------------------*---------*
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_ENABLE                        | SETTABLE VIA ENV VAR | *jobs_echo.Config.Enable                     | bool    |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_MINIMUM_NODES                 | SETTABLE VIA ENV VAR | *jobs_echo.Config.MinimumNodes               | int     |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_NAMESPACE                     | SETTABLE VIA ENV VAR | *jobs_echo.Config.Namespace                  | string  |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_REPOSITORY_BUSYBOX_PARTITION  | SETTABLE VIA ENV VAR | *jobs_echo.Config.RepositoryBusyboxPartition | string  |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_REPOSITORY_BUSYBOX_ACCOUNT_ID | SETTABLE VIA ENV VAR | *jobs_echo.Config.RepositoryBusyboxAccountID | string  |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_REPOSITORY_BUSYBOX_REGION     | SETTABLE VIA ENV VAR | *jobs_echo.Config.RepositoryBusyboxRegion    | string  |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_REPOSITORY_BUSYBOX_NAME       | SETTABLE VIA ENV VAR | *jobs_echo.Config.RepositoryBusyboxName      | string  |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_REPOSITORY_BUSYBOX_IMAGE_TAG  | SETTABLE VIA ENV VAR | *jobs_echo.Config.RepositoryBusyboxImageTag  | string  |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_JOB_TYPE                      | SETTABLE VIA ENV VAR | *jobs_echo.Config.JobType                    | string  |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_COMPLETES                     | SETTABLE VIA ENV VAR | *jobs_echo.Config.Completes                  | int32   |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_PARALLELS                     | SETTABLE VIA ENV VAR | *jobs_echo.Config.Parallels                  | int32   |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_ECHO_SIZE                     | SETTABLE VIA ENV VAR | *jobs_echo.Config.EchoSize                   | int32   |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_SCHEDULE                      | SETTABLE VIA ENV VAR | *jobs_echo.Config.Schedule                   | string  |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_SUCCESSFUL_JOBS_HISTORY_LIMIT | SETTABLE VIA ENV VAR | *jobs_echo.Config.SuccessfulJobsHistoryLimit | int32   |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO_FAILED_JOBS_HISTORY_LIMIT     | SETTABLE VIA ENV VAR | *jobs_echo.Config.FailedJobsHistoryLimit     | int32   |
*----------------------------------------------------------------*----------------------*----------------------------------------------*---------*


```
