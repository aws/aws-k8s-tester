//go:build e2e

package common

import (
	"flag"
	"github.com/octago/sflags/gen/gpflag"
	"github.com/spf13/pflag"
)

// for adding common flags using gpflag
var MetricDimensions = make(map[string]string)

func InitFlags(config interface{}) {
	flags, err := gpflag.Parse(config)
	if err != nil {
		panic(err)
	}
	flags.StringToStringVar(&MetricDimensions, "metricDimensions", nil, "Metric dimensions as key=value pairs")

	flags.VisitAll(func(pf *pflag.Flag) {
		flag.CommandLine.Var(pf.Value, pf.Name, pf.Usage)
	})
}
