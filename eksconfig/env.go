package eksconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

const (
	// EnvironmentVariablePrefix is the environment variable prefix used for "eksconfig".
	EnvironmentVariablePrefix = "AWS_K8S_TESTER_EKS_"
	// EnvironmentVariablePrefixParameters is the environment variable prefix used for "eksconfig".
	EnvironmentVariablePrefixParameters = "AWS_K8S_TESTER_EKS_PARAMETERS_"
	// EnvironmentVariablePrefixAddOnNodeGroups is the environment variable prefix used for "eksconfig".
	EnvironmentVariablePrefixAddOnNodeGroups = "AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_"
	// EnvironmentVariablePrefixAddOnManagedNodeGroups is the environment variable prefix used for "eksconfig".
	EnvironmentVariablePrefixAddOnManagedNodeGroups = "AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_"
	// EnvironmentVariablePrefixAddOnNLBHelloWorld is the environment variable prefix used for "eksconfig".
	EnvironmentVariablePrefixAddOnNLBHelloWorld = "AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_"
	// EnvironmentVariablePrefixAddOnALB2048 is the environment variable prefix used for "eksconfig".
	EnvironmentVariablePrefixAddOnALB2048 = "AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_"
	// EnvironmentVariablePrefixAddOnJobsPi is the environment variable prefix used for "eksconfig".
	EnvironmentVariablePrefixAddOnJobsPi = "AWS_K8S_TESTER_EKS_ADD_ON_JOBS_PI_"
	// EnvironmentVariablePrefixAddOnJobsEcho is the environment variable prefix used for "eksconfig".
	EnvironmentVariablePrefixAddOnJobsEcho = "AWS_K8S_TESTER_EKS_ADD_ON_JOBS_ECHO_"
	// EnvironmentVariablePrefixAddOnCronJobs is the environment variable prefix used for "eksconfig".
	EnvironmentVariablePrefixAddOnCronJobs = "AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_"
	// EnvironmentVariablePrefixAddOnCSRs is the environment variable prefix used for "eksconfig".
	EnvironmentVariablePrefixAddOnCSRs = "AWS_K8S_TESTER_EKS_ADD_ON_CSRS_"
	// EnvironmentVariablePrefixAddOnConfigMaps is the environment variable prefix used for "eksconfig".
	EnvironmentVariablePrefixAddOnConfigMaps = "AWS_K8S_TESTER_EKS_ADD_ON_CONFIG_MAPS_"
	// EnvironmentVariablePrefixAddOnSecrets is the environment variable prefix used for "eksconfig".
	EnvironmentVariablePrefixAddOnSecrets = "AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_"
	// EnvironmentVariablePrefixAddOnIRSA is the environment variable prefix used for "eksconfig".
	EnvironmentVariablePrefixAddOnIRSA = "AWS_K8S_TESTER_EKS_ADD_ON_IRSA_"
	// EnvironmentVariablePrefixAddOnFargate is the environment variable prefix used for "eksconfig".
	EnvironmentVariablePrefixAddOnFargate = "AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_"
	// EnvironmentVariablePrefixAddOnAppMesh is the environment variable prefix used for "eksconfig".
	EnvironmentVariablePrefixAddOnAppMesh = "AWS_K8S_TESTER_EKS_ADD_ON_APP_MESH_"
	// EnvironmentVariablePrefixAddOnWordpress is the environment variable prefix used for "eksconfig".
	EnvironmentVariablePrefixAddOnWordpress = "AWS_K8S_TESTER_EKS_ADD_ON_WORDPRESS_"
	// EnvironmentVariablePrefixAddOnKubeflow is the environment variable prefix used for "eksconfig".
	EnvironmentVariablePrefixAddOnKubeflow = "AWS_K8S_TESTER_EKS_ADD_ON_KUBEFLOW_"
)

