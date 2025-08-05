//go:build e2e

package common

import (
	"flag"
	"fmt"
	"github.com/octago/sflags/gen/gpflag"
	"github.com/spf13/pflag"
	"reflect"
)

// For CloudWatch metric dimension flag
type MetricOps struct {
	MetricDimensions map[string]string `flag:"metricDimensions" desc:"CloudWatch metric dimensions as comma-separated key=value pairs"`
}

func ParseFlags(config interface{}) (*pflag.FlagSet, error) {
	flags, err := gpflag.Parse(config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse flags: %w", err)
	}

	// Handle MetricDimensions map that gpflag doesn't support
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