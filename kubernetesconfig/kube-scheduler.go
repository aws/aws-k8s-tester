package kubernetesconfig

import (
	"fmt"
	"reflect"
)

// KubeScheduler represents "kube-scheduler" configuration.
type KubeScheduler struct {
	// Image is the container image name and tag for kube-scheduler to run as a static pod.
	Image string `json:"image"`

	Kubeconfig  string `json:"kubeconfig" kube-scheduler:"kubeconfig"`
	LeaderElect bool   `json:"leader-elect" kube-scheduler:"leader-elect"`
}

var defaultKubeScheduler = KubeScheduler{
	Image:       fmt.Sprintf("k8s.gcr.io/kube-apiserver:v%s", defaultKubernetesVersion),
	Kubeconfig:  "/var/lib/kube-scheduler/kubeconfig",
	LeaderElect: true,
}

func newDefaultKubeScheduler() *KubeScheduler {
	copied := defaultKubeScheduler
	return &copied
}

// Flags returns the list of "kube-scheduler" flags.
// Make sure to validate the configuration with "ValidateAndSetDefaults".
func (kb *KubeScheduler) Flags() (flags []string, err error) {
	tp, vv := reflect.TypeOf(kb).Elem(), reflect.ValueOf(kb).Elem()
	for i := 0; i < tp.NumField(); i++ {
		k := tp.Field(i).Tag.Get("kube-scheduler")
		if k == "" {
			continue
		}
		allowZeroValue := tp.Field(i).Tag.Get("allow-zero-value") == "true"

		switch vv.Field(i).Type().Kind() {
		case reflect.String:
			if vv.Field(i).String() != "" {
				flags = append(flags, fmt.Sprintf("--%s=%s", k, vv.Field(i).String()))
			} else if allowZeroValue {
				flags = append(flags, fmt.Sprintf(`--%s=""`, k))
			}

		case reflect.Int, reflect.Int32, reflect.Int64:
			if vv.Field(i).String() != "" {
				flags = append(flags, fmt.Sprintf("--%s=%d", k, vv.Field(i).Int()))
			} else if allowZeroValue {
				flags = append(flags, fmt.Sprintf(`--%s=0`, k))
			}

		case reflect.Bool:
			flags = append(flags, fmt.Sprintf("--%s=%v", k, vv.Field(i).Bool()))

		default:
			return nil, fmt.Errorf("unknown %q", k)
		}
	}
	return flags, nil
}
