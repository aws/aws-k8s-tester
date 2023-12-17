package metrics

import (
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

type MetricRegistry interface {
	// Record adds a new metric value to the registry
	Record(spec *MetricSpec, value float64, dimensions map[string]string)
	// Emit sends all registered metric values to cloudwatch, emptying the registry
	Emit() error
}

type MetricSpec struct {
	Namespace string
	Metric    string
	Unit      types.StandardUnit
}
