package metrics

func NewNoopMetricRegistry() MetricRegistry {
	return &noopRegistry{}
}

type noopRegistry struct{}

func (r *noopRegistry) Record(spec *MetricSpec, value float64, dimensions map[string]string) {}

func (r *noopRegistry) Emit() error {
	return nil
}
