package eksconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"sigs.k8s.io/yaml"
)

// UpdateFromEnvs updates fields from environmental variables.
// Empty values are ignored and do not overwrite fields with empty values.
// WARNING: The environmental variable value always overwrites current field
// values if there's a conflict.
func (cfg *Config) UpdateFromEnvs() (err error) {
	cfg.mu.Lock()
	defer func() {
		cfg.unsafeSync()
		cfg.mu.Unlock()
	}()

	if env, ok := os.LookupEnv("AWS_K8S_TESTER_EKS_CONFIG"); ok {
		mu := cfg.mu
		if err = yaml.Unmarshal([]byte(env), cfg, yaml.DisallowUnknownFields); err != nil {
			return err
		}
		cfg.mu = mu
	}

	if cfg.Parameters == nil {
		cfg.Parameters = &Parameters{}
	}
	var vv interface{}
	vv, err = parseEnvs(AWS_K8S_TESTER_EKS_PREFIX, cfg)
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

	if cfg.AddOnCNIVPC == nil {
		cfg.AddOnCNIVPC = &AddOnCNIVPC{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnCNIVPC, cfg.AddOnCNIVPC)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnCNIVPC); ok {
		cfg.AddOnCNIVPC = av
	} else {
		return fmt.Errorf("expected *AddOnCNIVPC, got %T", vv)
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

	if cfg.AddOnCWAgent == nil {
		cfg.AddOnCWAgent = &AddOnCWAgent{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnCWAgent, cfg.AddOnCWAgent)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnCWAgent); ok {
		cfg.AddOnCWAgent = av
	} else {
		return fmt.Errorf("expected *AddOnCWAgent, got %T", vv)
	}

	if cfg.AddOnFluentd == nil {
		cfg.AddOnFluentd = &AddOnFluentd{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnFluentd, cfg.AddOnFluentd)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnFluentd); ok {
		cfg.AddOnFluentd = av
	} else {
		return fmt.Errorf("expected *AddOnFluentd, got %T", vv)
	}

	if cfg.AddOnMetricsServer == nil {
		cfg.AddOnMetricsServer = &AddOnMetricsServer{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnMetricsServer, cfg.AddOnMetricsServer)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnMetricsServer); ok {
		cfg.AddOnMetricsServer = av
	} else {
		return fmt.Errorf("expected *AddOnMetricsServer, got %T", vv)
	}

	if cfg.AddOnConformance == nil {
		cfg.AddOnConformance = &AddOnConformance{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnConformance, cfg.AddOnConformance)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnConformance); ok {
		cfg.AddOnConformance = av
	} else {
		return fmt.Errorf("expected *AddOnConformance, got %T", vv)
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

	if cfg.AddOnCSIEBS == nil {
		cfg.AddOnCSIEBS = &AddOnCSIEBS{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnCSIEBS, cfg.AddOnCSIEBS)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnCSIEBS); ok {
		cfg.AddOnCSIEBS = av
	} else {
		return fmt.Errorf("expected *AddOnCSIEBS, got %T", vv)
	}

	if cfg.AddOnKubernetesDashboard == nil {
		cfg.AddOnKubernetesDashboard = &AddOnKubernetesDashboard{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnKubernetesDashboard, cfg.AddOnKubernetesDashboard)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnKubernetesDashboard); ok {
		cfg.AddOnKubernetesDashboard = av
	} else {
		return fmt.Errorf("expected *AddOnKubernetesDashboard, got %T", vv)
	}

	if cfg.AddOnPrometheusGrafana == nil {
		cfg.AddOnPrometheusGrafana = &AddOnPrometheusGrafana{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnPrometheusGrafana, cfg.AddOnPrometheusGrafana)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnPrometheusGrafana); ok {
		cfg.AddOnPrometheusGrafana = av
	} else {
		return fmt.Errorf("expected *AddOnPrometheusGrafana, got %T", vv)
	}

	if cfg.AddOnPHPApache == nil {
		cfg.AddOnPHPApache = &AddOnPHPApache{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnPHPApache, cfg.AddOnPHPApache)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnPHPApache); ok {
		cfg.AddOnPHPApache = av
	} else {
		return fmt.Errorf("expected *AddOnPHPApache, got %T", vv)
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

	if cfg.AddOnNLBGuestbook == nil {
		cfg.AddOnNLBGuestbook = &AddOnNLBGuestbook{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnNLBGuestbook, cfg.AddOnNLBGuestbook)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnNLBGuestbook); ok {
		cfg.AddOnNLBGuestbook = av
	} else {
		return fmt.Errorf("expected *AddOnNLBGuestbook, got %T", vv)
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

	if cfg.AddOnCSRsLocal == nil {
		cfg.AddOnCSRsLocal = &AddOnCSRsLocal{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnCSRsLocal, cfg.AddOnCSRsLocal)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnCSRsLocal); ok {
		cfg.AddOnCSRsLocal = av
	} else {
		return fmt.Errorf("expected *AddOnCSRsLocal, got %T", vv)
	}

	if cfg.AddOnCSRsRemote == nil {
		cfg.AddOnCSRsRemote = &AddOnCSRsRemote{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnCSRsRemote, cfg.AddOnCSRsRemote)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnCSRsRemote); ok {
		cfg.AddOnCSRsRemote = av
	} else {
		return fmt.Errorf("expected *AddOnCSRsRemote, got %T", vv)
	}

	if cfg.AddOnConfigmapsLocal == nil {
		cfg.AddOnConfigmapsLocal = &AddOnConfigmapsLocal{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnConfigmapsLocal, cfg.AddOnConfigmapsLocal)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnConfigmapsLocal); ok {
		cfg.AddOnConfigmapsLocal = av
	} else {
		return fmt.Errorf("expected *AddOnConfigmapsLocal, got %T", vv)
	}

	if cfg.AddOnConfigmapsRemote == nil {
		cfg.AddOnConfigmapsRemote = &AddOnConfigmapsRemote{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnConfigmapsRemote, cfg.AddOnConfigmapsRemote)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnConfigmapsRemote); ok {
		cfg.AddOnConfigmapsRemote = av
	} else {
		return fmt.Errorf("expected *AddOnConfigmapsRemote, got %T", vv)
	}

	if cfg.AddOnSecretsLocal == nil {
		cfg.AddOnSecretsLocal = &AddOnSecretsLocal{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnSecretsLocal, cfg.AddOnSecretsLocal)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnSecretsLocal); ok {
		cfg.AddOnSecretsLocal = av
	} else {
		return fmt.Errorf("expected *AddOnSecretsLocal, got %T", vv)
	}

	if cfg.AddOnSecretsRemote == nil {
		cfg.AddOnSecretsRemote = &AddOnSecretsRemote{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnSecretsRemote, cfg.AddOnSecretsRemote)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnSecretsRemote); ok {
		cfg.AddOnSecretsRemote = av
	} else {
		return fmt.Errorf("expected *AddOnSecretsRemote, got %T", vv)
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

	if cfg.AddOnIRSAFargate == nil {
		cfg.AddOnIRSAFargate = &AddOnIRSAFargate{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnIRSAFargate, cfg.AddOnIRSAFargate)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnIRSAFargate); ok {
		cfg.AddOnIRSAFargate = av
	} else {
		return fmt.Errorf("expected *AddOnIRSAFargate, got %T", vv)
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

	if cfg.AddOnJupyterHub == nil {
		cfg.AddOnJupyterHub = &AddOnJupyterHub{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnJupyterHub, cfg.AddOnJupyterHub)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnJupyterHub); ok {
		cfg.AddOnJupyterHub = av
	} else {
		return fmt.Errorf("expected *AddOnJupyterHub, got %T", vv)
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

	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnCUDAVectorAdd, cfg.AddOnCUDAVectorAdd)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnCUDAVectorAdd); ok {
		cfg.AddOnCUDAVectorAdd = av
	} else {
		return fmt.Errorf("expected *AddOnCUDAVectorAdd, got %T", vv)
	}

	if cfg.AddOnClusterLoaderLocal == nil {
		cfg.AddOnClusterLoaderLocal = &AddOnClusterLoaderLocal{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnClusterLoaderLocal, cfg.AddOnClusterLoaderLocal)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnClusterLoaderLocal); ok {
		cfg.AddOnClusterLoaderLocal = av
	} else {
		return fmt.Errorf("expected *AddOnClusterLoaderLocal, got %T", vv)
	}

	if cfg.AddOnClusterLoaderRemote == nil {
		cfg.AddOnClusterLoaderRemote = &AddOnClusterLoaderRemote{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnClusterLoaderRemote, cfg.AddOnClusterLoaderRemote)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnClusterLoaderRemote); ok {
		cfg.AddOnClusterLoaderRemote = av
	} else {
		return fmt.Errorf("expected *AddOnClusterLoaderRemote, got %T", vv)
	}

	if cfg.AddOnHollowNodesLocal == nil {
		cfg.AddOnHollowNodesLocal = &AddOnHollowNodesLocal{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnHollowNodesLocal, cfg.AddOnHollowNodesLocal)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnHollowNodesLocal); ok {
		cfg.AddOnHollowNodesLocal = av
	} else {
		return fmt.Errorf("expected *AddOnHollowNodesLocal, got %T", vv)
	}

	if cfg.AddOnHollowNodesRemote == nil {
		cfg.AddOnHollowNodesRemote = &AddOnHollowNodesRemote{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnHollowNodesRemote, cfg.AddOnHollowNodesRemote)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnHollowNodesRemote); ok {
		cfg.AddOnHollowNodesRemote = av
	} else {
		return fmt.Errorf("expected *AddOnHollowNodesRemote, got %T", vv)
	}

	if cfg.AddOnStresserLocal == nil {
		cfg.AddOnStresserLocal = &AddOnStresserLocal{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnStresserLocal, cfg.AddOnStresserLocal)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnStresserLocal); ok {
		cfg.AddOnStresserLocal = av
	} else {
		return fmt.Errorf("expected *AddOnStresserLocal, got %T", vv)
	}

	if cfg.AddOnStresserRemote == nil {
		cfg.AddOnStresserRemote = &AddOnStresserRemote{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnStresserRemote, cfg.AddOnStresserRemote)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnStresserRemote); ok {
		cfg.AddOnStresserRemote = av
	} else {
		return fmt.Errorf("expected *AddOnStresserRemote, got %T", vv)
	}

	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnStresserRemoteV2, cfg.AddOnStresserRemoteV2)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnStresserRemoteV2); ok {
		cfg.AddOnStresserRemoteV2 = av
	} else {
		return fmt.Errorf("expected *AddOnStresserRemoteV2, got %T", vv)
	}

	if cfg.AddOnClusterVersionUpgrade == nil {
		cfg.AddOnClusterVersionUpgrade = &AddOnClusterVersionUpgrade{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnClusterVersionUpgrade, cfg.AddOnClusterVersionUpgrade)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnClusterVersionUpgrade); ok {
		cfg.AddOnClusterVersionUpgrade = av
	} else {
		return fmt.Errorf("expected *AddOnClusterVersionUpgrade, got %T", vv)
	}

	if cfg.AddOnAmiSoftLockupIssue454 == nil {
		cfg.AddOnAmiSoftLockupIssue454 = &AddOnAmiSoftLockupIssue454{}
	}
	vv, err = parseEnvs(EnvironmentVariablePrefixAddOnAmiSoftLockupIssue454, cfg.AddOnAmiSoftLockupIssue454)
	if err != nil {
		return err
	}
	if av, ok := vv.(*AddOnAmiSoftLockupIssue454); ok {
		cfg.AddOnAmiSoftLockupIssue454 = av
	} else {
		return fmt.Errorf("expected *AddOnAmiSoftLockupIssue454, got %T", vv)
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
		if tp.Field(i).Tag.Get("read-only") == "true" { // error when read-only field is set for update
			return nil, fmt.Errorf("'%s=%s' is 'read-only' field; should not be set", env, sv)
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
			if vv.Field(i).Type().Name() == "Duration" {
				iv, err := time.ParseDuration(sv)
				if err != nil {
					return nil, fmt.Errorf("failed to parse %q (field name %q, environmental variable key %q, error %v)", sv, fieldName, env, err)
				}
				vv.Field(i).SetInt(int64(iv))
			} else {
				iv, err := strconv.ParseInt(sv, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("failed to parse %q (field name %q, environmental variable key %q, error %v)", sv, fieldName, env, err)
				}
				vv.Field(i).SetInt(iv)
			}

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
			case "Tags",
				"NodeSelector",
				"DeploymentNodeSelector",
				"DeploymentNodeSelector2048":
				vv.Field(i).Set(reflect.ValueOf(make(map[string]string)))
				mm := make(map[string]string)
				if err := json.Unmarshal([]byte(sv), &mm); err != nil {
					return nil, fmt.Errorf("failed to parse %q (field name %q, environmental variable key %q, error %v)", sv, fieldName, env, err)
				}
				vv.Field(i).Set(reflect.ValueOf(mm))

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
