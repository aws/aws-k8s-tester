
```


*----------------------------------------*----------------------------*----------------------------------------*------------------------------*
|         ENVIRONMENTAL VARIABLE         |         FIELD TYPE         |                  TYPE                  |           GO TYPE            |
*----------------------------------------*----------------------------*----------------------------------------*------------------------------*
| K8S_TESTER_CLUSTER_NAME                | SETTABLE VIA ENV VAR       | *k8s_tester.Config.ClusterName         | string                       |
| K8S_TESTER_MINIMUM_NODES               | SETTABLE VIA ENV VAR       | *k8s_tester.Config.MinimumNodes        | int                          |
| K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT     | NON-EMPTY TO ENABLE ADD-ON | *k8s_tester.Config.CloudwatchAgent     | *cloudwatch_agent.Config     |
| K8S_TESTER_ADD_ON_METRICS_SERVER       | NON-EMPTY TO ENABLE ADD-ON | *k8s_tester.Config.MetricsServer       | *metrics_server.Config       |
| K8S_TESTER_ADD_ON_FLUENT_BIT           | NON-EMPTY TO ENABLE ADD-ON | *k8s_tester.Config.FluentBit           | *fluent_bit.Config           |
| K8S_TESTER_ADD_ON_KUBERNETES_DASHBOARD | NON-EMPTY TO ENABLE ADD-ON | *k8s_tester.Config.KubernetesDashboard | *kubernetes_dashboard.Config |
| K8S_TESTER_ADD_ON_NLB_HELLOW_WORLD     | NON-EMPTY TO ENABLE ADD-ON | *k8s_tester.Config.NLBHelloWorld       | *nlb_hello_world.Config      |
| K8S_TESTER_ADD_ON_JOBS_PI              | NON-EMPTY TO ENABLE ADD-ON | *k8s_tester.Config.JobsPi              | *jobs_pi.Config              |
| K8S_TESTER_ADD_ON_JOBS_ECHO            | NON-EMPTY TO ENABLE ADD-ON | *k8s_tester.Config.JobsEcho            | *jobs_echo.Config            |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO       | NON-EMPTY TO ENABLE ADD-ON | *k8s_tester.Config.CronJobsEcho        | *jobs_echo.Config            |
*----------------------------------------*----------------------------*----------------------------------------*------------------------------*


*--------------------------------------------------*----------------------*---------------------------------------*---------*
|              ENVIRONMENTAL VARIABLE              |      FIELD TYPE      |                 TYPE                  | GO TYPE |
*--------------------------------------------------*----------------------*---------------------------------------*---------*
| K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT_REGION        | SETTABLE VIA ENV VAR | *cloudwatch_agent.Config.Region       | string  |
| K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT_CLUSTER_NAME  | SETTABLE VIA ENV VAR | *cloudwatch_agent.Config.ClusterName  | string  |
| K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT_MINIMUM_NODES | SETTABLE VIA ENV VAR | *cloudwatch_agent.Config.MinimumNodes | int     |
| K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT_NAMESPACE     | SETTABLE VIA ENV VAR | *cloudwatch_agent.Config.Namespace    | string  |
*--------------------------------------------------*----------------------*---------------------------------------*---------*


*------------------------------------------------*----------------------*-------------------------------------*---------*
|             ENVIRONMENTAL VARIABLE             |      FIELD TYPE      |                TYPE                 | GO TYPE |
*------------------------------------------------*----------------------*-------------------------------------*---------*
| K8S_TESTER_ADD_ON_METRICS_SERVER_MINIMUM_NODES | SETTABLE VIA ENV VAR | *metrics_server.Config.MinimumNodes | int     |
| K8S_TESTER_ADD_ON_METRICS_SERVER_NAMESPACE     | SETTABLE VIA ENV VAR | *metrics_server.Config.Namespace    | string  |
*------------------------------------------------*----------------------*-------------------------------------*---------*


*--------------------------------------------*----------------------*---------------------------------*---------*
|           ENVIRONMENTAL VARIABLE           |      FIELD TYPE      |              TYPE               | GO TYPE |
*--------------------------------------------*----------------------*---------------------------------*---------*
| K8S_TESTER_ADD_ON_FLUENT_BIT_MINIMUM_NODES | SETTABLE VIA ENV VAR | *fluent_bit.Config.MinimumNodes | int     |
| K8S_TESTER_ADD_ON_FLUENT_BIT_NAMESPACE     | SETTABLE VIA ENV VAR | *fluent_bit.Config.Namespace    | string  |
*--------------------------------------------*----------------------*---------------------------------*---------*


*------------------------------------------------------*----------------------*-------------------------------------------*---------*
|                ENVIRONMENTAL VARIABLE                |      FIELD TYPE      |                   TYPE                    | GO TYPE |
*------------------------------------------------------*----------------------*-------------------------------------------*---------*
| K8S_TESTER_ADD_ON_KUBERNETES_DASHBOARD_MINIMUM_NODES | SETTABLE VIA ENV VAR | *kubernetes_dashboard.Config.MinimumNodes | int     |
*------------------------------------------------------*----------------------*-------------------------------------------*---------*


*------------------------------------------------------------*----------------------*------------------------------------------------*-------------------*
|                   ENVIRONMENTAL VARIABLE                   |      FIELD TYPE      |                      TYPE                      |      GO TYPE      |
*------------------------------------------------------------*----------------------*------------------------------------------------*-------------------*
| K8S_TESTER_ADD_ON_NLB_HELLO_WORLD_ACCOUNT_ID               | READ-ONLY            | *nlb_hello_world.Config.AccountID              | string            |
| K8S_TESTER_ADD_ON_NLB_HELLO_WORLD_PARTITION                | SETTABLE VIA ENV VAR | *nlb_hello_world.Config.Partition              | string            |
| K8S_TESTER_ADD_ON_NLB_HELLO_WORLD_REGION                   | SETTABLE VIA ENV VAR | *nlb_hello_world.Config.Region                 | string            |
| K8S_TESTER_ADD_ON_NLB_HELLO_WORLD_MINIMUM_NODES            | SETTABLE VIA ENV VAR | *nlb_hello_world.Config.MinimumNodes           | int               |
| K8S_TESTER_ADD_ON_NLB_HELLO_WORLD_NAMESPACE                | SETTABLE VIA ENV VAR | *nlb_hello_world.Config.Namespace              | string            |
| K8S_TESTER_ADD_ON_NLB_HELLO_WORLD_DEPLOYMENT_NODE_SELECTOR | SETTABLE VIA ENV VAR | *nlb_hello_world.Config.DeploymentNodeSelector | map[string]string |
| K8S_TESTER_ADD_ON_NLB_HELLO_WORLD_DEPLOYMENT_REPLICAS      | SETTABLE VIA ENV VAR | *nlb_hello_world.Config.DeploymentReplicas     | int32             |
*------------------------------------------------------------*----------------------*------------------------------------------------*-------------------*


*-----------------------------------------*----------------------*------------------------------*---------*
|         ENVIRONMENTAL VARIABLE          |      FIELD TYPE      |             TYPE             | GO TYPE |
*-----------------------------------------*----------------------*------------------------------*---------*
| K8S_TESTER_ADD_ON_JOBS_PI_MINIMUM_NODES | SETTABLE VIA ENV VAR | *jobs_pi.Config.MinimumNodes | int     |
| K8S_TESTER_ADD_ON_JOBS_PI_NAMESPACE     | SETTABLE VIA ENV VAR | *jobs_pi.Config.Namespace    | string  |
| K8S_TESTER_ADD_ON_JOBS_PI_COMPLETES     | SETTABLE VIA ENV VAR | *jobs_pi.Config.Completes    | int32   |
| K8S_TESTER_ADD_ON_JOBS_PI_PARALLELS     | SETTABLE VIA ENV VAR | *jobs_pi.Config.Parallels    | int32   |
*-----------------------------------------*----------------------*------------------------------*---------*


*-----------------------------------------------------------*----------------------*----------------------------------------------*---------*
|                  ENVIRONMENTAL VARIABLE                   |      FIELD TYPE      |                     TYPE                     | GO TYPE |
*-----------------------------------------------------------*----------------------*----------------------------------------------*---------*
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
