//go:build e2e

package common

import (
	"flag"
	"fmt"
	"github.com/urfave/sflags/gen/gpflag"
	"github.com/spf13/pflag"
	"reflect"
)

// For CloudWatch metric dimension flag
type MetricOps struct {
	// gpflag supports map[string]string but with a different non-standard parsing format (key:val) that doesn't match
	// what the project wants (comma separated key=value pairs). So, we force it to skip parsing under gpflag.Parse.
	MetricDimensions map[string]string `flag:"-"`
}

func ParseFlags(config interface{}) (*pflag.FlagSet, error) {
	flags, err := gpflag.Parse(config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse flags: %w", err)
	}

	// gpflag supports map[string]string but with a different non-standard parsing format (key:val) that doesn't
	// match what the project wants (key=val,key=val). So, we handle MetricDimensions separately here to accept
	// comma separated key=value pairs.
	if _, hasField := reflect.TypeOf(config).Elem().FieldByName("MetricDimensions"); hasField {
		field := reflect.ValueOf(config).Elem().FieldByName("MetricDimensions")
		metricDims := field.Addr().Interface().(*map[string]string)
		flags.StringToStringVar(metricDims, "metricDimensions", nil, "CloudWatch metric dimensions as comma-separated key=value pairs")
	}

	flags.VisitAll(func(pf *pflag.Flag) {
		flag.CommandLine.Var(pf.Value, pf.Name, pf.Usage)
	})

	return flags, nil
}
