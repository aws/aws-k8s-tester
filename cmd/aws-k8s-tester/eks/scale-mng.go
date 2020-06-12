package eks

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	name string
)

func newScaleMNG() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mng",
		Short: "Scale a managed nodegroup",
		Run:   scaleMNGFunc,
	}
	return cmd
}

func scaleMNGFunc(cmd *cobra.Command, args []string) {
	if name == "" {
		fmt.Fprintln(os.Stderr, "'--name' flag is not specified")
		os.Exit(1)
	}

	//TODO update MNG to next ScalingConfig in array
	ts.eksAPI.UpdateNodegroupConfig(....ScaleConfigs[1])
}