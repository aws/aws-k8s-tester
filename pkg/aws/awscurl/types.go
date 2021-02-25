package awscurl

type Config struct {
	ClusterArn                 string
	MaxRequestsInflight        string
	KubeControllerManagerQPS   string
	KubeControllerManagerBurst string
	KubeSchedulerQPS           string
	KubeSchedulerBurst         string
	URI                        string

	Service string
	Region  string
	Method  string
}

// awscurl -X POST \
// --service eks-internal \
// --region us-west-2 \
// -d '{
//       "clusterArn": "GetRef.ClusterARN",
//       "customFlagsConfig":
//          "{
//             \"apiServer\":
//               {
//                 \"maxRequestsInflight\":\"3000\"
//               }
//             \"controllerManager\":
//               {
//                 \"kubeApiQps\":\"500\",
//                 \"kubeApiBurst\":\"500\"
//               },
//             \"scheduler\":
//               { \"kubeApiQps\":\"500\",
//                 \"kubeApiBurst\":\"500\"
//               }
//            }"
//     }' \
// https://dnd6dnyr8j.execute-api.us-west-2.amazonaws.com/test/internal/clusters/update-master-flags
type payload struct {
	ClusterArn        string `json:"clusterArn"`
	CustomFlagsConfig string `json:"customFlagsConfig"`
}

type customFlagsConfig struct {
	Apiserver         apiserverCustomFlag `json:"apiServer"`
	ControllerManager qpsBurst            `json:"controllerManager"`
	Scheduler         qpsBurst            `json:"scheduler"`
}

type apiserverCustomFlag struct {
	MaxRequestsInflight string `json:"maxRequestsInflight"`
}

type qpsBurst struct {
	KubeApiQps   string `json:"kubeApiQps"`
	KubeApiBurst string `json:"kubeApiBurst"`
}
