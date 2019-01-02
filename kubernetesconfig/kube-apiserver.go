package kubernetesconfig

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// KubeAPIServer represents "kube-apiserver" configuration.
type KubeAPIServer struct {
	// Image is the container image name and tag for kube-apiserver to run as a static pod.
	Image string `json:"image"`

	// TODO: support running as a systemd service?

	AllowPrivileged                 bool   `json:"allow-privileged" kube-apiserver:"allow-privileged"`
	AnonymousAuth                   bool   `json:"anonymous-auth" kube-apiserver:"anonymous-auth"`
	APIServerCount                  int    `json:"apiserver-count" kube-apiserver:"apiserver-count"`
	AuthorizationMode               string `json:"authorization-mode" kube-apiserver:"authorization-mode"`
	BasicAuthFile                   string `json:"basic-auth-file" kube-apiserver:"basic-auth-file"`
	BindAddress                     string `json:"bind-address" kube-apiserver:"bind-address"`
	ClientCAFile                    string `json:"client-ca-file" kube-apiserver:"client-ca-file"`
	CloudProvider                   string `json:"cloud-provider" kube-apiserver:"cloud-provider"`
	EnableAdmissionPlugins          string `json:"enable-admission-plugins" kube-apiserver:"enable-admission-plugins"`
	EtcdServersOverrides            string `json:"etcd-servers-overrides" kube-apiserver:"etcd-servers-overrides"`
	EtcdServers                     string `json:"etcd-servers" kube-apiserver:"etcd-servers"`
	InsecureBindAddress             string `json:"insecure-bind-address" kube-apiserver:"insecure-bind-address"`
	InsecurePort                    int    `json:"insecure-port" kube-apiserver:"insecure-port"`
	KubeletClientCertificate        string `json:"kubelet-client-certificate" kube-apiserver:"kubelet-client-certificate"`
	KubeletClientKey                string `json:"kubelet-client-key" kube-apiserver:"kubelet-client-key"`
	KubeletPreferredAddressTypes    string `json:"kubelet-preferred-address-types" kube-apiserver:"kubelet-preferred-address-types"`
	ProxyClientCertFile             string `json:"proxy-client-cert-file" kube-apiserver:"proxy-client-cert-file"`
	ProxyClientKeyFile              string `json:"proxy-client-key-file" kube-apiserver:"proxy-client-key-file"`
	RequestHeaderAllowedNames       string `json:"request-header-allowed-names" kube-apiserver:"requestheader-allowed-names"`
	RequestHeaderClientCAFile       string `json:"request-header-client-ca-file" kube-apiserver:"requestheader-client-ca-file"`
	RequestHeaderExtraHeadersPrefix string `json:"request-header-extra-headers-prefix" kube-apiserver:"requestheader-extra-headers-prefix"`
	RequestHeaderGroupHeaders       string `json:"request-header-group-headers" kube-apiserver:"requestheader-group-headers"`
	RequestHeaderUsernameHeaders    string `json:"request-header-username-headers" kube-apiserver:"requestheader-username-headers"`
	SecurePort                      int    `json:"secure-port" kube-apiserver:"secure-port"`
	ServiceClusterIPRange           string `json:"service-cluster-ip-range" kube-apiserver:"service-cluster-ip-range"`
	StorageBackend                  string `json:"storage-backend" kube-apiserver:"storage-backend"`
	TLSCertFile                     string `json:"tls-cert-file" kube-apiserver:"tls-cert-file"`
	TLSPrivateKeyFile               string `json:"tls-private-key-file" kube-apiserver:"tls-private-key-file"`
	TokenAuthFile                   string `json:"token-auth-file" kube-apiserver:"token-auth-file"`
	V                               int    `json:"v" kube-apiserver:"v"`
}

var defaultKubeAPIServer = KubeAPIServer{
	AllowPrivileged:                 true,
	AnonymousAuth:                   false,
	APIServerCount:                  1,
	AuthorizationMode:               "RBAC",
	BasicAuthFile:                   "",
	BindAddress:                     "0.0.0.0",
	ClientCAFile:                    "/srv/kubernetes/ca.crt",
	CloudProvider:                   "aws",
	EnableAdmissionPlugins:          "Initializers,NamespaceLifecycle,LimitRanger,ServiceAccount,PersistentVolumeLabel,DefaultStorageClass,DefaultTolerationSeconds,MutatingAdmissionWebhook,ValidatingAdmissionWebhook,NodeRestriction,ResourceQuota",
	EtcdServersOverrides:            "",
	EtcdServers:                     "http://127.0.0.1:2379",
	InsecureBindAddress:             "127.0.0.1",
	InsecurePort:                    8080,
	KubeletClientCertificate:        "/srv/kubernetes/kubelet-api.pem",
	KubeletClientKey:                "/srv/kubernetes/kubelet-api-key.pem",
	KubeletPreferredAddressTypes:    "InternalIP,Hostname,ExternalIP",
	ProxyClientCertFile:             "",
	ProxyClientKeyFile:              "",
	RequestHeaderAllowedNames:       "",
	RequestHeaderClientCAFile:       "",
	RequestHeaderExtraHeadersPrefix: "X-Remote-Extra-",
	RequestHeaderGroupHeaders:       "X-Remote-Group",
	RequestHeaderUsernameHeaders:    "X-Remote-User",
	SecurePort:                      443,
	ServiceClusterIPRange:           "100.64.0.0/13",
	StorageBackend:                  "etcd3",
	TLSCertFile:                     "/srv/kubernetes/server.cert",
	TLSPrivateKeyFile:               "/srv/kubernetes/server.key",
	TokenAuthFile:                   "/srv/kubernetes/known_tokens.csv",
	V:                               2,
}

func newDefaultKubeAPIServer() *KubeAPIServer {
	copied := defaultKubeAPIServer
	return &copied
}

func (kb *KubeAPIServer) updateFromEnvs(pfx string) error {
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

// Flags returns the list of "kube-apiserver" flags.
// Make sure to validate the configuration with "ValidateAndSetDefaults".
func (kb *KubeAPIServer) Flags() (flags []string, err error) {
	tp, vv := reflect.TypeOf(kb).Elem(), reflect.ValueOf(kb).Elem()
	for i := 0; i < tp.NumField(); i++ {
		k := tp.Field(i).Tag.Get("kube-apiserver")
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
			if vv.Field(i).Int() != 0 {
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
