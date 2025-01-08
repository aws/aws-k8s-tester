package eksapi

import (
	"path"

	"github.com/aws/aws-k8s-tester/internal/metrics"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

var DeployerMetricNamespace = path.Join("kubetest2", DeployerName)

var (
	totalRuntimeSeconds = &metrics.MetricSpec{
		Namespace: DeployerMetricNamespace,
		Metric:    "TotalRuntimeSeconds",
		Unit:      cloudwatchtypes.StandardUnitSeconds,
	}

	nodeTimeToRegistrationSeconds = &metrics.MetricSpec{
		Namespace: DeployerMetricNamespace,
		Metric:    "NodeTimeToRegistrationSeconds",
		Unit:      cloudwatchtypes.StandardUnitSeconds,
	}

	nodeTimeToReadySeconds = &metrics.MetricSpec{
		Namespace: DeployerMetricNamespace,
		Metric:    "NodeTimeToReadySeconds",
		Unit:      cloudwatchtypes.StandardUnitSeconds,
	}
)