// UpdateFromEnvs updates fields from environmental variables.
// Empty values are ignored and do not overwrite fields with empty values.
// WARNING: The environmetal variable value always overwrites current field
// values if there's a conflict.
func (cfg *Config) UpdateFromEnvs() (err error) {
	cfg.mu.Lock()
	defer func() {
		cfg.unsafeSync()
		cfg.mu.Unlock()
	}()

	if cfg.Parameters == nil {
		cfg.Parameters = &Parameters{}
	}
	var vv interface{}
	vv, err = parseEnvs(EnvironmentVariablePrefix, cfg)
	if err != nil {
		return err
	}
	if av, ok := vv.(*Config); ok {
		before := cfg.Parameters
		cfg = av
		after := cfg.Parameters
		if !reflect.DeepEqual(before, after) {
			return fmt.Errorf("Parameters overwritten [before %+v, after %+v]", before, after)
		}
	} else {
		return fmt.Errorf("expected *Config, got %T", vv)
	}

	if cfg.Parameters == nil {
		cfg.Parameters = &Parameters{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixParameters, cfg.Parameters)
	if err != nil {
		return err
	}
	if av, ok := vv.(*Parameters); ok {
		cfg.Parameters = av
	} else {
		return fmt.Errorf("expected *Parameters, got %T", vv)
	}

	if cfg.AddOnNodeGroups == nil {
		cfg.AddOnNodeGroups = &AddOnNodeGroups{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnNodeGroups, cfg.AddOnNodeGroups)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnNodeGroups); ok {
		cfg.AddOnNodeGroups = av
	} else {
		return fmt.Errorf("expected *AddOnNodeGroups, got %T", vv)
	}

	if cfg.AddOnManagedNodeGroups == nil {
		cfg.AddOnManagedNodeGroups = &AddOnManagedNodeGroups{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnManagedNodeGroups, cfg.AddOnManagedNodeGroups)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnManagedNodeGroups); ok {
		cfg.AddOnManagedNodeGroups = av
	} else {
		return fmt.Errorf("expected *AddOnManagedNodeGroups, got %T", vv)
	}

	if cfg.AddOnNLBHelloWorld == nil {
		cfg.AddOnNLBHelloWorld = &AddOnNLBHelloWorld{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnNLBHelloWorld, cfg.AddOnNLBHelloWorld)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnNLBHelloWorld); ok {
		cfg.AddOnNLBHelloWorld = av
	} else {
		return fmt.Errorf("expected *AddOnNLBHelloWorld, got %T", vv)
	}

	if cfg.AddOnALB2048 == nil {
		cfg.AddOnALB2048 = &AddOnALB2048{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnALB2048, cfg.AddOnALB2048)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnALB2048); ok {
		cfg.AddOnALB2048 = av
	} else {
		return fmt.Errorf("expected *AddOnALB2048, got %T", vv)
	}

	if cfg.AddOnJobsPi == nil {
		cfg.AddOnJobsPi = &AddOnJobsPi{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnJobsPi, cfg.AddOnJobsPi)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnJobsPi); ok {
		cfg.AddOnJobsPi = av
	} else {
		return fmt.Errorf("expected *AddOnJobsPi, got %T", vv)
	}

	if cfg.AddOnJobsEcho == nil {
		cfg.AddOnJobsEcho = &AddOnJobsEcho{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnJobsEcho, cfg.AddOnJobsEcho)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnJobsEcho); ok {
		cfg.AddOnJobsEcho = av
	} else {
		return fmt.Errorf("expected *AddOnJobsEcho, got %T", vv)
	}

	if cfg.AddOnCronJobs == nil {
		cfg.AddOnCronJobs = &AddOnCronJobs{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnCronJobs, cfg.AddOnCronJobs)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnCronJobs); ok {
		cfg.AddOnCronJobs = av
	} else {
		return fmt.Errorf("expected *AddOnCronJobs, got %T", vv)
	}

	if cfg.AddOnCSRs == nil {
		cfg.AddOnCSRs = &AddOnCSRs{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnCSRs, cfg.AddOnCSRs)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnCSRs); ok {
		cfg.AddOnCSRs = av
	} else {
		return fmt.Errorf("expected *AddOnCSRs, got %T", vv)
	}

	if cfg.AddOnConfigMaps == nil {
		cfg.AddOnConfigMaps = &AddOnConfigMaps{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnConfigMaps, cfg.AddOnConfigMaps)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnConfigMaps); ok {
		cfg.AddOnConfigMaps = av
	} else {
		return fmt.Errorf("expected *AddOnConfigMaps, got %T", vv)
	}

	if cfg.AddOnSecrets == nil {
		cfg.AddOnSecrets = &AddOnSecrets{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnSecrets, cfg.AddOnSecrets)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnSecrets); ok {
		cfg.AddOnSecrets = av
	} else {
		return fmt.Errorf("expected *AddOnSecrets, got %T", vv)
	}

	if cfg.AddOnIRSA == nil {
		cfg.AddOnIRSA = &AddOnIRSA{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnIRSA, cfg.AddOnIRSA)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnIRSA); ok {
		cfg.AddOnIRSA = av
	} else {
		return fmt.Errorf("expected *AddOnIRSA, got %T", vv)
	}

	if cfg.AddOnFargate == nil {
		cfg.AddOnFargate = &AddOnFargate{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnFargate, cfg.AddOnFargate)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnFargate); ok {
		cfg.AddOnFargate = av
	} else {
		return fmt.Errorf("expected *AddOnFargate, got %T", vv)
	}

	if cfg.AddOnAppMesh == nil {
		cfg.AddOnAppMesh = &AddOnAppMesh{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnAppMesh, cfg.AddOnAppMesh)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnAppMesh); ok {
		cfg.AddOnAppMesh = av
	} else {
		return fmt.Errorf("expected *AddOnAppMesh, got %T", vv)
	}

	if cfg.AddOnWordpress == nil {
		cfg.AddOnWordpress = &AddOnWordpress{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnWordpress, cfg.AddOnWordpress)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnWordpress); ok {
		cfg.AddOnWordpress = av
	} else {
		return fmt.Errorf("expected *AddOnWordpress, got %T", vv)
	}

	if cfg.AddOnKubeflow == nil {
		cfg.AddOnKubeflow = &AddOnKubeflow{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnKubeflow, cfg.AddOnKubeflow)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnKubeflow); ok {
		cfg.AddOnKubeflow = av
	} else {
		return fmt.Errorf("expected *AddOnKubeflow, got %T", vv)
	}

	return nil
}

func parseEnvs(pfx string, addOn interface{}) (interface{}, error) {
	tp, vv := reflect.TypeOf(addOn).Elem(), reflect.ValueOf(addOn).Elem()
	for i := 0; i < tp.NumField(); i++ {
		jv := tp.Field(i).Tag.Get("json")
		if jv == "" {
			continue
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := pfx + jv
		sv := os.Getenv(env)
		if sv == "" {
			continue
		}
		if tp.Field(i).Tag.Get("read-only") == "true" { // skip updating read-only field
			continue
		}
		fieldName := tp.Field(i).Name

		switch vv.Field(i).Type().Kind() {
		case reflect.String:
			vv.Field(i).SetString(sv)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %q (field name %q, environmental variable key %q, error %v)", sv, fieldName, env, err)
			}
			vv.Field(i).SetBool(bb)

		case reflect.Int, reflect.Int32, reflect.Int64:
			iv, err := strconv.ParseInt(sv, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %q (field name %q, environmental variable key %q, error %v)", sv, fieldName, env, err)
			}
			vv.Field(i).SetInt(iv)

		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			iv, err := strconv.ParseUint(sv, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %q (field name %q, environmental variable key %q, error %v)", sv, fieldName, env, err)
			}
			vv.Field(i).SetUint(iv)

		case reflect.Float32, reflect.Float64:
			fv, err := strconv.ParseFloat(sv, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %q (field name %q, environmental variable key %q, error %v)", sv, fieldName, env, err)
			}
			vv.Field(i).SetFloat(fv)

		case reflect.Slice: // only supports "[]string" for now
			ss := strings.Split(sv, ",")
			if len(ss) < 1 {
				continue
			}
			slice := reflect.MakeSlice(reflect.TypeOf([]string{}), len(ss), len(ss))
			for j := range ss {
				slice.Index(j).SetString(ss[j])
			}
			vv.Field(i).Set(slice)

		case reflect.Map:
			switch fieldName {
			case "Tags":
				vv.Field(i).Set(reflect.ValueOf(make(map[string]string)))
				for _, pair := range strings.Split(sv, ";") {
					fields := strings.Split(pair, "=")
					if len(fields) != 2 {
						return nil, fmt.Errorf("map %q for %q has unexpected format (e.g. should be 'a=b;c;d,e=f')", sv, fieldName)
					}
					vv.Field(i).SetMapIndex(reflect.ValueOf(fields[0]), reflect.ValueOf(fields[1]))
				}

			case "ASGs":
				asgs := make(map[string]ASG)
				if err := json.Unmarshal([]byte(sv), &asgs); err != nil {
					return nil, fmt.Errorf("failed to parse %q (field name %q, environmental variable key %q, error %v)", sv, fieldName, env, err)
				}
				for k, v := range asgs {
					tp2, vv2 := reflect.TypeOf(&v).Elem(), reflect.ValueOf(&v).Elem()
					for j := 0; j < tp2.NumField(); j++ {
						jv := tp2.Field(j).Tag.Get("json")
						if jv == "" {
							continue
						}
						if tp2.Field(j).Tag.Get("read-only") != "true" {
							continue
						}
						if vv2.Field(j).Type().Kind() != reflect.String {
							continue
						}
						// skip updating read-only field
						vv2.Field(j).SetString("")
					}
					asgs[k] = v
				}
				vv.Field(i).Set(reflect.ValueOf(asgs))

			case "MNGs":
				mngs := make(map[string]MNG)
				if err := json.Unmarshal([]byte(sv), &mngs); err != nil {
					return nil, fmt.Errorf("failed to parse %q (field name %q, environmental variable key %q, error %v)", sv, fieldName, env, err)
				}
				for k, v := range mngs {
					tp2, vv2 := reflect.TypeOf(&v).Elem(), reflect.ValueOf(&v).Elem()
					for j := 0; j < tp2.NumField(); j++ {
						jv := tp2.Field(j).Tag.Get("json")
						if jv == "" {
							continue
						}
						if tp2.Field(j).Tag.Get("read-only") != "true" {
							continue
						}
						if vv2.Field(j).Type().Kind() != reflect.String {
							continue
						}
						// skip updating read-only field
						vv2.Field(j).SetString("")
					}
					mngs[k] = v
				}
				vv.Field(i).Set(reflect.ValueOf(mngs))

			default:
				return nil, fmt.Errorf("field %q not supported for reflect.Map", fieldName)
			}

		default:
			return nil, fmt.Errorf("%q (type %v) is not supported as an env", env, vv.Field(i).Type())
		}
	}
	return addOn, nil
}
