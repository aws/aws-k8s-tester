package kubernetesconfig

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// KubeControllerManager represents "kube-controller-manager" configuration.
type KubeControllerManager struct {
	// Image is the container image name and tag for kube-controller-manager to run as a static pod.
	Image string `json:"image"`

	AllocateNodeCIDRs               bool   `json:"allocate-node-cidrs" kube-controller-manager:"allocate-node-cidrs"`
	AttachDetachReconcileSyncPeriod string `json:"attach-detach-reconcile-sync-period" kube-controller-manager:"attach-detach-reconcile-sync-period"`
	CloudProvider                   string `json:"cloud-provider" kube-controller-manager:"cloud-provider"`
	ClusterCIDR                     string `json:"cluster-cidr" kube-controller-manager:"cluster-cidr"`
	ClusterName                     string `json:"cluster-name" kube-controller-manager:"cluster-name"`
	ClusterSigningCertFile          string `json:"cluster-signing-cert-file" kube-controller-manager:"cluster-signing-cert-file"`
	ClusterSigningKeyFile           string `json:"cluster-signing-key-file" kube-controller-manager:"cluster-signing-key-file"`
	ConfigureCloudRoutes            bool   `json:"configure-cloud-routes" kube-controller-manager:"configure-cloud-routes"`
	Kubeconfig                      string `json:"kubeconfig" kube-controller-manager:"kubeconfig"`
	LeaderElect                     bool   `json:"leader-elect" kube-controller-manager:"leader-elect"`
	RootCAFile                      string `json:"root-ca-file" kube-controller-manager:"root-ca-file"`
	ServiceAccountPrivateKeyFile    string `json:"service-account-private-key-file" kube-controller-manager:"service-account-private-key-file"`
	UseServiceAccountCredentials    bool   `json:"use-service-account-credentials" kube-controller-manager:"use-service-account-credentials"`
	V                               int    `json:"v" kube-controller-manager:"v"`
}

var defaultKubeControllerManager = KubeControllerManager{
	AllocateNodeCIDRs:               true,
	AttachDetachReconcileSyncPeriod: "1m0s",
	CloudProvider:                   "aws",
	ClusterCIDR:                     "100.96.0.0/11",
	ClusterName:                     "leegyuho-kops.k8s.local",
	ClusterSigningCertFile:          "/srv/kubernetes/ca.crt",
	ClusterSigningKeyFile:           "/srv/kubernetes/ca.key",
	ConfigureCloudRoutes:            true,
	Kubeconfig:                      "/var/lib/kube-controller-manager/kubeconfig",
	LeaderElect:                     true,
	RootCAFile:                      "/srv/kubernetes/ca.crt",
	ServiceAccountPrivateKeyFile:    "/srv/kubernetes/server.key",
	UseServiceAccountCredentials:    true,
	V:                               2,
}

func newDefaultKubeControllerManager() *KubeControllerManager {
	copied := defaultKubeControllerManager
	return &copied
}

func (kb *KubeControllerManager) updateFromEnvs(pfx string) error {
	cc := *kb
	tp, vv := reflect.TypeOf(&cc).Elem(), reflect.ValueOf(&cc).Elem()
	for i := 0; i < tp.NumField(); i++ {
		jv := tp.Field(i).Tag.Get("json")
		if jv == "" {
			continue
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.Replace(jv, "-", "_", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := pfx + jv
		if os.Getenv(env) == "" {
			continue
		}
		sv := os.Getenv(env)

		switch vv.Field(i).Type().Kind() {
		case reflect.String:
			vv.Field(i).SetString(sv)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv.Field(i).SetBool(bb)

		case reflect.Int, reflect.Int32, reflect.Int64:
			// if tp.Field(i).Name { continue }
			iv, err := strconv.ParseInt(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv.Field(i).SetInt(iv)

		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			iv, err := strconv.ParseUint(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv.Field(i).SetUint(iv)

		case reflect.Float32, reflect.Float64:
			fv, err := strconv.ParseFloat(sv, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv.Field(i).SetFloat(fv)

		case reflect.Slice:
			ss := strings.Split(sv, ",")
			slice := reflect.MakeSlice(reflect.TypeOf([]string{}), len(ss), len(ss))
			for i := range ss {
				slice.Index(i).SetString(ss[i])
			}
			vv.Field(i).Set(slice)

		default:
			return fmt.Errorf("%q (%v) is not supported as an env", env, vv.Field(i).Type())
		}
	}
	*kb = cc
	return nil
}

// Flags returns the list of "kube-controller-manager" flags.
// Make sure to validate the configuration with "ValidateAndSetDefaults".
func (kb *KubeControllerManager) Flags() (flags []string, err error) {
	tp, vv := reflect.TypeOf(kb).Elem(), reflect.ValueOf(kb).Elem()
	for i := 0; i < tp.NumField(); i++ {
		k := tp.Field(i).Tag.Get("kube-controller-manager")
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
