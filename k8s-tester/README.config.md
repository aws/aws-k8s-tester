
```


*----------------------------------------*-----------------------------------*----------------------------------------*------------------------------*
|         ENVIRONMENTAL VARIABLE         |             READ ONLY             |                  TYPE                  |           GO TYPE            |
*----------------------------------------*-----------------------------------*----------------------------------------*------------------------------*
| K8S_TESTER_CLUSTER_NAME                | read-only "false", add-on "false" | *k8s_tester.Config.ClusterName         | string                       |
| K8S_TESTER_MINIMUM_NODES               | read-only "false", add-on "false" | *k8s_tester.Config.MinimumNodes        | int                          |
| K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT     | read-only "false", add-on "true"  | *k8s_tester.Config.CloudwatchAgent     | *cloudwatch_agent.Config     |
| K8S_TESTER_ADD_ON_METRICS_SERVER       | read-only "false", add-on "true"  | *k8s_tester.Config.MetricsServer       | *metrics_server.Config       |
| K8S_TESTER_ADD_ON_FLUENT_BIT           | read-only "false", add-on "true"  | *k8s_tester.Config.FluentBit           | *fluent_bit.Config           |
| K8S_TESTER_ADD_ON_KUBERNETES_DASHBOARD | read-only "false", add-on "true"  | *k8s_tester.Config.KubernetesDashboard | *kubernetes_dashboard.Config |
| K8S_TESTER_ADD_ON_NLB_HELLOW_WORLD     | read-only "false", add-on "true"  | *k8s_tester.Config.NLBHelloWorld       | *nlb_hello_world.Config      |
| K8S_TESTER_ADD_ON_JOBS_PI              | read-only "false", add-on "true"  | *k8s_tester.Config.JobsPi              | *jobs_pi.Config              |
| K8S_TESTER_ADD_ON_JOBS_ECHO            | read-only "false", add-on "true"  | *k8s_tester.Config.JobsEcho            | *jobs_echo.Config            |
| K8S_TESTER_ADD_ON_CRON_JOBS_ECHO       | read-only "false", add-on "true"  | *k8s_tester.Config.CronJobsEcho        | *jobs_echo.Config            |
*----------------------------------------*-----------------------------------*----------------------------------------*------------------------------*


*--------------------------------------------------*-----------------------------------*---------------------------------------*---------*
|              ENVIRONMENTAL VARIABLE              |             READ ONLY             |                 TYPE                  | GO TYPE |
*--------------------------------------------------*-----------------------------------*---------------------------------------*---------*
| K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT_REGION        | read-only "false", add-on "false" | *cloudwatch_agent.Config.Region       | string  |
| K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT_CLUSTER_NAME  | read-only "false", add-on "false" | *cloudwatch_agent.Config.ClusterName  | string  |
| K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT_MINIMUM_NODES | read-only "false", add-on "false" | *cloudwatch_agent.Config.MinimumNodes | int     |
| K8S_TESTER_ADD_ON_CLOUDWATCH_AGENT_NAMESPACE     | read-only "false", add-on "false" | *cloudwatch_agent.Config.Namespace    | string  |
*--------------------------------------------------*-----------------------------------*---------------------------------------*---------*


```
