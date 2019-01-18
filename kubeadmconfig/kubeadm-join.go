package kubeadmconfig

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// KubeadmJoin defines "kubeadm join" configuration.
// https://kubernetes.io/docs/reference/setup-tools/kubeadm/kubeadm-join/
type KubeadmJoin struct {
	RawCommand               string `json:"raw-command"`
	Target                   string `json:"target"`
	Token                    string `json:"token,omitempty" kubeadm-join:"token"`
	DiscoveryTokenCACertHash string `json:"discovery-token-ca-cert-hash,omitempty" kubeadm-join:"discovery-token-ca-cert-hash"`
	IgnorePreflightErrors    string `json:"ignore-preflight-errors,omitempty" kubeadm-join:"ignore-preflight-errors"`
}

var defaultKubeadmJoin = KubeadmJoin{
	IgnorePreflightErrors: "cri",
}

func newDefaultKubeadmJoin() *KubeadmJoin {
	copied := defaultKubeadmJoin
	return &copied
}

// Flags returns the list of "kubeadm join" flags.
// Make sure to validate the configuration with "ValidateAndSetDefaults".
// Make sure to run as root, otherwise "[ERROR IsPrivilegedUser]: user is not running as root".
func (ka *KubeadmJoin) Flags() (flags []string, err error) {
	arg := ka.Target
	if arg == "" {
		return nil, errors.New("unknown 'kubeadm join' target")
	}
	tp, vv := reflect.TypeOf(ka).Elem(), reflect.ValueOf(ka).Elem()
	for i := 0; i < tp.NumField(); i++ {
		k := tp.Field(i).Tag.Get("kubeadm-join")
		if k == "" {
			continue
		}
		allowZeroValue := tp.Field(i).Tag.Get("allow-zero-value") == "true"
		fieldName := tp.Field(i).Name
		if !strings.HasPrefix(fieldName, "Join") {
			continue
		}

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
	return append([]string{arg}, flags...), nil
}

// Command returns the "kubectl join" command.
func (ka *KubeadmJoin) Command() (cmd string, err error) {
	var joinFlags []string
	joinFlags, err = ka.Flags()
	if err != nil {
		return "", err
	}
	cmd = fmt.Sprintf("sudo kubeadm join %s", strings.Join(joinFlags, " "))
	return cmd, nil
}

func (ka *KubeadmJoin) updateFromEnvs(pfx string) error {
	cc := *ka
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
	*ka = cc
	return nil
}
