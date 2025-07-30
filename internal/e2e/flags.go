//go:build e2e

package e2e

import (
	"flag"
	"github.com/octago/sflags/gen/gpflag"
	"github.com/spf13/pflag"
)

// Bridging for test execution => InitFlags parses struct tags into command-line flags and registers them with Go's flag package
func InitFlags(config interface{}, metricDimensions *map[string]string) {
	flags, err := gpflag.Parse(config)
	if err != nil {
		panic(err)
	}

	// Manually add flags for data types that gpflag cannot parse
	flags.StringToStringVar(metricDimensions, "metricDimensions", nil, "CloudWatch metric dimensions as comma-separated key=value pairs")

	flags.VisitAll(func(pf *pflag.Flag) {
		flag.CommandLine.Var(pf.Value, pf.Name, pf.Usage)
	})
}
