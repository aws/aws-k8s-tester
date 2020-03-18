package ec2config

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// EnvironmentVariablePrefix is the environment variable prefix used for "ec2config".
const EnvironmentVariablePrefix = "AWS_K8S_TESTER_EC2_"

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

	var vv interface{}
	vv, err = parseEnvs(EnvironmentVariablePrefix, cfg)
	if err != nil {
		return err
	}
	if av, ok := vv.(*Config); ok {
		cfg = av
	} else {
		return fmt.Errorf("expected *Config, got %T", vv)
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

			default:
				return nil, fmt.Errorf("field %q not supported for reflect.Map", fieldName)
			}

		default:
			return nil, fmt.Errorf("%q (type %v) is not supported as an env", env, vv.Field(i).Type())
		}
	}
	return addOn, nil
}
