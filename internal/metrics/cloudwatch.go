package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go/aws"
	"k8s.io/klog"
)

// NewCloudWatchRegistry creates a new metric registry that will emit values using the specified cloudwatch client
func NewCloudWatchRegistry(cw *cloudwatch.Client) MetricRegistry {
	return &cloudwatchRegistry{
		cw:              cw,
		lock:            &sync.Mutex{},
		dataByNamespace: make(map[string][]*cloudwatchMetricDatum),
	}
}

type cloudwatchRegistry struct {
	cw              *cloudwatch.Client
	lock            *sync.Mutex
	dataByNamespace map[string][]*cloudwatchMetricDatum
}

type cloudwatchMetricDatum struct {
	spec       *MetricSpec
	value      float64
	dimensions map[string]string
	timestamp  time.Time
}

func (r *cloudwatchRegistry) Record(spec *MetricSpec, value float64, dimensions map[string]string) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.dataByNamespace[spec.Namespace] = append(r.dataByNamespace[spec.Namespace], &cloudwatchMetricDatum{
		spec:       spec,
		value:      value,
		dimensions: dimensions,
		timestamp:  time.Now(),
	})
}

func (r *cloudwatchRegistry) Emit() error {
	r.lock.Lock()
	defer r.lock.Unlock()
	for namespace, data := range r.dataByNamespace {
		for i := 0; i < len(data); {
			var metricData []types.MetricDatum
			// we can emit up to 1000 values per PutMetricData
			for j := 0; j < len(data) && j < 1000; j++ {
				datum := data[i]
				var dimensions []types.Dimension
				for key, val := range datum.dimensions {
					dimensions = append(dimensions, types.Dimension{
						Name:  aws.String(key),
						Value: aws.String(val),
					})
				}
				metricData = append(metricData, types.MetricDatum{
					MetricName: aws.String(datum.spec.Metric),
					Value:      aws.Float64(datum.value),
					Dimensions: dimensions,
					Timestamp:  &datum.timestamp,
				})
				i++
			}
			_, err := r.cw.PutMetricData(context.TODO(), &cloudwatch.PutMetricDataInput{
				Namespace:  aws.String(namespace),
				MetricData: metricData,
			})
			if err != nil {
				return err
			}
		}
		klog.Infof("emitted %d metrics to namespace: %s", len(data), namespace)
	}
	r.dataByNamespace = make(map[string][]*cloudwatchMetricDatum)
	return nil
}

func (r *cloudwatchRegistry) GetRegistered() int {
	r.lock.Lock()
	defer r.lock.Unlock()
	registered := 0
	for _, data := range r.dataByNamespace {
		registered += len(data)
	}
	return registered
}
